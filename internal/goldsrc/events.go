package goldsrc

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
)

const (
	macroHeaderSize       = 9
	protocol42NetInfoSize = 560
	protocol45NetInfoSize = 464
	maxNetworkChunkSize   = 16 << 20
	maxMacroStringSize    = 1 << 20
)

// Kill is one Counter-Strike DeathMsg event, not a scoreboard frag/score
// snapshot. Objective bonuses such as C4 defusal do not produce this event.
// Player slots are zero-based in PlayerReference, while the wire-format entity
// indexes are one-based.
type Kill struct {
	TimeSeconds float64          `json:"time_seconds"`
	Frame       uint32           `json:"frame"`
	Killer      *PlayerReference `json:"killer,omitempty"`
	Victim      PlayerReference  `json:"victim"`
	Weapon      string           `json:"weapon"`
	Headshot    bool             `json:"headshot"`
	World       bool             `json:"world,omitempty"`
}

// HLTVProxy identifies the broadcast proxy client embedded in an HLTV demo.
type HLTVProxy struct {
	Name           string  `json:"name"`
	SteamID64      string  `json:"steam_id64,omitempty"`
	Slot           uint8   `json:"slot_zero_based"`
	DelaySeconds   float64 `json:"delay_seconds,omitempty"`
	SpectatorSlots int     `json:"spectator_slots,omitempty"`
}

// TeamChange records a player joining or leaving a Counter-Terrorist or
// Terrorist team. Player slots in the wire message are one-based.
type TeamChange struct {
	TimeSeconds float64         `json:"time_seconds"`
	Frame       uint32          `json:"frame"`
	Player      PlayerReference `json:"player"`
	Team        string          `json:"team"`
}

// ScoreUpdate is emitted whenever a team score increases.
type ScoreUpdate struct {
	TimeSeconds    float64 `json:"time_seconds"`
	Frame          uint32  `json:"frame"`
	CTScore        int     `json:"ct_score"`
	TerroristScore int     `json:"terrorist_score"`
}

// Round groups global DeathMsg events between consecutive score updates.
type Round struct {
	Number         int     `json:"number"`
	StartSeconds   float64 `json:"start_seconds"`
	EndSeconds     float64 `json:"end_seconds"`
	Winner         string  `json:"winner"`
	CTScore        int     `json:"ct_score"`
	TerroristScore int     `json:"terrorist_score"`
	Kills          []Kill  `json:"kills"`
}

type networkFrame struct {
	time         float64
	frame        uint32
	data         []byte
	lengthOffset int64
	dataOffset   int64
}

type timelineState struct {
	deathMessageID byte
	deathMessageOK bool
	maxClients     uint8
	slots          map[uint8]PlayerReference
	kills          []Kill
	teamChanges    []TeamChange
	scoreUpdates   []ScoreUpdate
	hltv           *HLTVProxy
	userMessages   map[string]userMessageDefinition
	ctScore        int
	terroristScore int
}

type userMessageDefinition struct {
	id            byte
	defaultLength byte
}

type messageCandidate struct {
	position int
	kind     byte
	slot     uint8
	userID   uint32
	userinfo []byte
	team     string
	score    int
	killer   uint8
	victim   uint8
	headshot bool
	weapon   string
}

const (
	candidateUserInfo byte = iota + 1
	candidateTeamInfo
	candidateTeamScore
	candidateKill
)

func scanTimeline(r io.ReaderAt, networkProtocol uint32, entries []DirectoryEntry, server *Server) (*timelineState, error) {
	ordered := append([]DirectoryEntry(nil), entries...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Offset < ordered[j].Offset })
	state := &timelineState{
		maxClients:   32,
		slots:        make(map[uint8]PlayerReference),
		userMessages: make(map[string]userMessageDefinition),
	}
	if server != nil && server.MaxClients > 0 {
		state.maxClients = server.MaxClients
	}
	for _, entry := range ordered {
		err := forEachNetworkFrame(r, networkProtocol, entry, func(frame networkFrame) error {
			state.consume(frame)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("segment %q at offset %d: %w", entry.Title, entry.Offset, err)
		}
	}
	return state, nil
}

