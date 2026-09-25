package goldsrc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// PlayerReplacement changes every recorded userinfo update for one SteamID64.
// The source demo is never modified; the result is written to a new file.
type PlayerReplacement struct {
	SourceSteamID64 string
	Name            string
	SteamID64       string
}

type fileEdit struct {
	start int64
	end   int64
	data  []byte
}

func RewritePlayerFile(sourcePath, outputPath string, replacement PlayerReplacement) (*Demo, int, error) {
	if err := validateReplacement(replacement); err != nil {
		return nil, 0, err
	}
	return rewritePlayerFile(sourcePath, outputPath, replacement, "", nil)
}

// RewritePlayerPrefixFile adds a prefix to every recorded name for one player.
// It preserves that player's SteamID64 and any distinct names used over time.
func RewritePlayerPrefixFile(sourcePath, outputPath, sourceSteamID64, prefix string) (*Demo, int, error) {
	return RewritePlayersPrefixFile(sourcePath, outputPath, []string{sourceSteamID64}, prefix)
}

// RewritePlayersPrefixFile applies a prefix to all selected SteamID64s in one
// pass, preserving each player's SteamID64 and recorded name changes.
func RewritePlayersPrefixFile(sourcePath, outputPath string, sourceSteamID64s []string, prefix string) (*Demo, int, error) {
	if len(sourceSteamID64s) == 0 {
		return nil, 0, fmt.Errorf("请至少选择一名玩家")
	}
	targets := make(map[string]bool, len(sourceSteamID64s))
	for _, steamID64 := range sourceSteamID64s {
		if steamID64 == "" || strings.ContainsAny(steamID64, "\\\x00") {
			return nil, 0, fmt.Errorf("请选择有效的原 SteamID64")
		}
		targets[steamID64] = true
	}
	if !utf8.ValidString(prefix) || strings.TrimSpace(prefix) == "" || strings.TrimLeftFunc(prefix, unicode.IsSpace) != prefix || len([]byte(prefix)) > 30 || strings.ContainsRune(prefix, '\\') {
		return nil, 0, fmt.Errorf("前缀须为 1–30 字节，不能以空格开头或包含反斜杠")
	}
	for _, character := range prefix {
		if character < 0x20 || character == 0x7f {
			return nil, 0, fmt.Errorf("前缀不能包含控制字符")
		}
	}
	return rewritePlayerFile(sourcePath, outputPath, PlayerReplacement{}, prefix, targets)
}

