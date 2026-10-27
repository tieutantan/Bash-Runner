package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend
var assets embed.FS

func main() {
	app := newApp()

	err := wails.Run(&options.App{
		Title:     "Bash Runner",
		Width:     880,
		Height:    680,
		MinWidth:  640,
		MinHeight: 480,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 0xf3, G: 0xf6, B: 0xfc, A: 0xff},
		OnStartup:        app.startup,
		Bind:             []interface{}{app},
		Mac: &mac.Options{
			// Transparent title bar; the close and minimize buttons sit over the UI header.
			TitleBar: mac.TitleBarHiddenInset(),
			About: &mac.AboutInfo{
				Title:   "Bash Runner",
				Message: "Run .sh files with one click.",
			},
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