func (state *timelineState) consume(frame networkFrame) {
	if state.slots == nil {
		state.slots = make(map[uint8]PlayerReference)
	}
	if state.userMessages == nil {
		state.userMessages = make(map[string]userMessageDefinition)
	}
	state.findUserMessageRegistrations(frame.data)
	candidates := state.findMessageCandidates(frame.data)
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].position < candidates[j].position })
	previousCTScore := state.ctScore
	previousTerroristScore := state.terroristScore
	for _, candidate := range candidates {
		switch candidate.kind {
		case candidateUserInfo:
			fields := parseInfoString(candidate.userinfo)
			name := cleanText(fields["name"])
			steamID64 := cleanText(fields["*sid"])
			if name == "" {
				continue
			}
			if fields["*hltv"] == "1" {
				state.hltv = &HLTVProxy{
					Name:           name,
					SteamID64:      steamID64,
					Slot:           candidate.slot,
					DelaySeconds:   parseInfoFloat(fields["hdelay"]),
					SpectatorSlots: parseInfoInt(fields["hslots"]),
				}
				continue
			}
			player := state.slots[candidate.slot]
			player.Name = name
			player.SteamID64 = steamID64
			player.Slot = candidate.slot
			state.slots[candidate.slot] = player
		case candidateTeamInfo:
			player := state.slots[candidate.slot]
			player.Slot = candidate.slot
			player.Team = candidate.team
			state.slots[candidate.slot] = player
			state.teamChanges = append(state.teamChanges, TeamChange{
				TimeSeconds: frame.time,
				Frame:       frame.frame,
				Player:      player,
				Team:        candidate.team,
			})
		case candidateTeamScore:
			switch candidate.team {
			case "CT":
				state.ctScore = candidate.score
			case "TERRORIST":
				state.terroristScore = candidate.score
			}
		case candidateKill:
			kill := Kill{
				TimeSeconds: frame.time,
				Frame:       frame.frame,
				Victim:      state.playerReference(candidate.victim),
				Weapon:      candidate.weapon,
				Headshot:    candidate.headshot,
				World:       candidate.killer == 0,
			}
			if candidate.killer != 0 {
				killer := state.playerReference(candidate.killer)
				kill.Killer = &killer
			}
			state.kills = append(state.kills, kill)
		}
	}
	if (state.ctScore != previousCTScore || state.terroristScore != previousTerroristScore) && state.ctScore+state.terroristScore > 0 {
		state.scoreUpdates = append(state.scoreUpdates, ScoreUpdate{
			TimeSeconds:    frame.time,
			Frame:          frame.frame,
			CTScore:        state.ctScore,
			TerroristScore: state.terroristScore,
		})
	}
}

func (state *timelineState) playerReference(entityIndex uint8) PlayerReference {
	if entityIndex == 0 {
		return PlayerReference{}
	}
	slot := entityIndex - 1
	if player, ok := state.slots[slot]; ok {
		return player
	}
	return PlayerReference{Slot: slot}
}

func (state *timelineState) findUserMessageRegistrations(data []byte) {
	// svc_newusermsg is id, assigned ID, default length, then a 16-byte
	// zero-padded name in the protocol used by these demos.
	for position := 0; position+19 <= len(data); position++ {
		if data[position] != 0x27 {
			continue
		}
		name := trimCString(data[position+3 : position+19])
		if name != "" {
			state.userMessages[name] = userMessageDefinition{id: data[position+1], defaultLength: data[position+2]}
		}
		if name == "DeathMsg" {
			state.deathMessageID = data[position+1]
			state.deathMessageOK = true
		}
	}
}