func rewritePlayerFile(sourcePath, outputPath string, replacement PlayerReplacement, prefix string, targets map[string]bool) (*Demo, int, error) {
	sourcePath, err := filepath.Abs(sourcePath)
	if err != nil {
		return nil, 0, err
	}
	outputPath, err = filepath.Abs(outputPath)
	if err != nil {
		return nil, 0, err
	}
	if !strings.EqualFold(filepath.Ext(outputPath), ".dem") {
		return nil, 0, fmt.Errorf("输出文件必须是 .dem")
	}
	if strings.EqualFold(sourcePath, outputPath) {
		return nil, 0, fmt.Errorf("请选择与原文件不同的输出路径")
	}
	if _, err := os.Stat(outputPath); err == nil {
		return nil, 0, fmt.Errorf("输出文件已存在，请选择新的文件名")
	} else if !os.IsNotExist(err) {
		return nil, 0, err
	}

	demo, err := ParseFile(sourcePath)
	if err != nil {
		return nil, 0, err
	}
	foundPlayers := make(map[string]bool)
	for _, player := range demo.Players {
		foundPlayers[player.SteamID64] = true
	}
	if prefix == "" && !foundPlayers[replacement.SourceSteamID64] {
		return nil, 0, fmt.Errorf("Demo 中没有 SteamID64 为 %s 的玩家", replacement.SourceSteamID64)
	}
	for steamID64 := range targets {
		if !foundPlayers[steamID64] {
			return nil, 0, fmt.Errorf("Demo 中没有 SteamID64 为 %s 的玩家", steamID64)
		}
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return nil, 0, err
	}
	defer source.Close()
	edits, updates, err := playerFileEdits(source, demo, replacement, prefix, targets)
	if err != nil {
		return nil, 0, err
	}
	if updates == 0 {
		if prefix != "" {
			return nil, 0, fmt.Errorf("选中的玩家没有可添加此前缀的名称")
		}
		return nil, 0, fmt.Errorf("没有找到可修改的玩家信息更新")
	}
	if err := writeEditedFile(source, outputPath, edits); err != nil {
		return nil, 0, err
	}
	result, err := ParseFile(outputPath)
	if err != nil {
		os.Remove(outputPath)
		return nil, 0, fmt.Errorf("验证修改后的 Demo 失败: %w", err)
	}
	verifiedTargets := make(map[string]bool)
	foundReplacement := false
	for _, player := range result.Players {
		if (prefix != "" && targets[player.SteamID64]) || (prefix == "" && player.SteamID64 == replacement.SteamID64) {
			if (prefix != "" && strings.HasPrefix(player.Name, prefix)) || (prefix == "" && player.Name == replacement.Name) {
				foundReplacement = true
				verifiedTargets[player.SteamID64] = true
			}
			for _, alias := range player.Aliases {
				if (prefix != "" && strings.HasPrefix(alias, prefix)) || (prefix == "" && alias == replacement.Name) {
					foundReplacement = true
					verifiedTargets[player.SteamID64] = true
				}
			}
		}
	}
	for steamID64 := range targets {
		if !verifiedTargets[steamID64] {
			foundReplacement = false
		}
	}
	if !foundReplacement {
		os.Remove(outputPath)
		return nil, 0, fmt.Errorf("修改后的 Demo 未能解析出新玩家信息")
	}
	return result, updates, nil
}

func validateReplacement(value PlayerReplacement) error {
	if value.SourceSteamID64 == "" || strings.ContainsAny(value.SourceSteamID64, "\\\x00") {
		return fmt.Errorf("请选择有效的原 SteamID64")
	}
	if !utf8.ValidString(value.Name) || strings.TrimSpace(value.Name) != value.Name || value.Name == "" || len([]byte(value.Name)) > 31 || strings.ContainsAny(value.Name, "\\\x00\r\n") {
		return fmt.Errorf("游戏内名称须为 1–31 字节，不能包含反斜杠、换行或首尾空格")
	}
	for _, character := range value.Name {
		if character < 0x20 || character == 0x7f {
			return fmt.Errorf("游戏内名称不能包含控制字符")
		}
	}
	if len(value.SteamID64) != 17 {
		return fmt.Errorf("新 SteamID64 须为 17 位数字")
	}
	for _, digit := range value.SteamID64 {
		if digit < '0' || digit > '9' {
			return fmt.Errorf("新 SteamID64 须为 17 位数字")
		}
	}
	return nil
}

