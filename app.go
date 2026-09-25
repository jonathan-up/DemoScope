package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cs_demo_parser/internal/goldsrc"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the desktop boundary between the Vue interface and the GoldSrc parser.
type App struct {
	ctx context.Context
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// OpenDemo presents a native file picker and parses the selected CS 1.6 demo.
// A nil result means the picker was cancelled.
func (a *App) OpenDemo() (*goldsrc.Demo, error) {
	defaultDirectory := ""
	if absolute, err := filepath.Abs("demos"); err == nil {
		if info, statErr := os.Stat(absolute); statErr == nil && info.IsDir() {
			defaultDirectory = absolute
		}
	}

	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "打开 Counter-Strike 1.6 Demo",
		DefaultDirectory: defaultDirectory,
		Filters: []runtime.FileFilter{
			{DisplayName: "GoldSrc Demo (*.dem)", Pattern: "*.dem"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("打开文件选择器: %w", err)
	}
	if path == "" {
		return nil, nil
	}
	return a.ParseDemo(path)
}

// ParseDemo parses a path supplied by the desktop UI.
func (a *App) ParseDemo(path string) (*goldsrc.Demo, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("请选择一个 Demo 文件")
	}
	if !strings.EqualFold(filepath.Ext(path), ".dem") {
		return nil, fmt.Errorf("不支持的文件类型：只接受 .dem 文件")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("解析文件路径: %w", err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return nil, fmt.Errorf("读取 Demo 文件: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("请选择 .dem 文件，而不是目录")
	}

	demo, err := goldsrc.ParseFile(absolute)
	if err != nil {
		return nil, err
	}
	return demo, nil
}

// SavePlayerReplacement writes a new demo with all userinfo updates for the
// selected SteamID64 changed to the requested game name and SteamID64.
func (a *App) SavePlayerReplacement(path, sourceSteamID64, name, steamID64 string) (*goldsrc.Demo, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("请先打开 Demo")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	base := strings.TrimSuffix(filepath.Base(absolute), filepath.Ext(absolute))
	outputPath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:            "保存修改后的 Counter-Strike 1.6 Demo",
		DefaultDirectory: filepath.Dir(absolute),
		DefaultFilename:  base + "_edited.dem",
		Filters: []runtime.FileFilter{
			{DisplayName: "GoldSrc Demo (*.dem)", Pattern: "*.dem"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("打开保存对话框: %w", err)
	}
	if outputPath == "" {
		return nil, nil
	}
	result, _, err := goldsrc.RewritePlayerFile(absolute, outputPath, goldsrc.PlayerReplacement{
		SourceSteamID64: sourceSteamID64,
		Name:            name,
		SteamID64:       steamID64,
	})
	return result, err
}

// SavePrefixedPlayers adds a name prefix to selected players in a new demo file.
func (a *App) SavePrefixedPlayers(path string, sourceSteamID64s []string, prefix string) (*goldsrc.Demo, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("请先打开 Demo")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	base := strings.TrimSuffix(filepath.Base(absolute), filepath.Ext(absolute))
	outputPath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:            "保存加前缀后的 Counter-Strike 1.6 Demo",
		DefaultDirectory: filepath.Dir(absolute),
		DefaultFilename:  base + "_prefixed.dem",
		Filters: []runtime.FileFilter{
			{DisplayName: "GoldSrc Demo (*.dem)", Pattern: "*.dem"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("打开保存对话框: %w", err)
	}
	if outputPath == "" {
		return nil, nil
	}
	result, _, err := goldsrc.RewritePlayersPrefixFile(absolute, outputPath, sourceSteamID64s, prefix)
	return result, err
}
