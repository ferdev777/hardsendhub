# Hardsend - Server Edition

## 🚀 Overview

Hardsend es un sistema enterprise de despacho masivo de facturas electrónicas por email. Procesa archivos ZIP/RAR/PDF con facturas, los cruza contra una base de clientes (TXT), y los envía vía Resend con monitoreo en tiempo real, registro histórico y seguimiento de compromiso (aperturas, rebotes, quejas).

## 🏗️ Arquitectura

```
┌─────────────────────────────────────────────────────────────┐
│                    Frontend (React 18)                       │
│  Login → Dashboard (tiempo real) → Registro (histórico)     │
│  TailwindCSS · Recharts · Lucide Icons · WebSocket          │
└─────────────────────┬───────────────────────────────────────┘
                      │ HTTP + WebSocket
┌─────────────────────┴───────────────────────────────────────┐
│                    Backend (Go 1.21+)                        │
│  Chi Router · JWT Auth · Worker Pool (50 goroutines)        │
│  Circuit Breaker · Rate Limiter · Temp Cleaner              │
└───────┬──────────────────┬──────────────────┬───────────────┘
        │                  │                  │
   ┌────┴────┐      ┌─────┴─────┐     ┌─────┴─────┐
   │ SQLite  │      │  Resend   │     │ Filesystem│
   │  (WAL)  │      │  (email)  │     │  (tmp/)   │
   └─────────┘      └───────────┘     └───────────┘
```

- **Backend**: Go 1.21+ con Chi router, Gorilla WebSockets, SQLite3 (pure Go)
- **Frontend**: React 18 + Vite + TailwindCSS + Recharts + Lucide
- **Email**: Resend con rate limiting (14/s), circuit breaker y engagement tracking (webhooks)
- **Database**: SQLite3 con WAL mode y tracking de aperturas

## 📋 Requisitos

- Go 1.21+
- Node.js 18+
- npm 9+
- Cuenta Resend con API Key
- Dominio verificado en Resend

## 🛠️ Setup de Desarrollo

### 1. Configurar credenciales

```bash
cd backend
cp .env.example .env
# Editar .env con tu RESEND_API_KEY real
```

### 2. Compilar y ejecutar

```bash
# Frontend
cd frontend
npm install
npm run build          # Output → backend/static/

# Backend
cd ../backend
go mod tidy
go build -o hardsend.exe .
.\hardsend.exe
```

### 3. Acceder

Abrir **http://localhost:8080** en el navegador.

## 🔐 Credenciales de Acceso

- **Usuario**: `hardsendvideodigital`
- **Contraseña**: `modeloxvz91`

## 📦 Distribución (Windows)

Solo se necesitan estos archivos para correr en otra PC:

```
📦 hardsend/
├── hardsend.exe          ← Binario Go compilado (todo incluido)
├── static/               ← Frontend React compilado
└── .env                  ← Credenciales Resend y configuración
```

La PC destino **NO necesita** Go, Node.js, ni ninguna dependencia.

### Build para Linux (Ubuntu Server)

```bash
cd frontend && npm run build
cd ../backend && GOOS=linux GOARCH=amd64 go build -o hardsend .
```

Despliegue con systemd usando `hardsend.service` incluido en el repo.

## ⚙️ Variables de Entorno

Ver `backend/.env.example` para referencia completa.

| Variable | Default | Descripción |
|----------|---------|-------------|
| `PORT` | `8080` | Puerto HTTP |
| `RESEND_API_KEY` | — | API Key de Resend |
| `RESEND_FROM` | `facturas@videodigital.com.ar` | Email remitente verificado |
| `RESEND_RATE_LIMIT` | `14` | Emails por segundo |
| `WORKER_COUNT` | `50` | Goroutines de envío simultáneo |
| `MAX_RETRIES` | `3` | Reintentos por factura fallida |
| `RETRY_DELAY` | `60s` | Espera entre reintentos |
| `RESEND_RATE_LIMIT` | `14` | Límite de envíos por segundo |
| `CB_FAILURE_THRESHOLD` | `5` | Umbral del circuit breaker |
| `CB_RECOVERY_TIMEOUT` | `300s` | Tiempo de recuperación del circuit breaker |

## 📄 Formato del Archivo TXT

El archivo de clientes (`CABLE-INTERNET_DET.TXT`) usa este formato:

