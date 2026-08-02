//go:build !windows

package main

func initSystray(app *App) {
	// System tray integration is only enabled on Windows to avoid C dependency on libappindicator in Linux
}
