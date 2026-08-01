#!/usr/bin/env bash
set -e

echo "=== Construyendo Hardsend Server Portable para Windows ==="

# 1. Crear carpeta de salida portable
mkdir -p dist/HardsendPortable/static
mkdir -p dist/HardsendPortable/tmp

# 2. Compilar el Backend para Windows x64 en modo silencioso (-H=windowsgui evita la consola DOS)
echo "[1/4] Compilando Backend (Go -> Windows EXE en segundo plano sin ventana)..."
cd backend
GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -H=windowsgui" -o ../dist/HardsendPortable/hardsend-server.exe .
cd ..

# 3. Compilar el Frontend (React/Vite -> HTML/CSS/JS estático)
echo "[2/4] Construyendo Frontend React..."
cd frontend
npm run build
cp -r dist/* ../dist/HardsendPortable/static/
cd ..

# 4. Crear archivo .env de ejemplo en la carpeta portable si no existe
cat << 'EOF' > dist/HardsendPortable/.env.example
RESEND_API_KEY=re_xxxxxxxxx
RESEND_FROM=notificaciones@tuservidor.com
RESEND_RATE_LIMIT=10
SVIX_SECRET=whsec_xxxxxxxxxxx
SCHEDULE_TIME=09:00
DB_PATH=./hardsend_metrics.db
TEMP_DIR=./tmp
JWT_SECRET=tu-clave-secreta-jwt-muy-segura
PORT=8080
EOF

# 5. Crear el script de Instalación en Inicio Automático (.bat) para Windows
cat << 'EOF' > dist/HardsendPortable/instalar_autoarranque.bat
@echo off
echo ====================================================================
echo  Instalando Hardsend Server en Inicio Automatico de Windows
echo ====================================================================
set "EXEPATH=%~dp0hardsend-server.exe"

reg add "HKCU\Software\Microsoft\Windows\CurrentVersion\Run" /v "HardsendServer" /t REG_SZ /d "\"%EXEPATH%\"" /f
echo [OK] Hardsend Server configurado para iniciar automaticamente con Windows.
echo Iniciando servicio ahora...
start "" "%EXEPATH%"
echo [OK] Servicio corriendo en segundo plano en http://localhost:8080
pause
EOF

# 6. Crear un script para detener el servidor de segundo plano
cat << 'EOF' > dist/HardsendPortable/detener_servidor.bat
@echo off
echo Deteniendo Hardsend Server...
taskkill /F /IM hardsend-server.exe /T 2>nul
echo [OK] Servidor detenido.
pause
EOF

# 7. Crear Acceso Directo al Dashboard
cat << 'EOF' > dist/HardsendPortable/Hardsend_Dashboard.url
[InternetShortcut]
URL=http://localhost:8080
EOF

echo "[3/4] Permisos y scripts auxiliares creados con éxito."
echo "[4/4] ¡Listo! El portable de Windows se generó en dist/HardsendPortable/"
