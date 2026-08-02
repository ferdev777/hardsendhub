//go:build windows

package main

import (
	"os"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func initSystray(app *App) {
	go systray.Run(func() {
		setupSystray(app)
	}, func() {})
}

func setupSystray(app *App) {
	systray.SetTemplateIcon(trayIcon, trayIcon)
	systray.SetTitle("Hardsend")
	systray.SetTooltip("Hardsend Desktop - Corriendo en segundo plano")

	mOpen := systray.AddMenuItem("Abrir Hardsend Desktop", "Mostrar la interfaz gráfica")
	mStatus := systray.AddMenuItem("Servicio Activo (localhost:8088)", "Servidor local en ejecución")
	mStatus.Disable()
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Salir completamente", "Cerrar servidor y aplicación")

	for {
		select {
		case <-mOpen.ClickedCh:
			if app.ctx != nil {
				runtime.WindowShow(app.ctx)
				runtime.WindowUnminimise(app.ctx)
			}
		case <-mQuit.ClickedCh:
			systray.Quit()
			if app.ctx != nil {
				runtime.Quit(app.ctx)
			}
			os.Exit(0)
		}
	}
}
