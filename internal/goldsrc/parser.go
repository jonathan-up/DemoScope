// Package goldsrc reads metadata from GoldSrc (Half-Life 1 / Counter-Strike 1.6)
// POV demo files.
package goldsrc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	headerSize         = 544
	directoryEntrySize = 92
	maxDirectoryCount  = 1024
	serverScanLimit    = 4 << 20
	maxUserInfoLength  = 8 << 10
)

var demoNamePattern = regexp.MustCompile(`^\((CT|T)\)_(.+)_(\d{4}\.\d{2}\.\d{2}-\d{2}\.\d{2}\.\d{2})\.dem$`)

// Demo is the metadata extracted from one demo file.
type Demo struct {
	Path             string           `json:"path"`
	FileSize         int64            `json:"file_size"`
	Format           string           `json:"format"`
	DemoProtocol     uint32           `json:"demo_protocol"`
	NetworkProtocol  uint32           `json:"network_protocol"`
	Map              string           `json:"map"`
	GameDirectory    string           `json:"game_directory"`
	MapCRC32         string           `json:"map_crc32"`
	DirectoryOffset  uint32           `json:"directory_offset"`
	DurationSeconds  float64          `json:"duration_seconds"`
	FrameCount       uint32           `json:"frame_count"`
	RecordingType    string           `json:"recording_type"`
	SideHint         string           `json:"side_hint,omitempty"`
	RecordedAtHint   string           `json:"recorded_at_hint,omitempty"`
	Server           *Server          `json:"server,omitempty"`
	HLTVProxy        *HLTVProxy       `json:"hltv_proxy,omitempty"`
	POVPlayer        *PlayerReference `json:"pov_player,omitempty"`
	Players          []Player         `json:"players,omitempty"`
	UserInfoUpdates  int              `json:"userinfo_updates"`
	Kills            []Kill           `json:"kills,omitempty"`
	TeamChanges      []TeamChange     `json:"team_changes,omitempty"`
	ScoreUpdates     []ScoreUpdate    `json:"score_updates,omitempty"`
	Rounds           []Round          `json:"rounds,omitempty"`
	DirectoryEntries []DirectoryEntry `json:"directory_entries"`
}

// Server is server metadata carried by svc_serverinfo.
type Server struct {
	Name        string `json:"name,omitempty"`
	Count       uint32 `json:"count"`
	CRC32       string `json:"crc32"`
	MaxClients  uint8  `json:"max_clients"`
	ClientSlot  uint8  `json:"client_slot_zero_based"`
	MapFile     string `json:"map_file,omitempty"`
	MapChecksum string `json:"map_checksum,omitempty"`
}

// PlayerReference identifies the player whose client recorded the POV demo.
type PlayerReference struct {
	Name      string `json:"name"`
	SteamID64 string `json:"steam_id64,omitempty"`
	Slot      uint8  `json:"slot_zero_based"`
	Team      string `json:"team,omitempty"`
}

// Player is a player observed in an updateuserinfo network message.
type Player struct {
	Name      string   `json:"name"`
	Aliases   []string `json:"aliases,omitempty"`
	SteamID64 string   `json:"steam_id64,omitempty"`
	Team      string   `json:"team,omitempty"`
	Models    []string `json:"models,omitempty"`
	Slots     []int    `json:"slots_zero_based,omitempty"`
	UserIDs   []uint32 `json:"user_ids,omitempty"`
	IsPOV     bool     `json:"is_pov,omitempty"`
}

// DirectoryEntry describes one loading or playback segment in the demo.
type DirectoryEntry struct {
	Number      uint32  `json:"number"`
	Title       string  `json:"title"`
	Flags       uint32  `json:"flags"`
	Play        int32   `json:"play"`
	TimeSeconds float64 `json:"time_seconds"`
	Frames      uint32  `json:"frames"`
	Offset      uint32  `json:"offset"`
	Length      uint32  `json:"length"`
}