func playerFileEdits(source io.ReaderAt, demo *Demo, value PlayerReplacement, prefix string, targets map[string]bool) ([]fileEdit, int, error) {
	entries := append([]DirectoryEntry(nil), demo.DirectoryEntries...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Offset < entries[j].Offset })
	for i, entry := range entries {
		if i > 0 && uint64(entry.Offset) < uint64(entries[i-1].Offset)+uint64(entries[i-1].Length) {
			return nil, 0, fmt.Errorf("Demo Directory 中的分段重叠，无法安全写回")
		}
		if uint64(demo.DirectoryOffset) >= uint64(entry.Offset) && uint64(demo.DirectoryOffset) < uint64(entry.Offset)+uint64(entry.Length) {
			return nil, 0, fmt.Errorf("Demo Directory 与分段重叠，无法安全写回")
		}
	}
	maxClients := uint8(32)
	if demo.Server != nil && demo.Server.MaxClients > 0 {
		maxClients = demo.Server.MaxClients
	}
	var edits []fileEdit
	updates := 0
	for _, entry := range entries {
		err := forEachNetworkFrame(source, demo.NetworkProtocol, entry, func(frame networkFrame) error {
			newData, count, err := replaceFrameUserinfo(frame.data, maxClients, value, prefix, targets)
			if err != nil {
				return err
			}
			if count == 0 {
				return nil
			}
			if len(newData) > maxNetworkChunkSize {
				return fmt.Errorf("修改后网络块超过大小限制")
			}
			updates += count
			length := make([]byte, 4)
			binary.LittleEndian.PutUint32(length, uint32(len(newData)))
			edits = append(edits,
				fileEdit{start: frame.lengthOffset, end: frame.lengthOffset + 4, data: length},
				fileEdit{start: frame.dataOffset, end: frame.dataOffset + int64(len(frame.data)), data: newData},
			)
			return nil
		})
		if err != nil {
			return nil, 0, fmt.Errorf("扫描分段 %q: %w", entry.Title, err)
		}
	}
	if updates == 0 {
		return nil, 0, nil
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	directory, err := rewriteDirectory(source, demo, edits)
	if err != nil {
		return nil, 0, err
	}
	newDirectoryOffset, err := shiftedPosition(int64(demo.DirectoryOffset), edits)
	if err != nil || newDirectoryOffset > math.MaxUint32 {
		return nil, 0, fmt.Errorf("修改后的 Directory offset 无效")
	}
	headerOffset := make([]byte, 4)
	binary.LittleEndian.PutUint32(headerOffset, uint32(newDirectoryOffset))
	edits = append(edits,
		fileEdit{start: 540, end: 544, data: headerOffset},
		fileEdit{start: int64(demo.DirectoryOffset), end: int64(demo.DirectoryOffset) + int64(len(directory)), data: directory},
	)
	sort.Slice(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	for i := 1; i < len(edits); i++ {
		if edits[i].start < edits[i-1].end {
			return nil, 0, fmt.Errorf("修改位置重叠，无法安全写回")
		}
	}
	return edits, updates, nil
}

func rewriteDirectory(source io.ReaderAt, demo *Demo, edits []fileEdit) ([]byte, error) {
	length := 4 + len(demo.DirectoryEntries)*directoryEntrySize
	directory := make([]byte, length)
	if _, err := source.ReadAt(directory, int64(demo.DirectoryOffset)); err != nil {
		return nil, err
	}
	for i, entry := range demo.DirectoryEntries {
		start, err := shiftedPosition(int64(entry.Offset), edits)
		if err != nil {
			return nil, err
		}
		end, err := shiftedPosition(int64(entry.Offset)+int64(entry.Length), edits)
		if err != nil {
			return nil, err
		}
		if start < 0 || end < start || start > math.MaxUint32 || end-start > math.MaxUint32 {
			return nil, fmt.Errorf("修改后的分段位置超出 Demo 格式范围")
		}
		entryBytes := directory[4+i*directoryEntrySize:]
		binary.LittleEndian.PutUint32(entryBytes[84:88], uint32(start))
		binary.LittleEndian.PutUint32(entryBytes[88:92], uint32(end-start))
	}
	return directory, nil
}

func shiftedPosition(position int64, edits []fileEdit) (int64, error) {
	shifted := position
	for _, edit := range edits {
		if edit.start >= position {
			break
		}
		if edit.end > position {
			return 0, fmt.Errorf("分段边界位于被修改的网络块中")
		}
		shifted += int64(len(edit.data)) - (edit.end - edit.start)
	}
	return shifted, nil
}

func replaceFrameUserinfo(data []byte, maxClients uint8, value PlayerReplacement, prefix string, targets map[string]bool) ([]byte, int, error) {
	var result bytes.Buffer
	last := 0
	count := 0
	for position := 0; position+7 <= len(data); position++ {
		if data[position] != 0x0d || data[position+1] >= maxClients || data[position+6] != '\\' {
			continue
		}
		endRelative := bytes.IndexByte(data[position+6:], 0)
		if endRelative < 0 || endRelative > maxUserInfoLength {
			continue
		}
		start := position + 6
		end := start + endRelative
		newInfo, matched, err := replaceInfoString(data[start:end], value, prefix, targets)
		if err != nil {
			return nil, 0, err
		}
		if !matched {
			continue
		}
		if len(newInfo) > maxUserInfoLength {
			return nil, 0, fmt.Errorf("修改后玩家信息超过长度限制")
		}
		result.Write(data[last:start])
		result.Write(newInfo)
		last = end
		count++
		position = end
	}
	if count == 0 {
		return nil, 0, nil
	}
	result.Write(data[last:])
	return result.Bytes(), count, nil
}

func replaceInfoString(raw []byte, value PlayerReplacement, prefix string, targets map[string]bool) ([]byte, bool, error) {
	type field struct {
		key        string
		start, end int
	}
	var fields []field
	for position := 0; position < len(raw); {
		if raw[position] != '\\' {
			return nil, false, nil
		}
		keyEnd := bytes.IndexByte(raw[position+1:], '\\')
		if keyEnd < 0 {
			return nil, false, nil
		}
		keyEnd += position + 1
		valueStart := keyEnd + 1
		valueEnd := bytes.IndexByte(raw[valueStart:], '\\')
		if valueEnd < 0 {
			valueEnd = len(raw)
		} else {
			valueEnd += valueStart
		}
		fields = append(fields, field{key: string(raw[position+1 : keyEnd]), start: valueStart, end: valueEnd})
		position = valueEnd
	}
	lastSteamID := ""
	hasName := false
	for _, field := range fields {
		if field.key == "*sid" {
			lastSteamID = string(raw[field.start:field.end])
		}
		if field.key == "name" {
			hasName = true
		}
	}
	steamID64 := cleanText(lastSteamID)
	if !hasName || (prefix != "" && !targets[steamID64]) || (prefix == "" && steamID64 != value.SourceSteamID64) {
		return nil, false, nil
	}
	var result bytes.Buffer
	last := 0
	changedName := false
	for _, field := range fields {
		if field.key != "name" && (field.key != "*sid" || prefix != "") {
			continue
		}
		result.Write(raw[last:field.start])
		if field.key == "name" {
			if prefix == "" {
				result.WriteString(value.Name)
			} else {
				originalName := raw[field.start:field.end]
				if !bytes.HasPrefix(originalName, []byte(prefix)) {
					if len(prefix)+len(originalName) > 31 {
						return nil, false, fmt.Errorf("加前缀后的玩家名称超过 31 字节")
					}
					result.WriteString(prefix)
					changedName = true
				}
				result.Write(originalName)
			}
		} else {
			result.WriteString(value.SteamID64)
		}
		last = field.end
	}
	if prefix != "" && !changedName {
		return nil, false, nil
	}
	result.Write(raw[last:])
	return result.Bytes(), true, nil
}

func writeEditedFile(source *os.File, outputPath string, edits []fileEdit) (err error) {
	output, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("创建输出 Demo: %w", err)
	}
	defer func() {
		if closeErr := output.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
		if err != nil {
			os.Remove(outputPath)
		}
	}()
	if _, err = source.Seek(0, io.SeekStart); err != nil {
		return err
	}
	position := int64(0)
	for _, edit := range edits {
		if _, err = io.CopyN(output, source, edit.start-position); err != nil {
			return err
		}
		if _, err = source.Seek(edit.end, io.SeekStart); err != nil {
			return err
		}
		if _, err = output.Write(edit.data); err != nil {
			return err
		}
		position = edit.end
	}
	_, err = io.Copy(output, source)
	return err
}