```
email@ejemplo.com;B0002-00338911
cliente@dominio.com;B0002-00338912
```

- Sin encabezados
- Separado por `;`
- Formato: `email;numero_factura`
- El TXT real contiene ~9,200+ clientes

## 📁 Formato de Nombres de PDF

Los PDFs generados por el sistema de facturación usan este formato:

```
00000149 - Factura  B0002-00338911 - ABRIGO NORMA DIANA.pdf
```

### Extracción automática

| Dato | Fuente | Regex/Método |
|------|--------|-------------|
| **Número de factura** | Nombre del archivo | `[A-Z]\d{4}-\d{8}` |
| **Nombre del cliente** | Nombre del archivo | Regex del formato largo |
| **Email del destinatario** | Archivo TXT | Cruce por número de factura |

### Reglas de negocio

- **Facturas tipo B** (ej: `B0002-00338911`) → Se envían normalmente
- **Facturas tipo A** (ej: `A0002-00010043`) → Se envían normalmente
- **Facturas tipo X** (ej: `X0003-00017479`) → Se ignoran silenciosamente (no se envían)
- **Sin nombre en archivo** → Se usa "Cliente/a" como saludo genérico
- **PDFs generados con Crystal Reports** → Streams comprimidos (FlateDecode)

## 📧 Formato del Email

| Campo | Valor |
|-------|-------|
| **Asunto** | `FACTURA MENSUAL VIDEO DIGITAL S.R.L` |
| **Remitente** | `Video Digital S.R.L <facturas@videodigital.com.ar>` |
| **Cuerpo** | HTML profesional con fondo oscuro, saludo personalizado, fecha de vencimiento (30 días), datos de contacto |
| **Adjunto** | PDF de la factura |
| **Tracking** | Seguimiento de Aperturas, Rebotes y Quejas vía Webhooks |
| **Formato MIME** | `multipart/mixed` con `multipart/alternative` (text/plain + text/html) |

## 📊 Funcionalidades

### Panel de Control (Dashboard)
- **Zona de carga**: Drag & drop de archivos TXT + ZIP/RAR/PDF
- **Métricas en tiempo real**: Facturas procesadas, exitosas, errores, aperturas (via WebSocket)
- **Gráfico en vivo**: Recharts con datos de éxito y compromiso (aperturas)
- **Engagement Row**: Tarjetas específicas para Aperturas, Rebotes y Spam
- **Barra de progreso**: Porcentaje de avance del batch
- **Tabla de errores**: Detalle de facturas con errores de validación o red

### Registro Histórico
- **Filtros**: Hoy, Semana, Mes, Año, Todo
- **Resumen**: Total de envíos, facturas, exitosas, errores, tasa de éxito
- **Tabla de historial**: Cada job con fecha, archivo, totales, y estado

### Archivos Soportados
- ✅ **PDF** (sueltos)
- ✅ **ZIP** (con PDFs adentro)
- ✅ **RAR** (con PDFs adentro)
- ✅ **TXT** (base de datos de clientes)

## 🔄 Manejo de Errores

### ERROR_VALIDATION (Sin reintento)
- Email inválido (sin @, formato incorrecto)
- Factura no encontrada en el TXT
- Nombre de archivo PDF inválido

### ERROR_NETWORK (Reintento x3, 60s entre intentos)
- Timeouts de Resend
- Rate limiting de Resend
- Problemas de conexión

### Circuit Breaker
- Se **abre** después de 5 fallos consecutivos de SES
- **Pausa todos los workers** por 5 minutos
- Previene blacklisting de Resend
- Transiciona a **half-open** para probar recuperación

### Idempotencia
- Una factura con el mismo número no se envía dos veces en el mismo mes
- Se marca como "ya enviada" sin generar error

### Webhooks de Resend
- El sistema expone `/api/webhooks/resend` para recibir eventos.
- Se deben configurar en el dashboard de Resend con los eventos: `email.opened`, `email.bounced`, `email.complained`.

## 🗑️ Limpieza Automática

Los archivos PDF temporales en `tmp/` se limpian automáticamente:
- **Cada 24 horas** se revisa la carpeta
- Se eliminan archivos con **más de 20 días** de antigüedad
- Los directorios vacíos se borran después

## 🧪 Tests