// ParseFile reads metadata from path without loading the whole demo into memory.
func ParseFile(path string) (*Demo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open demo: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat demo: %w", err)
	}
	if info.Size() < headerSize {
		return nil, fmt.Errorf("demo is too small: got %d bytes, need at least %d", info.Size(), headerSize)
	}

	header := make([]byte, headerSize)
	if _, err := io.ReadFull(f, header); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	if !bytes.Equal(header[:6], []byte("HLDEMO")) {
		return nil, fmt.Errorf("unsupported demo magic %q: expected GoldSrc HLDEMO", trimCString(header[:8]))
	}

	demo := &Demo{
		Path:            filepath.Clean(path),
		FileSize:        info.Size(),
		Format:          "goldsrc",
		DemoProtocol:    binary.LittleEndian.Uint32(header[8:12]),
		NetworkProtocol: binary.LittleEndian.Uint32(header[12:16]),
		Map:             trimCString(header[16:276]),
		GameDirectory:   trimCString(header[276:536]),
		MapCRC32:        hex32(binary.LittleEndian.Uint32(header[536:540])),
		DirectoryOffset: binary.LittleEndian.Uint32(header[540:544]),
	}
	demo.SideHint, demo.RecordedAtHint = parseFilenameHints(filepath.Base(path))

	entries, err := readDirectory(f, info.Size(), demo.DirectoryOffset)
	if err != nil {
		return nil, err
	}
	demo.DirectoryEntries = entries
	for _, entry := range entries {
		if entry.TimeSeconds > demo.DurationSeconds {
			demo.DurationSeconds = entry.TimeSeconds
			demo.FrameCount = entry.Frames
		}
	}

	demo.Server = findServerInfo(f, info.Size(), demo)
	players, updates, err := scanPlayers(f)
	if err != nil {
		return nil, fmt.Errorf("scan player userinfo: %w", err)
	}
	demo.UserInfoUpdates = updates
	demo.Players = players
	timeline, err := scanTimeline(f, demo.NetworkProtocol, demo.DirectoryEntries, demo.Server)
	if err != nil {
		return nil, fmt.Errorf("scan match events: %w", err)
	}
	demo.Kills = timeline.kills
	demo.TeamChanges = timeline.teamChanges
	demo.ScoreUpdates = timeline.scoreUpdates
	demo.Rounds = deriveRounds(demo.Kills, demo.ScoreUpdates)
	demo.HLTVProxy = timeline.hltv
	mergeTimelineTeams(demo.Players, timeline.slots)
	if demo.HLTVProxy != nil {
		demo.RecordingType = "hltv"
	} else {
		demo.RecordingType = "pov"
		markPOVPlayer(demo)
	}

	return demo, nil
}

func mergeTimelineTeams(players []Player, slots map[uint8]PlayerReference) {
	for index := range players {
		for _, slot := range players[index].Slots {
			reference, ok := slots[uint8(slot)]
			if !ok || reference.Team == "" || reference.Team == "UNASSIGNED" {
				continue
			}
			if players[index].SteamID64 != "" && reference.SteamID64 != "" && players[index].SteamID64 != reference.SteamID64 {
				continue
			}
			players[index].Team = reference.Team
		}
	}
}

