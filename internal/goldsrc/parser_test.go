package goldsrc

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestParseFile(t *testing.T) {
	const directoryOffset = 600
	raw := make([]byte, directoryOffset+4+directoryEntrySize)
	copy(raw[0:8], []byte{'H', 'L', 'D', 'E', 'M', 'O', 0, 0})
	binary.LittleEndian.PutUint32(raw[8:12], 5)
	binary.LittleEndian.PutUint32(raw[12:16], 48)
	copy(raw[16:276], "de_dust2")
	copy(raw[276:536], "cstrike")
	binary.LittleEndian.PutUint32(raw[536:540], 0x1234abcd)
	binary.LittleEndian.PutUint32(raw[540:544], directoryOffset)
	raw[headerSize] = 5 // End-of-segment macro.

	binary.LittleEndian.PutUint32(raw[directoryOffset:directoryOffset+4], 1)
	entry := raw[directoryOffset+4:]
	copy(entry[4:68], "Playback")
	binary.LittleEndian.PutUint32(entry[68:72], 0xffffffff)
	binary.LittleEndian.PutUint32(entry[72:76], ^uint32(0))
	binary.LittleEndian.PutUint32(entry[76:80], math.Float32bits(90.5))
	binary.LittleEndian.PutUint32(entry[80:84], 9000)
	binary.LittleEndian.PutUint32(entry[84:88], headerSize)
	binary.LittleEndian.PutUint32(entry[88:92], macroHeaderSize)

	path := filepath.Join(t.TempDir(), "(CT)_de_dust2_2026.08.16-23.10.24.dem")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	demo, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if demo.Map != "de_dust2" || demo.GameDirectory != "cstrike" {
		t.Fatalf("unexpected header: %#v", demo)
	}
	if demo.DurationSeconds != 90.5 || demo.FrameCount != 9000 {
		t.Fatalf("unexpected playback metadata: %#v", demo.DirectoryEntries)
	}
	if demo.SideHint != "CT" || demo.RecordedAtHint != "2026-08-16T23:10:24" {
		t.Fatalf("unexpected filename hints: %q %q", demo.SideHint, demo.RecordedAtHint)
	}
}

func TestAddPlayerUpdate(t *testing.T) {
	players := make(map[string]*playerAccumulator)
	slot := uint8(9)
	userID := uint32(1035)
	raw := []byte(`\bottomcolor\6\name\Mature | KIM 4.5\*sid\76561199290899139\model\urban`)
	if !addPlayerUpdate(players, raw, &slot, &userID) {
		t.Fatal("update was not accepted")
	}
	player := players["steam:76561199290899139"].player
	if player.Name != "Mature | KIM 4.5" || player.Models[0] != "urban" || player.Slots[0] != 9 || player.UserIDs[0] != 1035 {
		t.Fatalf("unexpected player: %#v", player)
	}
}

func TestRejectsWrongMagic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.dem")
	if err := os.WriteFile(path, make([]byte, headerSize), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseFile(path); err == nil {
		t.Fatal("expected an error")
	}
}

func TestTimelineStateConsumesExtendedDeathMessage(t *testing.T) {
	state := &timelineState{maxClients: 20, slots: make(map[uint8]PlayerReference)}
	data := make([]byte, 0)
	registration := make([]byte, 19)
	registration[0], registration[1], registration[2] = 0x27, 0x53, 0xff
	copy(registration[3:], "DeathMsg")
	data = append(data, registration...)
	data = appendUserInfoMessage(data, 0, 100, `\bottomcolor\6\name\Killer\*sid\111\model\urban`)
	data = appendUserInfoMessage(data, 1, 101, `\bottomcolor\6\name\Victim\*sid\222\model\leet`)
	payload := append([]byte{1, 2, 1}, []byte("ak47\x00")...)
	payload = append(payload, bytes.Repeat([]byte{0x05}, 10)...)
	data = append(data, 0x53, byte(len(payload)))
	data = append(data, payload...)

	state.consume(networkFrame{time: 12.5, frame: 1250, data: data})
	if len(state.kills) != 1 {
		t.Fatalf("expected one kill, got %#v", state.kills)
	}
	kill := state.kills[0]
	if kill.Killer == nil || kill.Killer.Name != "Killer" || kill.Victim.Name != "Victim" || kill.Weapon != "ak47" || !kill.Headshot {
		t.Fatalf("unexpected kill: %#v", kill)
	}
}