func (state *timelineState) findMessageCandidates(data []byte) []messageCandidate {
	var candidates []messageCandidate
	for position := 0; position < len(data); position++ {
		if position+7 <= len(data) && data[position] == 0x0d && data[position+1] < state.maxClients && data[position+6] == '\\' {
			endRelative := bytes.IndexByte(data[position+6:], 0)
			if endRelative >= 0 && endRelative <= maxUserInfoLength {
				userinfo := data[position+6 : position+6+endRelative]
				fields := parseInfoString(userinfo)
				if fields["name"] != "" && (fields["*sid"] != "" || fields["model"] != "" || fields["*hltv"] == "1") {
					candidates = append(candidates, messageCandidate{
						position: position,
						kind:     candidateUserInfo,
						slot:     data[position+1],
						userID:   binary.LittleEndian.Uint32(data[position+2 : position+6]),
						userinfo: userinfo,
					})
				}
			}
		}
		if definition, ok := state.userMessages["TeamInfo"]; ok && data[position] == definition.id {
			if candidate, valid := state.parseTeamInfoCandidate(data, position); valid {
				candidates = append(candidates, candidate)
			}
		}
		if definition, ok := state.userMessages["TeamScore"]; ok && data[position] == definition.id {
			if candidate, valid := state.parseTeamScoreCandidate(data, position); valid {
				candidates = append(candidates, candidate)
			}
		}
		if !state.deathMessageOK || data[position] != state.deathMessageID {
			continue
		}
		candidate, ok := state.parseDeathCandidate(data, position)
		if ok {
			candidates = append(candidates, candidate)
		}
	}
	return candidates
}

func parseInfoFloat(value string) float64 {
	parsed, _ := strconv.ParseFloat(value, 64)
	return parsed
}

func parseInfoInt(value string) int {
	parsed, _ := strconv.Atoi(value)
	return parsed
}

func (state *timelineState) parseTeamInfoCandidate(data []byte, position int) (messageCandidate, bool) {
	payload, ok := variableUserMessagePayload(data, position, 64)
	if !ok || len(payload) < 3 || payload[0] == 0 || payload[0] > state.maxClients {
		return messageCandidate{}, false
	}
	end := bytes.IndexByte(payload[1:], 0)
	if end < 0 {
		return messageCandidate{}, false
	}
	team := string(payload[1 : 1+end])
	if !knownTeam[team] {
		return messageCandidate{}, false
	}
	return messageCandidate{
		position: position,
		kind:     candidateTeamInfo,
		slot:     payload[0] - 1,
		team:     team,
	}, true
}

func (state *timelineState) parseTeamScoreCandidate(data []byte, position int) (messageCandidate, bool) {
	payload, ok := variableUserMessagePayload(data, position, 64)
	if !ok || len(payload) < 5 {
		return messageCandidate{}, false
	}
	end := bytes.IndexByte(payload, 0)
	if end < 0 || end+3 > len(payload) {
		return messageCandidate{}, false
	}
	team := string(payload[:end])
	if team != "CT" && team != "TERRORIST" {
		return messageCandidate{}, false
	}
	score := int(binary.LittleEndian.Uint16(payload[end+1 : end+3]))
	if score < 0 || score > 100 {
		return messageCandidate{}, false
	}
	return messageCandidate{
		position: position,
		kind:     candidateTeamScore,
		team:     team,
		score:    score,
	}, true
}

func variableUserMessagePayload(data []byte, position, maxLength int) ([]byte, bool) {
	if position+2 > len(data) {
		return nil, false
	}
	length := int(data[position+1])
	if length <= 0 || length > maxLength || position+2+length > len(data) {
		return nil, false
	}
	return data[position+2 : position+2+length], true
}

var knownTeam = map[string]bool{
	"CT": true, "TERRORIST": true, "SPECTATOR": true, "UNASSIGNED": true,
}

func (state *timelineState) parseDeathCandidate(data []byte, position int) (messageCandidate, bool) {
	if position+2 > len(data) {
		return messageCandidate{}, false
	}
	length := int(data[position+1])
	if length < 5 || length > 64 || position+2+length > len(data) {
		return messageCandidate{}, false
	}
	payload := data[position+2 : position+2+length]
	killer, victim, headshot := payload[0], payload[1], payload[2]
	if killer > state.maxClients || victim == 0 || victim > state.maxClients || headshot > 1 {
		return messageCandidate{}, false
	}
	weaponEnd := bytes.IndexByte(payload[3:], 0)
	if weaponEnd <= 0 {
		return messageCandidate{}, false
	}
	weaponBytes := payload[3 : 3+weaponEnd]
	weapon := string(weaponBytes)
	if !knownDeathWeapon[weapon] {
		return messageCandidate{}, false
	}
	return messageCandidate{
		position: position,
		kind:     candidateKill,
		killer:   killer,
		victim:   victim,
		headshot: headshot == 1,
		weapon:   weapon,
	}, true
}