func readDirectory(r io.ReaderAt, fileSize int64, offset uint32) ([]DirectoryEntry, error) {
	if int64(offset) < headerSize || int64(offset)+4 > fileSize {
		return nil, fmt.Errorf("invalid directory offset %d for %d-byte file", offset, fileSize)
	}

	countBytes := make([]byte, 4)
	if _, err := r.ReadAt(countBytes, int64(offset)); err != nil {
		return nil, fmt.Errorf("read directory count: %w", err)
	}
	count := binary.LittleEndian.Uint32(countBytes)
	if count == 0 || count > maxDirectoryCount {
		return nil, fmt.Errorf("invalid directory entry count %d", count)
	}
	byteCount := int64(count) * directoryEntrySize
	if int64(offset)+4+byteCount > fileSize {
		return nil, fmt.Errorf("directory with %d entries extends beyond end of file", count)
	}

	raw := make([]byte, byteCount)
	if _, err := r.ReadAt(raw, int64(offset)+4); err != nil {
		return nil, fmt.Errorf("read directory entries: %w", err)
	}
	entries := make([]DirectoryEntry, 0, count)
	for i := uint32(0); i < count; i++ {
		data := raw[int(i)*directoryEntrySize : int(i+1)*directoryEntrySize]
		seconds := math.Float32frombits(binary.LittleEndian.Uint32(data[76:80]))
		if math.IsNaN(float64(seconds)) || math.IsInf(float64(seconds), 0) || seconds < 0 {
			return nil, fmt.Errorf("directory entry %d has invalid playback time", i)
		}
		entry := DirectoryEntry{
			Number:      binary.LittleEndian.Uint32(data[0:4]),
			Title:       trimCString(data[4:68]),
			Flags:       binary.LittleEndian.Uint32(data[68:72]),
			Play:        int32(binary.LittleEndian.Uint32(data[72:76])),
			TimeSeconds: float64(seconds),
			Frames:      binary.LittleEndian.Uint32(data[80:84]),
			Offset:      binary.LittleEndian.Uint32(data[84:88]),
			Length:      binary.LittleEndian.Uint32(data[88:92]),
		}
		if int64(entry.Offset)+int64(entry.Length) > fileSize {
			return nil, fmt.Errorf("directory entry %d points beyond end of file", i)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func findServerInfo(r io.ReaderAt, fileSize int64, demo *Demo) *Server {
	length := int64(serverScanLimit)
	if fileSize-headerSize < length {
		length = fileSize - headerSize
	}
	if length <= 0 {
		return nil
	}
	raw := make([]byte, length)
	n, err := r.ReadAt(raw, headerSize)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil
	}
	raw = raw[:n]

	protocol := make([]byte, 4)
	binary.LittleEndian.PutUint32(protocol, demo.NetworkProtocol)
	needle := append([]byte{0x0b}, protocol...)
	for from := 0; from < len(raw); {
		relative := bytes.Index(raw[from:], needle)
		if relative < 0 {
			return nil
		}
		position := from + relative
		server, ok := parseServerInfoCandidate(raw[position:], demo)
		if ok {
			return server
		}
		from = position + 1
	}
	return nil
}

func parseServerInfoCandidate(raw []byte, demo *Demo) (*Server, bool) {
	// id + version/count/crc + client DLL hash + maxclients/playernum/unknown
	if len(raw) < 33 {
		return nil, false
	}
	maxClients := raw[29]
	povSlot := raw[30]
	if maxClients == 0 || maxClients > 64 || povSlot >= maxClients {
		return nil, false
	}

	position := 32
	gameDir, next, ok := readCString(raw, position, 260)
	if !ok || gameDir != demo.GameDirectory {
		return nil, false
	}
	position = next
	serverName := ""
	if demo.NetworkProtocol >= 45 {
		serverName, position, ok = readCString(raw, position, 1024)
		if !ok {
			return nil, false
		}
	}
	mapFile, position, ok := readCString(raw, position, 512)
	if !ok || !strings.Contains(mapFile, demo.Map) {
		return nil, false
	}
	mapChecksum, _, ok := readCString(raw, position, 512)
	if !ok {
		return nil, false
	}

	return &Server{
		Name:        cleanText(serverName),
		Count:       binary.LittleEndian.Uint32(raw[5:9]),
		CRC32:       hex32(binary.LittleEndian.Uint32(raw[9:13])),
		MaxClients:  maxClients,
		ClientSlot:  povSlot,
		MapFile:     cleanText(mapFile),
		MapChecksum: cleanText(mapChecksum),
	}, true
}

type playerAccumulator struct {
	player    Player
	aliasSet  map[string]bool
	modelSet  map[string]bool
	slotSet   map[int]bool
	userIDSet map[uint32]bool
}

func scanPlayers(f *os.File) ([]Player, int, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, 0, err
	}
	const chunkSize = 64 << 10
	marker := []byte(`\bottomcolor\`)
	overlap := len(marker) + 6
	chunk := make([]byte, chunkSize)
	buffer := make([]byte, 0, chunkSize+maxUserInfoLength)
	players := make(map[string]*playerAccumulator)
	updates := 0

	for {
		n, readErr := f.Read(chunk)
		if n > 0 {
			buffer = append(buffer, chunk[:n]...)
			for {
				position := bytes.Index(buffer, marker)
				if position < 0 {
					if len(buffer) > overlap {
						buffer = append(buffer[:0], buffer[len(buffer)-overlap:]...)
					}
					break
				}
				endRelative := bytes.IndexByte(buffer[position:], 0)
				if endRelative < 0 {
					if len(buffer)-position > maxUserInfoLength {
						buffer = buffer[position+len(marker):]
						continue
					}
					start := position - 6
					if start < 0 {
						start = 0
					}
					buffer = append(buffer[:0], buffer[start:]...)
					break
				}

				end := position + endRelative
				userinfo := buffer[position:end]
				var slot *uint8
				var userID *uint32
				if position >= 6 && buffer[position-6] == 0x0d {
					slotValue := buffer[position-5]
					userIDValue := binary.LittleEndian.Uint32(buffer[position-4 : position])
					slot = &slotValue
					userID = &userIDValue
				}
				if addPlayerUpdate(players, userinfo, slot, userID) {
					updates++
				}
				buffer = buffer[end+1:]
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return nil, 0, readErr
		}
	}

	result := make([]Player, 0, len(players))
	for _, accumulator := range players {
		sort.Strings(accumulator.player.Aliases)
		sort.Strings(accumulator.player.Models)
		sort.Slice(accumulator.player.Slots, func(i, j int) bool { return accumulator.player.Slots[i] < accumulator.player.Slots[j] })
		sort.Slice(accumulator.player.UserIDs, func(i, j int) bool { return accumulator.player.UserIDs[i] < accumulator.player.UserIDs[j] })
		result = append(result, accumulator.player)
	}
	sort.Slice(result, func(i, j int) bool {
		if len(result[i].Slots) > 0 && len(result[j].Slots) > 0 && result[i].Slots[0] != result[j].Slots[0] {
			return result[i].Slots[0] < result[j].Slots[0]
		}
		return result[i].SteamID64 < result[j].SteamID64
	})
	return result, updates, nil
}

func addPlayerUpdate(players map[string]*playerAccumulator, raw []byte, slot *uint8, userID *uint32) bool {
	fields := parseInfoString(raw)
	name := cleanText(fields["name"])
	steamID64 := cleanText(fields["*sid"])
	model := cleanText(fields["model"])
	if name == "" || (steamID64 == "" && model == "") {
		return false
	}
	key := "name:" + name
	if steamID64 != "" {
		key = "steam:" + steamID64
	}
	accumulator := players[key]
	if accumulator == nil {
		accumulator = &playerAccumulator{
			player:    Player{Name: name, SteamID64: steamID64},
			aliasSet:  make(map[string]bool),
			modelSet:  make(map[string]bool),
			slotSet:   make(map[int]bool),
			userIDSet: make(map[uint32]bool),
		}
		players[key] = accumulator
	} else if name != accumulator.player.Name && !accumulator.aliasSet[name] {
		accumulator.aliasSet[name] = true
		accumulator.player.Aliases = append(accumulator.player.Aliases, name)
	}
	if model != "" && !accumulator.modelSet[model] {
		accumulator.modelSet[model] = true
		accumulator.player.Models = append(accumulator.player.Models, model)
	}
	if slot != nil && !accumulator.slotSet[int(*slot)] {
		accumulator.slotSet[int(*slot)] = true
		accumulator.player.Slots = append(accumulator.player.Slots, int(*slot))
	}
	if userID != nil && !accumulator.userIDSet[*userID] {
		accumulator.userIDSet[*userID] = true
		accumulator.player.UserIDs = append(accumulator.player.UserIDs, *userID)
	}
	return true
}

func markPOVPlayer(demo *Demo) {
	if demo.Server == nil {
		return
	}
	for i := range demo.Players {
		for _, slot := range demo.Players[i].Slots {
			if slot == int(demo.Server.ClientSlot) {
				demo.Players[i].IsPOV = true
				demo.POVPlayer = &PlayerReference{
					Name:      demo.Players[i].Name,
					SteamID64: demo.Players[i].SteamID64,
					Slot:      uint8(slot),
					Team:      demo.Players[i].Team,
				}
				return
			}
		}
	}
}

func parseInfoString(raw []byte) map[string]string {
	parts := strings.Split(strings.ToValidUTF8(string(raw), "�"), `\`)
	fields := make(map[string]string)
	for i := 1; i+1 < len(parts); i += 2 {
		fields[parts[i]] = parts[i+1]
	}
	return fields
}

func parseFilenameHints(name string) (string, string) {
	matches := demoNamePattern.FindStringSubmatch(name)
	if matches == nil {
		return "", ""
	}
	recordedAt, err := time.Parse("2006.01.02-15.04.05", matches[3])
	if err != nil {
		return matches[1], ""
	}
	return matches[1], recordedAt.Format("2006-01-02T15:04:05")
}

func readCString(raw []byte, start, limit int) (string, int, bool) {
	if start < 0 || start >= len(raw) {
		return "", start, false
	}
	endLimit := start + limit
	if endLimit > len(raw) {
		endLimit = len(raw)
	}
	relative := bytes.IndexByte(raw[start:endLimit], 0)
	if relative < 0 {
		return "", start, false
	}
	end := start + relative
	return cleanText(string(raw[start:end])), end + 1, true
}

func trimCString(raw []byte) string {
	if end := bytes.IndexByte(raw, 0); end >= 0 {
		raw = raw[:end]
	}
	return cleanText(string(raw))
}

func cleanText(value string) string {
	if !utf8.ValidString(value) {
		value = strings.ToValidUTF8(value, "�")
	}
	value = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, value)
	return strings.TrimSpace(value)
}

func hex32(value uint32) string {
	return fmt.Sprintf("%08x", value)
}
