package goldsrc

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestRewritePlayerFileUpdatesAllUserinfoAndDirectory(t *testing.T) {
	const oldID = "76561198000000001"
	const newID = "76561198000000002"
	otherID := "76561198000000003"
	firstChunk := appendUserInfoMessage(nil, 0, 101, `\bottomcolor\6\name\Old\*sid\`+oldID+`\model\urban`)
	firstChunk = appendUserInfoMessage(firstChunk, 1, 102, `\bottomcolor\6\name\Other\*sid\`+otherID+`\model\leet`)
	secondChunk := appendUserInfoMessage(nil, 0, 101, `\bottomcolor\6\name\Old\*sid\`+oldID+`\model\urban`)
	secondChunk = append(secondChunk, userMessageRegistration(0x53, "DeathMsg")...)
	killPayload := append([]byte{1, 2, 0}, []byte("ak47\x00")...)
	secondChunk = append(secondChunk, 0x53, byte(len(killPayload)))
	secondChunk = append(secondChunk, killPayload...)
	segments := [][]byte{testNetworkSegment(firstChunk), testNetworkSegment(secondChunk)}
	original := testDemoWithSegments(segments)
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source.dem")
	outputPath := filepath.Join(root, "edited.dem")
	if err := os.WriteFile(sourcePath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	result, updates, err := RewritePlayerFile(sourcePath, outputPath, PlayerReplacement{
		SourceSteamID64: oldID, Name: "Longer Player Name", SteamID64: newID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updates != 2 {
		t.Fatalf("updated %d userinfo messages, want 2", updates)
	}
	if len(result.Players) != 2 || result.Players[0].Name != "Longer Player Name" || result.Players[0].SteamID64 != newID || result.Players[1].Name != "Other" || result.Players[1].SteamID64 != otherID {
		t.Fatalf("unexpected players: %#v", result.Players)
	}
	if len(result.Kills) != 1 || result.Kills[0].Killer == nil || result.Kills[0].Killer.Name != "Longer Player Name" || result.Kills[0].Killer.SteamID64 != newID || result.Kills[0].Victim.Name != "Other" {
		t.Fatalf("unexpected kill after rewrite: %#v", result.Kills)
	}
	if result.DirectoryEntries[1].Offset <= uint32(headerSize+len(segments[0])) || result.DirectoryOffset <= uint32(headerSize+len(segments[0])+len(segments[1])) {
		t.Fatalf("directory positions were not shifted: %#v", result.DirectoryEntries)
	}
	currentSource, err := os.ReadFile(sourcePath)
	if err != nil || !bytes.Equal(currentSource, original) {
		t.Fatal("source demo changed")
	}
	currentOutput, err := os.ReadFile(outputPath)
	if err != nil || bytes.Contains(currentOutput, []byte(oldID)) {
		t.Fatal("old SteamID remains in output")
	}
	shorterPath := filepath.Join(root, "shorter.dem")
	shorter, shorterUpdates, err := RewritePlayerFile(outputPath, shorterPath, PlayerReplacement{
		SourceSteamID64: newID, Name: "X", SteamID64: oldID,
	})
	if err != nil || shorterUpdates != 2 || shorter.DirectoryOffset >= result.DirectoryOffset || shorter.Players[0].Name != "X" {
		t.Fatalf("shorter replacement failed: updates=%d result=%#v err=%v", shorterUpdates, shorter, err)
	}
}

func TestRewritePlayerFileRejectsExistingOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "demo.dem")
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := RewritePlayerFile(path, path, PlayerReplacement{SourceSteamID64: "76561198000000001", Name: "New", SteamID64: "76561198000000002"})
	if err == nil {
		t.Fatal("expected a refusal to overwrite the source")
	}
	current, _ := os.ReadFile(path)
	if string(current) != "keep" {
		t.Fatal("source was overwritten")
	}
}

func TestRewritePlayerPrefixPreservesSteamIDAndAliases(t *testing.T) {
	const targetID = "76561198000000001"
	const otherID = "76561198000000003"
	chunk := appendUserInfoMessage(nil, 0, 101, `\bottomcolor\6\name\Old\*sid\`+targetID+`\model\urban`)
	chunk = appendUserInfoMessage(chunk, 1, 102, `\bottomcolor\6\name\Other\*sid\`+otherID+`\model\leet`)
	chunk = appendUserInfoMessage(chunk, 0, 101, `\bottomcolor\6\name\New\*sid\`+targetID+`\model\urban`)
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source.dem")
	outputPath := filepath.Join(root, "prefixed.dem")
	if err := os.WriteFile(sourcePath, testDemoWithSegments([][]byte{testNetworkSegment(chunk)}), 0o600); err != nil {
		t.Fatal(err)
	}
	result, updates, err := RewritePlayerPrefixFile(sourcePath, outputPath, targetID, "[X] ")
	if err != nil {
		t.Fatal(err)
	}
	if updates != 2 || len(result.Players) != 2 || result.Players[0].Name != "[X] Old" || len(result.Players[0].Aliases) != 1 || result.Players[0].Aliases[0] != "[X] New" || result.Players[0].SteamID64 != targetID || result.Players[1].Name != "Other" {
		t.Fatalf("unexpected prefixed players: updates=%d players=%#v", updates, result.Players)
	}
	secondPath := filepath.Join(root, "again.dem")
	if _, _, err := RewritePlayerPrefixFile(outputPath, secondPath, targetID, "[X] "); err == nil {
		t.Fatal("expected already-prefixed names to be skipped")
	}
	if _, err := os.Stat(secondPath); !os.IsNotExist(err) {
		t.Fatal("created a second output without changes")
	}
}

func TestRewritePlayersPrefixFileUpdatesSelectedPlayersTogether(t *testing.T) {
	const firstID = "76561198000000001"
	const secondID = "76561198000000002"
	const thirdID = "76561198000000003"
	chunk := appendUserInfoMessage(nil, 0, 101, `\bottomcolor\6\name\First\*sid\`+firstID+`\model\urban`)
	chunk = appendUserInfoMessage(chunk, 1, 102, `\bottomcolor\6\name\Second\*sid\`+secondID+`\model\leet`)
	chunk = appendUserInfoMessage(chunk, 2, 103, `\bottomcolor\6\name\Third\*sid\`+thirdID+`\model\gign`)
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source.dem")
	outputPath := filepath.Join(root, "prefixed.dem")
	if err := os.WriteFile(sourcePath, testDemoWithSegments([][]byte{testNetworkSegment(chunk)}), 0o600); err != nil {
		t.Fatal(err)
	}
	result, updates, err := RewritePlayersPrefixFile(sourcePath, outputPath, []string{firstID, secondID}, "[A] ")
	if err != nil {
		t.Fatal(err)
	}
	if updates != 2 || len(result.Players) != 3 || result.Players[0].Name != "[A] First" || result.Players[1].Name != "[A] Second" || result.Players[2].Name != "Third" || result.Players[0].SteamID64 != firstID || result.Players[1].SteamID64 != secondID || result.Players[2].SteamID64 != thirdID {
		t.Fatalf("unexpected bulk prefix result: updates=%d players=%#v", updates, result.Players)
	}
}

func TestRewritePlayerPrefixRejectsTooLongName(t *testing.T) {
	const targetID = "76561198000000001"
	chunk := appendUserInfoMessage(nil, 0, 101, `\bottomcolor\6\name\ABCDEFGHIJKLMNOPQRSTUVWXYZ123\*sid\`+targetID+`\model\urban`)
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source.dem")
	outputPath := filepath.Join(root, "prefixed.dem")
	if err := os.WriteFile(sourcePath, testDemoWithSegments([][]byte{testNetworkSegment(chunk)}), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := RewritePlayerPrefixFile(sourcePath, outputPath, targetID, "[X] "); err == nil {
		t.Fatal("expected a name length error")
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatal("created an output after name length validation failed")
	}
}

func testNetworkSegment(chunk []byte) []byte {
	frame := make([]byte, macroHeaderSize+protocol45NetInfoSize+4+len(chunk))
	frame[0] = 1
	binary.LittleEndian.PutUint32(frame[1:5], math.Float32bits(1))
	binary.LittleEndian.PutUint32(frame[5:9], 1)
	lengthAt := macroHeaderSize + protocol45NetInfoSize
	binary.LittleEndian.PutUint32(frame[lengthAt:lengthAt+4], uint32(len(chunk)))
	copy(frame[lengthAt+4:], chunk)
	end := make([]byte, macroHeaderSize)
	end[0] = 5
	return append(frame, end...)
}

func testDemoWithSegments(segments [][]byte) []byte {
	offset := headerSize
	for _, segment := range segments {
		offset += len(segment)
	}
	directoryOffset := offset
	raw := make([]byte, directoryOffset+4+len(segments)*directoryEntrySize)
	copy(raw[0:8], []byte{'H', 'L', 'D', 'E', 'M', 'O', 0, 0})
	binary.LittleEndian.PutUint32(raw[8:12], 5)
	binary.LittleEndian.PutUint32(raw[12:16], 48)
	copy(raw[16:276], "de_dust2")
	copy(raw[276:536], "cstrike")
	binary.LittleEndian.PutUint32(raw[540:544], uint32(directoryOffset))
	binary.LittleEndian.PutUint32(raw[directoryOffset:directoryOffset+4], uint32(len(segments)))
	offset = headerSize
	for i, segment := range segments {
		copy(raw[offset:], segment)
		entry := raw[directoryOffset+4+i*directoryEntrySize:]
		binary.LittleEndian.PutUint32(entry[0:4], uint32(i))
		copy(entry[4:68], "Playback")
		binary.LittleEndian.PutUint32(entry[76:80], math.Float32bits(1))
		binary.LittleEndian.PutUint32(entry[84:88], uint32(offset))
		binary.LittleEndian.PutUint32(entry[88:92], uint32(len(segment)))
		offset += len(segment)
	}
	return raw
}
