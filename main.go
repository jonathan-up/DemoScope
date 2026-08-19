package main

import (
	"context"
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()
	err := wails.Run(&options.App{
		Title:                    "DemoScope — CS 1.6 Demo Inspector",
		Width:                    1360,
		Height:                   860,
		MinWidth:                 1040,
		MinHeight:                680,
		Frameless:                false,
		StartHidden:              true,
		WindowStartState:         options.Normal,
		BackgroundColour:         &options.RGBA{R: 8, G: 12, B: 17, A: 1},
		AssetServer:              &assetserver.Options{Assets: assets},
		OnStartup:                app.startup,
		OnDomReady:               func(ctx context.Context) { runtime.WindowShow(ctx) },
		EnableDefaultContextMenu: false,
		Bind:                     []interface{}{app},
	})
	if err != nil {
		println("DemoScope failed to start:", err.Error())
	}
}
