# Hardsend — Sistema de Envío Masivo de Facturas (v2.0)

**Video Digital S.R.L** — Plataforma interna y aplicación de escritorio para el envío masivo de facturas por email.

## Descripción & Novedades v2.0

Hardsend es un sistema integral (App de Escritorio Wails + Backend Go + Frontend React) para procesar y enviar automáticamente facturas en PDF a clientes via email a través de [Resend](https://resend.com). Permite operar tanto con subida de archivos ZIP/RAR como con **Carpeta Local** monitoreada en tiempo real.

### Novedades Principales de la Versión 2.0:
- 🖥️ **Hardsend Desktop (Wails v2.0)**: Aplicación nativa de escritorio para Windows/Linux con instalador NSIS profesional, integración en **System Tray** (área de notificación al lado del reloj), ocultamiento al cerrar (`HideWindowOnClose`) y **arranque automático con Windows (`--tray`)** en segundo plano. Ver [Guía Rápida del Cliente](file:///home/fer/Documentos/workspace/hardsendhub/GUIA_RAPIDA_CLIENTE.md).
- 🛡️ **Normalización A Prueba de Fallos (`NormalizeInvoiceNumber`)**: Estrategia arquitectónica de normalización canónica de identificadores de factura (`[Letra] + [4 dígitos POS] + [8 dígitos Secuencia]`), inmune a ceros adicionales en nombres de archivos PDF o bases de clientes TXT.
- 🔄 **Sincronización Manual por Uso con Resend**: Eliminación del polling de fondo y webhooks en favor de consulta bajo demanda (botón *Sincronizar con Resend* en interfaz), cuidando cuotas de API y recursos de red.
- 🎨 **Diseño Glassmorphism Premium**: Interfaz limpia orientada a productividad, con tipografía Inter, alertas visuales claras y barra superior optimizada.

## Stack Tecnológico

| Componente | Tecnología |
|------------|-----------|
| **Escritorio (Desktop)** | Wails v2 (Go + WebKit2GTK / WebView2) |
| **Backend** | Go 1.23 (chi router, SQLite, WebSocket, JWT) |
| **Frontend** | React 18 + Vite 5 + TailwindCSS 3 |
| **Email** | Resend API (v2) |
| **Base de datos** | SQLite (modernc.org/sqlite — pure Go, sin CGO) |
| **Autenticación** | JWT (golang-jwt/v5) |
| **WebSocket** | gorilla/websocket |
| **Archivos** | ZIP nativo + RAR (nwaples/rardecode/v2) |


## Estructura del Proyecto

```
devrow/
├── backend/
│   ├── main.go                 # Entry point, router chi, middleware, servidor HTTP
│   ├── .env                    # Configuración (API keys, rate limits, JWT, etc.)
│   ├── .env.example            # Template de configuración
│   ├── hardsend_metrics.db     # Base de datos SQLite (generada automáticamente)
│   ├── hardsend.service        # Archivo systemd para deploy en Linux
│   ├── auth/
│   │   ├── auth.go             # Generación y validación de JWT tokens
│   │   └── middleware.go       # Middleware de autenticación HTTP
│   ├── config/
│   │   └── config.go           # Carga de configuración desde .env con defaults
│   ├── analyzer/
│   │   ├── analyzer.go         # Motor de análisis con goroutines (v2)
│   │   └── analyzer_test.go    # 9 tests unitarios del analyzer
│   ├── database/
│   │   ├── database.go         # Operaciones SQLite (jobs, invoices, engagement)
│   │   ├── campaign.go         # CRUD de campañas + campaign_invoices (v2)
│   │   ├── blacklist.go        # CRUD de lista negra de emails (v2)
│   │   └── history.go          # Consultas de historial mensual (v2)
│   ├── email/
│   │   └── client.go           # Cliente Resend con rate limiting (token bucket), template HTML
│   ├── handlers/
│   │   ├── auth.go             # Handler de login
│   │   ├── campaign.go         # Endpoints de campaña: analyze, rescan, start, cancel (v2)
│   │   ├── history.go          # Historial mensual + corrección manual de estados (v2)
│   │   ├── jobs.go             # Handlers para jobs, errores, métricas, historial
│   │   ├── missing_emails.go   # CRUD de emails faltantes/rebotados + exportación CSV
│   │   ├── upload.go           # Handler de upload (ZIP/RAR + TXT) con validación
│   │   └── webhooks.go         # Webhook de Resend (opens, bounces, delivered, complaints)
│   ├── models/
│   │   └── models.go           # Modelos de datos (Campaign, Invoice, BlacklistEntry, etc.)
│   ├── parser/
│   │   ├── txt_parser.go       # Parser del TXT de clientes, validación de facturas
│   │   └── txt_parser_test.go  # Tests unitarios del parser
│   ├── websocket/
│   │   └── hub.go              # Hub WebSocket para métricas en tiempo real (broadcast 1/seg)
│   ├── workers/
│   │   ├── pool.go             # Worker pool con retry, circuit breaker, context timeouts
│   │   ├── circuit_breaker.go  # Circuit breaker (CLOSED → OPEN → HALF-OPEN)
│   │   ├── circuit_breaker_test.go  # Tests unitarios del circuit breaker
│   │   └── temp_cleaner.go     # Limpieza automática de archivos temporales
│   ├── cmd/                    # Herramientas CLI auxiliares
│   │   ├── dbcheck/            # Verificación de integridad de la base de datos
│   │   ├── generate_test_pdfs/ # Generador de PDFs de prueba
│   │   ├── test_data/          # Datos de prueba para desarrollo
│   │   └── verify_parser/      # Verificador del parser de TXT
│   └── static/                 # Build de producción del frontend (generado por Vite)
├── hardsend-desktop/           # Aplicación de escritorio portable Wails v2 (v2.0)
│   ├── main.go                 # Entry point Wails Desktop
│   ├── app.go                  # Struct App con selectores nativos de archivos/carpetas
│   ├── server.go               # Servidor embebido Hardsend API (puerto 8088 por defecto)
│   ├── wails.json              # Configuración del proyecto Wails
│   └── frontend/               # Interfaz de usuario Wails (Vite + React)
├── frontend/
│   ├── src/
│   │   ├── App.jsx             # Routing principal (Dashboard, History, MissingEmails, CampaignPlan)
│   │   ├── main.jsx            # Entry point React
│   │   ├── index.css           # Sistema de diseño (dark theme premium)
│   │   ├── context/
│   │   │   └── AuthContext.jsx # Provider de autenticación JWT
│   │   ├── hooks/
│   │   │   └── useWebSocket.js # Hook custom para conexión WebSocket con reconexión
│   │   └── components/
│   │       ├── Dashboard.jsx       # Panel principal con métricas en tiempo real
│   │       ├── Dropzone.jsx        # Zona de carga de archivos (drag & drop) + fecha de vencimiento
│   │       ├── CampaignPlan.jsx    # Modo Carpeta Local — análisis, plan, inicio (v2)
│   │       ├── History.jsx         # Historial de envíos con filtros por período
│   │       ├── MissingEmails.jsx   # Gestión de emails faltantes y rebotados
│   │       ├── Login.jsx           # Pantalla de login con autenticación JWT
│   │       ├── ErrorDatagrid.jsx   # Tabla de errores con detalle por factura
│   │       ├── ProgressBar.jsx     # Barra de progreso animada del envío
│   │       └── StatsRow.jsx        # Fila de estadísticas con tarjetas métricas
│   ├── vite.config.js          # Configuración Vite (proxy API/WS, build → backend/static/)
│   ├── tailwind.config.js      # Tema personalizado TailwindCSS
│   └── package.json            # Dependencias (React, Recharts, Lucide, react-dropzone)
├── test_data/                  # Datos de prueba
└── hardsend.service            # Archivo systemd para deploy
```

## Configuración (.env)

```env
# --- Server ---
PORT=8080

# --- Resend API ---
RESEND_API_KEY=re_your_api_key_here
RESEND_FROM=notificaciones@facturasvideodigital.com
RESEND_RATE_LIMIT=2

# --- Database ---
DB_PATH=./hardsend_metrics.db

# --- Worker Pool ---
WORKER_COUNT=1
MAX_RETRIES=3
RETRY_DELAY=60s

# --- Circuit Breaker ---
CB_FAILURE_THRESHOLD=5
CB_RECOVERY_TIMEOUT=300s

# --- JWT ---
JWT_SECRET=change-this-to-a-secure-random-string
JWT_EXPIRY=24h

# --- Auth ---
ADMIN_USERNAME=hardsendvideodigital
ADMIN_PASSWORD=modeloxvz91

# --- File Storage ---
TEMP_DIR=./tmp
```

> 📄 Copiar `.env.example` a `.env` y llenar con los valores reales.

## Funcionamiento

### Modo 1: Archivos (Upload directo)

1. **Login**: El usuario se autentica con credenciales → recibe JWT token
2. **Subir archivos**: Se sube el TXT (base de datos de clientes) y el RAR/ZIP con PDFs
3. **Fecha de vencimiento**: Se selecciona la fecha que aparecerá en el email (OBLIGATORIO)
4. **Validación**: Cada PDF se valida contra el TXT para encontrar el email del cliente
5. **Envío**: Los workers envían emails via Resend respetando el rate limit con reintentos
6. **Monitoreo**: El dashboard muestra progreso en tiempo real via WebSocket (1 update/seg)
7. **Engagement**: Resend envía webhooks de opens, bounces, delivered y complaints

### Modo 2: Carpeta Local (v2 — Análisis con Persistencia)

1. **Indicar rutas**: El operador ingresa la ruta de la carpeta de PDFs y del TXT
2. **Analizar**: El motor Analyzer escanea la carpeta con goroutines, cruza con el TXT y genera un plan
3. **Revisar plan**: Se muestran facturas categorizadas: Válidas, Sin Email, Blacklisteadas, Omitidas
4. **Re-escanear**: Si se agregan más PDFs a la carpeta, se puede re-escanear sin perder el progreso
5. **Iniciar envío**: Se confirma el límite diario y se inicia. Los PDFs QUEUED se envían al worker pool
6. **Persistencia**: Si se cierra la app, al reabrirla se recupera la campaña activa automáticamente
7. **Cancelar**: En cualquier momento se puede frenar la campaña

### Tipos de Facturas

| Tipo | Prefijo | Comportamiento |
|------|---------|---------------|
| **B** | `B0002-...` | Se envían normalmente por email |
| **A** | `A0002-...` | Se envían normalmente por email (igual que B) |
| **X** | `X0003-...` | Se omiten del envío, contadas como exitosas |

### Normalización A Prueba de Fallos (`NormalizeInvoiceNumber`)

El parser de Hardsend v2.0 implementa una normalización de dominio canónica para todos los identificadores de factura:
- **Estructura Canónica**: `[Letra] + [4 dígitos POS] + [8 dígitos Secuencia]` (ej: `B0002-00338911`).
- **Limpieza Automática**: Si un sistema de facturación externo añade ceros iniciales adicionales en el PDF (`00000149 - Factura  0B00002-000338911...`) o en el archivo TXT de clientes, el sistema los normaliza automáticamente antes de cruzar los datos.
- **Facturas Tipo X**: La detección de facturas internas tipo X (`IsTypeXInvoice`) normaliza previamente la cadena, previniendo envíos accidentales causados por ceros sobrantes.

### Sistema de Idempotencia

- Cada factura se verifica si ya fue enviada exitosamente **este mes**
- Si ya fue enviada, se marca como SUCCESS sin reenviar
- Esto permite re-subir el mismo lote sin duplicar emails

### Rate Limiting

- El sistema usa un **token bucket rate limiter** configurable
- Los workers compiten por tokens (configurable via `RESEND_RATE_LIMIT`)
- Si Resend devuelve error 429, el worker reintenta automáticamente

### Circuit Breaker

| Estado | Descripción |
|--------|------------|
| **CLOSED** | Operación normal, se envían emails |
| **OPEN** | Se acumularon 5+ fallos consecutivos. No envía por 300s |
| **HALF-OPEN** | Prueba con un email. Si funciona → CLOSED, si falla → OPEN |

### Limpieza Automática de Temporales

- El `TempCleaner` elimina archivos PDF procesados mayores a **20 días**
- Se ejecuta automáticamente cada **24 horas** en background
- Remueve archivos y directorios vacíos en dos pasadas

## API Endpoints

### Autenticación
| Método | Ruta | Descripción |
|--------|------|------------|
| POST | `/api/login` | Login con username/password, devuelve JWT |
| GET | `/api/validate` | Verifica si el JWT actual es válido |

### Upload (protegido con JWT)
| Método | Ruta | Descripción |
|--------|------|------------|
| POST | `/api/upload` | Sube archivos ZIP/RAR + TXT + `due_date` |

### Jobs y Métricas (protegido)
| Método | Ruta | Descripción |
|--------|------|------------|
| GET | `/api/jobs` | Últimos 20 jobs recientes |
| GET | `/api/jobs/{jobID}/errors` | Errores de un job específico |
| GET | `/api/jobs/{jobID}/metrics` | Métricas en tiempo real de un job |
| GET | `/api/errors` | Todos los errores de todos los jobs |
| GET | `/api/history` | Historial con filtros: `?period=day\|week\|month\|year\|all` |
| GET | `/api/history/monthly` | Historial mensual: `?year=2026&month=3&status=SUCCESS` |

### Campañas (protegido — v2)
| Método | Ruta | Descripción |
|--------|------|------------|
| POST | `/api/campaigns/analyze` | Escanea carpeta + TXT y crea un plan de envío |
| GET | `/api/campaigns/active` | Recuperar campaña pendiente (persistencia) |
| GET | `/api/campaigns/{id}` | Detalle de campaña + lista de facturas |
| POST | `/api/campaigns/{id}/rescan` | Re-escanear carpeta (solo archivos nuevos) |
| POST | `/api/campaigns/{id}/start` | Iniciar envío de facturas QUEUED |
| POST | `/api/campaigns/{id}/cancel` | Cancelar campaña |

### Corrección Manual (protegido — v2)
| Método | Ruta | Descripción |
|--------|------|------------|
| PATCH | `/api/invoices/{id}/status` | Marcar como MANUAL_SUCCESS (corrección manual) |

### Missing Emails (protegido)
| Método | Ruta | Descripción |
|--------|------|------------|
| GET | `/api/missing-emails` | Lista con filtros (period, show_resolved) |
| GET | `/api/missing-emails/export` | Exportar a CSV |
| POST | `/api/missing-emails/resolve` | Resolver (individual, bulk, todos) |

### Webhooks y Sincronización (v2.0)
| Método | Ruta | Descripción |
|--------|------|------------|
| POST | `/api/webhooks/resend` | Eventos de Resend (opens, bounces, delivered, complaints) |
| POST | `/api/resend/sync` | Sincronización manual de métricas y estados contra la API de Resend bajo demanda |

### WebSocket (autenticado via query param)
| Ruta | Descripción |
|------|------------|
| `/ws/metrics?token=<JWT>` | Métricas en tiempo real (broadcast 1 update/seg) |

## Gestión de Emails Faltantes

La sección "Faltantes" agrupa dos tipos de problemas:

| Razón | Descripción | Badge |
|-------|------------|-------|
| **Sin email** | El número de factura no se encontró en el TXT | 🟡 Ámbar |
| **Rebotó** | El email fue enviado pero rebotó (dirección inválida) | 🔴 Rojo |

### Funcionalidades:
- **Filtrar** por período (día, semana, mes, año, total)
- **Filtrar por razón** (Todos, Sin email, Rebotados)
- **Ordenar** por más nuevos o más viejos
- **Buscar** por número de factura, cliente o email
- **Resolver** individual, masivo o todos
- **Exportar CSV** con columnas: Nro Factura, Cliente, Email, Razón, Fecha, Estado
- **Anti-duplicación**: si una factura ya fue resuelta, no se re-registra

## Frontend — Componentes

| Componente | Descripción |
|-----------|------------|
| **Dashboard** | Panel principal con métricas del job activo, gráfico de progreso, actividad en tiempo real |
| **Dropzone** | Drag & drop de archivos (ZIP/RAR + TXT), selector de fecha de vencimiento |
| **CampaignPlan** | Modo Carpeta Local — escáner de carpeta, plan de envío, re-escaneo, inicio, cancelación (v2) |
| **History** | Historial de envíos con filtros por período, tarjetas de resumen, lista de jobs |
| **MissingEmails** | Gestión completa de emails faltantes/rebotados con filtros, búsqueda y exportación |
| **ErrorDatagrid** | Tabla detallada de errores por factura (número, email, razón, intentos) |
| **ProgressBar** | Barra de progreso animada del envío actual |
| **StatsRow** | Fila de tarjetas con métricas: Opens, Bounces, Complaints, Success Rate |
| **Login** | Autenticación con diseño dark theme premium |

## Planes de Resend

| Plan | Precio | Emails/mes | Emails/día | Rate limit |
|------|--------|-----------|------------|------------|
| Free | $0 | 3,000 | 100/día | 2/seg |
| **Pro** | **$20/mes** | **50,000** | Sin límite | 2/seg |
| Scale | $90/mes | 100,000 | Sin límite | 2/seg |

> **Recomendación**: Plan Pro ($20/mes) para ~10,000 facturas mensuales.

## Base de Datos (SQLite)

### Portabilidad
El archivo `hardsend_metrics.db` es **portable**: se puede copiar a otra máquina/servidor y funciona. No requiere instalación de motor de base de datos (usa SQLite puro en Go, sin dependencia CGO).

### Tablas
| Tabla | Campos principales | Descripción |
|-------|-------------------|------------|
| **jobs** | id, filename, total_files, status, created_at | Lotes de envío |
| **invoices** | id, job_id, invoice_number, recipient_email, status, error_reason, attempts, opened, bounced, complained, delivered | Facturas individuales con estado de engagement |
| **missing_emails** | id, job_id, invoice_number, client_name, email, reason, resolved, resolved_at | Emails faltantes o rebotados |
| **campaigns** | id, folder_path, txt_path, status, total_invoices, valid_count, no_email_count, etc. | Campañas de envío (v2) |
| **campaign_invoices** | id, campaign_id, invoice_number, client_name, email, pdf_path, status, reason | Detalle de facturas por campaña (v2) |
| **blacklist** | id, email, reason, original_invoice, created_at | Lista negra de emails (auto-blacklist por bounce) (v2) |

## Tests

```bash
# Tests del parser de TXT
cd backend
go test ./parser/ -v

# Tests del circuit breaker
go test ./workers/ -v

# Tests del analyzer (escaneo, cruce, idempotencia, blacklist, re-scan)
go test ./analyzer/ -v

# Todos los tests
go test ./... -v
```

### Cobertura de Tests del Analyzer (v2)
| Test | Descripción |
|------|------------|
| TestScanFolder_ValidPDFs | Escanea carpeta con PDFs válidos |
| TestScanFolder_EmptyFolder | Carpeta vacía devuelve resultado vacío |
| TestScanFolder_MixedFiles | Ignora archivos no-PDF |
| TestScanFolder_InvalidPath | Ruta inválida devuelve error |
| TestCrossReference_AllMatched | Todos los PDFs tienen email en el TXT |
| TestCrossReference_SomeMissing | PDFs sin match en el TXT → NO_EMAIL |
| TestCrossReference_Idempotency | Facturas ya enviadas → SKIPPED |
| TestRescan_DetectsNewFiles | Re-escaneo detecta solo archivos nuevos |
| TestAnalyzeFolder_TypeXInvoices | Facturas tipo X → SKIPPED |

## Compilación y Ejecución

### Desarrollo

```bash
# Terminal 1 — Backend
cd backend
go run .

# Terminal 2 — Frontend (con hot reload y proxy al backend)
cd frontend
npm install
npm run dev
```

El frontend de desarrollo corre en `http://localhost:5173` con proxy automático al backend en `:8080`.

### Producción

```bash
# 1. Build del frontend (genera archivos en backend/static/)
cd frontend
npm install
npm run build

# 2. Build del backend
cd backend
go build -o hardsend.exe .

# 3. Ejecutar
./hardsend.exe
```

El servidor arranca en `http://localhost:8080` y sirve el frontend estáticamente con fallback SPA.

### Build de Hardsend Desktop y Setup Windows (.exe)

Para generar el instalador oficial de Windows (con NSIS, icono en System Tray, autoarranque y descargas automáticas de WebView2):

```bash
# En terminal de Windows:
cd hardsend-desktop
wails build -platform windows/amd64 -nsis
```

El instalador final y el ejecutable portable quedarán en **`hardsend-desktop/build/bin/hardsend-desktop-amd64-installer.exe`**.  
Para instrucciones detalladas de uso para el cliente final, consultar la [Guía Rápida del Cliente](file:///home/fer/Documentos/workspace/hardsendhub/GUIA_RAPIDA_CLIENTE.md).

### Deploy con systemd (Linux)

```bash
# Copiar binario y archivos
sudo mkdir -p /opt/hardsend
sudo cp backend/hardsend /opt/hardsend/
sudo cp backend/.env /opt/hardsend/
sudo cp backend/hardsend_metrics.db /opt/hardsend/  # Si existe

# Crear usuario del servicio
sudo useradd -r -s /sbin/nologin hardsend

# Instalar y arrancar el servicio
sudo cp hardsend.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable hardsend
sudo systemctl start hardsend

# Ver logs
sudo journalctl -u hardsend -f
```

## Herramientas CLI

El directorio `backend/cmd/` contiene utilidades para desarrollo:

| Herramienta | Descripción |
|------------|-------------|
| `dbcheck` | Verificación de integridad de la base de datos SQLite |
| `generate_test_pdfs` | Genera PDFs de prueba para testing |
| `verify_parser` | Verifica el correcto funcionamiento del parser de TXT |
| `test_data` | Directorio con datos de prueba listos para usar |

```bash
# Ejemplo: verificar la base de datos
cd backend
go run ./cmd/dbcheck/
```

## Template del Email

Cada email incluye:
- **Asunto**: "FACTURA MENSUAL VIDEO DIGITAL S.R.L"
- **Cuerpo HTML**: Template profesional con tema oscuro
- **Cuerpo texto plano**: Fallback para clientes sin HTML
- **Fecha de vencimiento**: La fecha seleccionada manualmente en el frontend
- **PDF adjunto**: La factura correspondiente

## Dependencias Principales

### Backend (Go)
| Paquete | Uso |
|---------|-----|
| `go-chi/chi/v5` | Router HTTP |
| `go-chi/cors` | CORS middleware |
| `golang-jwt/jwt/v5` | Autenticación JWT |
| `gorilla/websocket` | WebSocket |
| `resend/resend-go/v2` | API de Resend |
| `modernc.org/sqlite` | SQLite puro en Go (sin CGO) |
| `nwaples/rardecode/v2` | Descomprimir archivos RAR |
| `google/uuid` | Generación de UUIDs |
| `joho/godotenv` | Carga de `.env` |

### Frontend (Node.js)
| Paquete | Uso |
|---------|-----|
| `react` / `react-dom` | UI framework |
| `recharts` | Gráficos y charts |
| `react-dropzone` | Drag & drop de archivos |
| `lucide-react` | Iconos SVG |
| `tailwindcss` | Framework CSS |
| `vite` | Build tool y dev server |

## Autores

- **Fernando Hirschfeld** — Desarrollo y arquitectura
- **Devrow** — Plataforma

---

© 2026 Fernando Hirschfeld & Devrow. All rights reserved. Closed Source.
