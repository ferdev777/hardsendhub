# Hardsend Desktop (Wails v2.0)

Aplicación de escritorio nativa para Windows y Linux construida con **Wails v2** (Go + React/Vite + WebKitGTK/WebView2). Embebe el servidor Go (`backend/`) sin requerir dependencias externas ni bases de datos separadas.

## Características Principales

- 🖥️ **Selectores Nativos OS**: Diálogos nativos para selección de carpetas de facturas PDF y archivo TXT de clientes.
- 🕒 **System Tray (Reloj de Windows)**: Icono en el área de notificación de Windows con menú para abrir/cerrar y funcionamiento silencioso en segundo plano (`--tray`).
- 🚀 **Arranque Automático en Windows**: Registro automático en `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` mediante instalador NSIS.
- 🛡️ **Normalización Inteligente (`NormalizeInvoiceNumber`)**: Resistente a ceros adicionales en nombres de archivo PDF o TXT.

---

## Compilación del Instalador (.exe) en Windows

Para generar el setup oficial de instalación (`hardsend-desktop-amd64-installer.exe`) con NSIS:

```bash
# En consola de Windows con Wails instalado:
wails build -platform windows/amd64 -nsis
```

El instalador quedará en `build/bin/hardsend-desktop-amd64-installer.exe`.

---

## Desarrollo Local (Live Reload)

```bash
wails dev
```
Este comando inicia el servidor backend embebido (puerto 8088 por defecto) y el frontend de Vite con Hot Module Replacement (HMR).
