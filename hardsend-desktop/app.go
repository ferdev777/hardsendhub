package main

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// GetServerURL returns the base URL of the embedded backend server
func (a *App) GetServerURL() string {
	return "http://localhost:8088"
}

// SelectFolder opens a native OS directory selector dialog and returns the selected folder path
func (a *App) SelectFolder() (string, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Seleccionar carpeta con archivos PDF",
	})
	return dir, err
}

// SelectTxtFile opens a native OS file dialog to select a text file
func (a *App) SelectTxtFile() (string, error) {
	file, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Seleccionar archivo TXT de correos",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Archivos de texto (*.txt)",
				Pattern:     "*.txt",
			},
		},
	})
	return file, err
}