func TestTimelineStateDetectsHLTVProxy(t *testing.T) {
	state := &timelineState{maxClients: 12, slots: make(map[uint8]PlayerReference)}
	data := appendUserInfoMessage(nil, 3, 5, `\cl_lw\1\cl_lc\1\*hltv\1\hdelay\90\name\HLTV.org - VeryGames.net\*sid\90071996842377216\model\urban\hslots\255`)
	state.consume(networkFrame{data: data})
	if state.hltv == nil {
		t.Fatal("expected an HLTV proxy")
	}
	if state.hltv.Name != "HLTV.org - VeryGames.net" || state.hltv.Slot != 3 || state.hltv.DelaySeconds != 90 || state.hltv.SpectatorSlots != 255 {
		t.Fatalf("unexpected HLTV proxy: %#v", state.hltv)
	}
}

func TestTimelineStateConsumesTeamsAndScore(t *testing.T) {
	state := &timelineState{maxClients: 20}
	data := make([]byte, 0)
	data = append(data, userMessageRegistration(0x56, "TeamInfo")...)
	data = append(data, userMessageRegistration(0x57, "TeamScore")...)
	data = appendUserInfoMessage(data, 0, 100, `\bottomcolor\6\name\Player\*sid\111\model\urban`)
	data = append(data, 0x56, 4, 1, 'C', 'T', 0)
	data = append(data, 0x57, 5, 'C', 'T', 0, 1, 0)
	data = append(data, 0x57, 12)
	data = append(data, []byte("TERRORIST\x00")...)
	data = append(data, 0, 0)

	state.consume(networkFrame{time: 80, frame: 8000, data: data})
	if len(state.teamChanges) != 1 || state.teamChanges[0].Player.Name != "Player" || state.teamChanges[0].Team != "CT" {
		t.Fatalf("unexpected team changes: %#v", state.teamChanges)
	}
	if len(state.scoreUpdates) != 1 || state.scoreUpdates[0].CTScore != 1 || state.scoreUpdates[0].TerroristScore != 0 {
		t.Fatalf("unexpected score updates: %#v", state.scoreUpdates)
	}
}

func TestDeriveRoundsSkipsCarriedSecondHalfScore(t *testing.T) {
	scores := []ScoreUpdate{
		{TimeSeconds: 0.1, CTScore: 8, TerroristScore: 7},
		{TimeSeconds: 70, CTScore: 8, TerroristScore: 8},
	}
	rounds := deriveRounds(nil, scores)
	if len(rounds) != 1 || rounds[0].Number != 1 || rounds[0].Winner != "TERRORIST" {
		t.Fatalf("unexpected rounds: %#v", rounds)
	}
}

func TestDeriveRoundsContinuesAfterScoreReset(t *testing.T) {
	scores := []ScoreUpdate{
		{TimeSeconds: 60, CTScore: 1, TerroristScore: 0},
		{TimeSeconds: 120, CTScore: 1, TerroristScore: 1},
		{TimeSeconds: 180, CTScore: 0, TerroristScore: 1},
		{TimeSeconds: 240, CTScore: 1, TerroristScore: 1},
	}
	rounds := deriveRounds(nil, scores)
	if len(rounds) != 4 {
		t.Fatalf("expected four rounds across a score reset, got %#v", rounds)
	}
	if rounds[2].Winner != "TERRORIST" || rounds[3].Winner != "CT" {
		t.Fatalf("unexpected winners after reset: %#v", rounds)
	}
}

func userMessageRegistration(id byte, name string) []byte {
	registration := make([]byte, 19)
	registration[0], registration[1], registration[2] = 0x27, id, 0xff
	copy(registration[3:], name)
	return registration
}

func appendUserInfoMessage(data []byte, slot uint8, userID uint32, userinfo string) []byte {
	data = append(data, 0x0d, slot)
	userIDBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(userIDBytes, userID)
	data = append(data, userIDBytes...)
	data = append(data, userinfo...)
	return append(data, 0)
}
