package main

import (
	"embed"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Eliminate GTK3 Wayland frame-callback micro-stutter by defaulting to X11/XWayland backend
	if os.Getenv("GDK_BACKEND") == "" {
		os.Setenv("GDK_BACKEND", "x11")
	}
	// Ensure zero-copy DMA-BUF GPU acceleration is enabled in WebKitGTK
	os.Unsetenv("WEBKIT_DISABLE_DMABUF_RENDERER")

	// Start embedded Hardsend API server in background
	go startBackendServer()

	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "hardsend-desktop",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 5, G: 5, B: 5, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
		Linux: &linux.Options{
			WebviewGpuPolicy: linux.WebviewGpuPolicyAlways,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