```bash
cd backend
go test ./parser/ ./workers/ -v
```

| Paquete | Tests | Cobertura |
|---------|-------|-----------|
| `parser` | 7 tests | ExtractInvoiceNumber, ValidateEmail, ClientName, TypeX, ParseTXT, ValidateAndBuild |
| `workers` | 4 tests | CircuitBreaker: initial, threshold, reset, halfOpen |
| `webhooks` | — | Tracking de eventos por Tag (invoice_id) |

## ⚠️ Resend Sandbox vs Producción

Si la cuenta está en **Sandbox**:
- Solo envía a emails **verificados manualmente**.
- Para enviar a todos los clientes → [solicitar acceso a producción](https://resend.com/docs/dashboard/emails/sending-limit).

## 🔐 Seguridad

- Autenticación JWT con expiración de 24 horas
- Credenciales Resend en `.env` (excluido de Git via `.gitignore`)
- Archivo `.env.example` con placeholders para referencia
- CORS configurado para desarrollo y producción

## 📂 Estructura del Proyecto

```
hardsend/
├── backend/
│   ├── main.go                    # Entry point, router, server
│   ├── .env                       # Credenciales (NO en Git)
│   ├── .env.example               # Template de credenciales
│   ├── auth/
│   │   └── middleware.go          # JWT middleware
│   ├── config/
│   │   └── config.go             # Carga de configuración + godotenv
│   ├── database/
│   │   └── database.go           # SQLite + queries + historial
│   ├── handlers/
│   │   ├── auth.go               # Login handler
│   │   ├── jobs.go               # Jobs + History API
│   │   └── upload.go             # Upload + ZIP/RAR extraction
│   ├── models/
│   │   └── models.go             # Structs: Job, Invoice, History, etc.
│   ├── parser/
│   │   ├── txt_parser.go         # Parser TXT + extracción de datos
│   │   └── txt_parser_test.go    # Tests unitarios
│   ├── email/
│   │   └── client.go             # Resend client + template HTML + fecha 30d
│   ├── websocket/
│   │   └── hub.go                # WebSocket hub para métricas en vivo
│   ├── handlers/
│   │   ├── webhooks.go           # Handler de webhooks para tracking
│   │   └── ...                   # Otros handlers (upload, auth, jobs)
│   ├── workers/
│   │   ├── pool.go               # Worker pool (goroutines)
│   │   ├── circuit_breaker.go    # Circuit breaker pattern
│   │   ├── circuit_breaker_test.go # Tests del circuit breaker
│   │   └── temp_cleaner.go       # Limpieza automática de tmp/
│   └── cmd/
│       └── generate_test_pdfs/   # Script para generar PDFs de prueba
├── frontend/
│   ├── src/
│   │   ├── App.jsx               # Navegación Dashboard ↔ History
│   │   ├── components/
│   │   │   ├── Dashboard.jsx     # Panel principal con métricas
│   │   │   ├── History.jsx       # Registro histórico con filtros
│   │   │   ├── Dropzone.jsx      # Zona de carga drag & drop
│   │   │   ├── StatsRow.jsx      # Tarjetas de estadísticas
│   │   │   ├── ProgressBar.jsx   # Barra de progreso
│   │   │   └── ErrorDatagrid.jsx # Tabla de errores
│   │   ├── context/
│   │   │   └── AuthContext.jsx   # Context de autenticación
│   │   └── hooks/
│   │       └── useWebSocket.js   # Hook de WebSocket
│   └── vite.config.js
├── test_data/                     # PDFs y TXT de prueba
├── hardsend.service               # Systemd service file
├── .gitignore
└── README.md
```

## 🔮 Next Steps

- [ ] **Embeber static/ en el .exe** — Usar `go:embed` para distribuir un único archivo ejecutable sin carpeta static/
- [ ] **Salir de Resend Sandbox** — Solicitar acceso a producción para enviar a cualquier email
- [ ] **HTTPS** — Configurar con reverse proxy (nginx/caddy)
- [ ] **Multi-usuario** — Panel de administración con múltiples usuarios y roles
- [ ] **Notificaciones** — Alertas por email/webhook cuando un batch falla
- [ ] **Exportar reportes** — Descargar historial como CSV/Excel

---
Este proyecto es de uso interno exclusivo. Este servidor es un envío automático. Por favor no responda este correo.