var knownDeathWeapon = map[string]bool{
	"p228": true, "scout": true, "hegrenade": true, "xm1014": true,
	"c4": true, "mac10": true, "aug": true, "smokegrenade": true,
	"elite": true, "fiveseven": true, "ump45": true, "sg550": true,
	"galil": true, "famas": true, "usp": true, "glock18": true,
	"awp": true, "mp5navy": true, "m249": true, "m3": true,
	"m4a1": true, "tmp": true, "g3sg1": true, "flashbang": true,
	"deagle": true, "sg552": true, "ak47": true, "knife": true,
	"p90": true, "shield": true, "grenade": true, "world": true,
	"worldspawn": true, "trigger_hurt": true, "vehicle": true,
}

func deriveRounds(kills []Kill, scores []ScoreUpdate) []Round {
	if len(scores) == 0 {
		return nil
	}
	result := make([]Round, 0, len(scores))
	previousCT, previousT := 0, 0
	previousEnd := 0.0
	killIndex := 0
	startAt := 0
	// A second-half POV recording begins with the carried match score. Treat
	// that first TeamScore pair as a baseline, not as a newly completed round.
	if scores[0].CTScore+scores[0].TerroristScore > 1 {
		previousCT, previousT = scores[0].CTScore, scores[0].TerroristScore
		previousEnd = scores[0].TimeSeconds
		startAt = 1
		for killIndex < len(kills) && kills[killIndex].TimeSeconds <= previousEnd+0.001 {
			killIndex++
		}
	}
	for _, score := range scores[startAt:] {
		// HLTV recordings can contain multiple halves or matches. TeamScore is
		// reset in-place, so start a fresh scoreboard baseline when either side
		// drops instead of discarding every subsequent round.
		if score.CTScore < previousCT || score.TerroristScore < previousT {
			previousCT, previousT = 0, 0
		}
		if score.CTScore+score.TerroristScore <= previousCT+previousT {
			continue
		}
		winner := ""
		if score.CTScore > previousCT {
			winner = "CT"
		} else if score.TerroristScore > previousT {
			winner = "TERRORIST"
		}
		roundKills := make([]Kill, 0)
		for killIndex < len(kills) && kills[killIndex].TimeSeconds <= score.TimeSeconds+0.001 {
			if kills[killIndex].TimeSeconds >= previousEnd-0.001 {
				roundKills = append(roundKills, kills[killIndex])
			}
			killIndex++
		}
		result = append(result, Round{
			Number:         len(result) + 1,
			StartSeconds:   previousEnd,
			EndSeconds:     score.TimeSeconds,
			Winner:         winner,
			CTScore:        score.CTScore,
			TerroristScore: score.TerroristScore,
			Kills:          roundKills,
		})
		previousCT, previousT = score.CTScore, score.TerroristScore
		previousEnd = score.TimeSeconds
	}
	return result
}

