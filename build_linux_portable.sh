#!/usr/bin/env bash
set -e

# Cargar entorno de Homebrew/Linuxbrew si existe (para tener go y node disponibles)
if [ -f "/home/linuxbrew/.linuxbrew/bin/brew" ]; then
    eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)"
fi

echo "=== Construyendo Hardsend Server Portable para Linux (Local/Servidor) ==="

# 1. Crear carpeta de salida portable
mkdir -p dist/HardsendPortableLinux/static
mkdir -p dist/HardsendPortableLinux/tmp

# 2. Compilar el Backend para Linux x64
echo "[1/4] Compilando Backend (Go -> Linux ELF binario)..."
cd backend
GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o ../dist/HardsendPortableLinux/hardsend-server .
cd ..

# 3. Compilar el Frontend (React/Vite -> HTML/CSS/JS estático)
echo "[2/4] Construyendo Frontend React..."
cd frontend
npm run build
cp -r dist/* ../dist/HardsendPortableLinux/static/
cd ..

# 4. Crear archivo .env de ejemplo en la carpeta portable si no existe
cat << 'EOF' > dist/HardsendPortableLinux/.env.example
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

# 5. Crear script de arranque rápido (run_server.sh)
cat << 'EOF' > dist/HardsendPortableLinux/run_server.sh
#!/usr/bin/env bash
cd "$(dirname "$0")"

# Copiar .env de ejemplo si no existe .env
if [ ! -f ".env" ]; then
    echo "[Info] Creando archivo .env a partir de .env.example..."
    cp .env.example .env
fi

echo "=========================================================="
echo "  Iniciando Hardsend Server (Modo Portable Linux)"
echo "  Abre tu navegador en: http://localhost:8080"
echo "=========================================================="
./hardsend-server
EOF
chmod +x dist/HardsendPortableLinux/run_server.sh
chmod +x dist/HardsendPortableLinux/hardsend-server

echo "[3/4] Permisos y scripts de ejecución en Linux creados."
echo "[4/4] ¡Listo! El portable para Linux está en dist/HardsendPortableLinux/"
echo "-> Para probarlo ahora mismo, ejecuta:"
echo "   cd dist/HardsendPortableLinux && ./run_server.sh"