func forEachNetworkFrame(r io.ReaderAt, networkProtocol uint32, entry DirectoryEntry, visit func(networkFrame) error) error {
	section := io.NewSectionReader(r, int64(entry.Offset), int64(entry.Length))
	reader := bufio.NewReaderSize(section, 256<<10)
	consumed := int64(0)
	segmentLength := int64(entry.Length)
	header := make([]byte, macroHeaderSize)

	for consumed < segmentLength {
		if segmentLength-consumed < macroHeaderSize {
			return fmt.Errorf("%d trailing bytes cannot contain a macro header", segmentLength-consumed)
		}
		if err := readFullCount(reader, header, &consumed); err != nil {
			return fmt.Errorf("read macro header: %w", err)
		}
		macroType := header[0]
		timeSeconds := math.Float32frombits(binary.LittleEndian.Uint32(header[1:5]))
		frameNumber := binary.LittleEndian.Uint32(header[5:9])

		switch macroType {
		case 0, 1:
			netInfoSize, err := netInfoSize(networkProtocol)
			if err != nil {
				return err
			}
			if err := skipCount(reader, int64(netInfoSize), &consumed); err != nil {
				return fmt.Errorf("skip network frame info: %w", err)
			}
			lengthOffset := int64(entry.Offset) + consumed
			lengthBytes := make([]byte, 4)
			if err := readFullCount(reader, lengthBytes, &consumed); err != nil {
				return fmt.Errorf("read network chunk length: %w", err)
			}
			chunkLength := int64(int32(binary.LittleEndian.Uint32(lengthBytes)))
			if chunkLength < 0 || chunkLength > maxNetworkChunkSize || chunkLength > segmentLength-consumed {
				return fmt.Errorf("invalid network chunk length %d", chunkLength)
			}
			dataOffset := int64(entry.Offset) + consumed
			data := make([]byte, int(chunkLength))
			if err := readFullCount(reader, data, &consumed); err != nil {
				return fmt.Errorf("read network chunk: %w", err)
			}
			if err := visit(networkFrame{time: float64(timeSeconds), frame: frameNumber, data: data, lengthOffset: lengthOffset, dataOffset: dataOffset}); err != nil {
				return err
			}
		case 2:
			// The original engine reserves this macro type without payload.
		case 3:
			if err := skipCount(reader, 64, &consumed); err != nil {
				return fmt.Errorf("skip console command macro: %w", err)
			}
		case 4:
			if err := skipCount(reader, 32, &consumed); err != nil {
				return fmt.Errorf("skip client data macro: %w", err)
			}
		case 5:
			return nil
		case 6:
			if err := skipCount(reader, 84, &consumed); err != nil {
				return fmt.Errorf("skip macro 6: %w", err)
			}
		case 7:
			if err := skipCount(reader, 8, &consumed); err != nil {
				return fmt.Errorf("skip macro 7: %w", err)
			}
		case 8:
			prefix := make([]byte, 8)
			if err := readFullCount(reader, prefix, &consumed); err != nil {
				return fmt.Errorf("read sound macro: %w", err)
			}
			nameLength := int64(binary.LittleEndian.Uint32(prefix[4:8]))
			if nameLength < 0 || nameLength > maxMacroStringSize {
				return fmt.Errorf("invalid sound name length %d", nameLength)
			}
			if err := skipCount(reader, nameLength+16, &consumed); err != nil {
				return fmt.Errorf("skip sound macro: %w", err)
			}
		case 9:
			lengthBytes := make([]byte, 4)
			if err := readFullCount(reader, lengthBytes, &consumed); err != nil {
				return fmt.Errorf("read macro 9 length: %w", err)
			}
			length := int64(int32(binary.LittleEndian.Uint32(lengthBytes)))
			if length < 0 || length > maxNetworkChunkSize {
				return fmt.Errorf("invalid macro 9 length %d", length)
			}
			if err := skipCount(reader, length, &consumed); err != nil {
				return fmt.Errorf("skip macro 9: %w", err)
			}
		default:
			return fmt.Errorf("unknown macro type %d after %d bytes", macroType, consumed)
		}
	}
	return nil
}

func netInfoSize(networkProtocol uint32) (int, error) {
	switch {
	case networkProtocol == 42:
		return protocol42NetInfoSize, nil
	case networkProtocol >= 45:
		return protocol45NetInfoSize, nil
	default:
		return 0, fmt.Errorf("unsupported network protocol %d for frame parsing", networkProtocol)
	}
}

func readFullCount(reader io.Reader, data []byte, consumed *int64) error {
	n, err := io.ReadFull(reader, data)
	*consumed += int64(n)
	return err
}

func skipCount(reader io.Reader, count int64, consumed *int64) error {
	if count < 0 {
		return fmt.Errorf("negative skip length %d", count)
	}
	n, err := io.CopyN(io.Discard, reader, count)
	*consumed += n
	return err
}
