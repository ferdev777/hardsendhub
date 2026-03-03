# Hardsend — Sistema de Envío Masivo de Facturas

**Video Digital S.R.L** — Plataforma interna para el envío masivo de facturas por email.

## Descripción

Hardsend es un sistema completo (backend + frontend) para procesar y enviar automáticamente facturas en PDF a clientes via email a través de [Resend](https://resend.com). Permite subir un archivo RAR/ZIP con miles de facturas junto con un TXT de base de datos de clientes, procesarlos y enviarlos en segundo plano con reintentos automáticos, rate limiting inteligente, y monitoreo en tiempo real.

## Stack Tecnológico

| Componente | Tecnología |
|------------|-----------|
| **Backend** | Go (chi router, SQLite, WebSocket) |
| **Frontend** | React + Vite + TailwindCSS |
| **Email** | Resend API |
| **Base de datos** | SQLite (archivo único, portable) |
| **Autenticación** | JWT |

## Estructura del Proyecto

```
devrow/
├── backend/
│   ├── main.go              # Entry point, rutas, middleware
│   ├── .env                 # Configuración (API keys, rate limits)
│   ├── hardsend_metrics.db  # Base de datos SQLite
│   ├── config/              # Carga de configuración desde .env
│   ├── database/            # Operaciones SQLite (jobs, invoices, missing_emails)
│   ├── email/               # Cliente Resend con rate limiting
│   ├── handlers/            # Handlers HTTP (upload, webhooks, missing_emails, etc.)
│   ├── models/              # Modelos de datos (Job, Invoice, InvoiceJob, etc.)
│   ├── parser/              # Parser del TXT de clientes, validación de facturas
│   ├── websocket/           # Hub WebSocket para métricas en tiempo real
│   └── workers/             # Worker pool, circuit breaker, limpieza de temp
├── frontend/
│   ├── src/
│   │   ├── App.jsx          # Routing principal (Dashboard, History, MissingEmails)
│   │   ├── context/         # AuthContext (JWT)
│   │   ├── components/
│   │   │   ├── Dashboard.jsx     # Panel principal con métricas en tiempo real
│   │   │   ├── Dropzone.jsx      # Zona de carga de archivos + fecha de vencimiento
│   │   │   ├── History.jsx       # Historial de envíos con filtros
│   │   │   ├── MissingEmails.jsx # Gestión de emails faltantes y rebotados
│   │   │   └── Login.jsx         # Pantalla de login
│   │   └── index.css        # Sistema de diseño (dark theme premium)
│   └── dist/                # Build de producción (servido por Go)
└── test-data/               # Datos de prueba
```

## Configuración (.env)

```env
# --- Auth ---
JWT_SECRET=<secreto>
ADMIN_USERNAME=<usuario>
ADMIN_PASSWORD=<contraseña>

# --- Server ---
PORT=8080
FRONTEND_PATH=../frontend/dist

# --- Resend ---
RESEND_API_KEY=re_xxxxx
RESEND_FROM=notificaciones@facturasvideodigital.com
RESEND_RATE_LIMIT=2      # Resend permite máx 2 req/seg en todos los planes

# --- Database ---
DB_PATH=./hardsend_metrics.db

# --- Worker Pool ---
WORKER_COUNT=2            # 2 workers para respetar rate limit de 2/seg
MAX_RETRIES=3
RETRY_DELAY=60s

# --- Circuit Breaker ---
CB_FAILURE_THRESHOLD=5
CB_RECOVERY_TIMEOUT=300s
```

## Funcionamiento

### Flujo de Envío

1. **Subir archivos**: Se sube el TXT (base de datos de clientes) y el RAR/ZIP con PDFs
2. **Fecha de vencimiento**: Se selecciona la fecha que aparecerá en el email (OBLIGATORIO)
3. **Validación**: Cada PDF se valida contra el TXT para encontrar el email del cliente
4. **Envío**: Los workers envían emails via Resend respetando el rate limit
5. **Monitoreo**: El dashboard muestra progreso en tiempo real via WebSocket

### Tipos de Facturas

| Tipo | Prefijo | Comportamiento |
|------|---------|---------------|
| **B** | `B0002-...` | Se envían normalmente por email |
| **A** | `A0002-...` | Se envían normalmente por email (igual que B) |
| **X** | `X0003-...` | Se omiten del envío, contadas como exitosas |

### Sistema de Idempotencia

- Cada factura se verifica si ya fue enviada exitosamente **este mes**
- Si ya fue enviada, se marca como SUCCESS sin reenviar
- Esto permite re-subir el mismo lote sin duplicar emails

### Rate Limiting

- **Resend permite 2 requests/segundo** en todos los planes (Free, Pro, Scale)
- El sistema usa un token bucket rate limiter
- 2 workers compiten por 2 tokens/segundo
- Si se excede, Resend devuelve error 429 y el worker reintenta

### Circuit Breaker

- Si se acumulan 5 fallos consecutivos, el circuit breaker se ABRE
- En estado abierto, no se envían emails por 300 segundos
- Después pasa a HALF-OPEN y prueba con un email
- Si funciona, vuelve a CLOSED

## API Endpoints

### Autenticación
| Método | Ruta | Descripción |
|--------|------|------------|
| POST | `/api/login` | Login, devuelve JWT |

### Upload (protegido)
| Método | Ruta | Descripción |
|--------|------|------------|
| POST | `/api/upload` | Sube archivos + TXT + `due_date` |

### Métricas (protegido)
| Método | Ruta | Descripción |
|--------|------|------------|
| GET | `/api/metrics` | Métricas del último job |
| GET | `/api/errors` | Errores del último job |
| GET | `/api/history` | Historial con filtros por fecha |

### Missing Emails (protegido)
| Método | Ruta | Descripción |
|--------|------|------------|
| GET | `/api/missing-emails` | Lista con filtros (period, show_resolved) |
| GET | `/api/missing-emails/export` | Exportar a CSV |
| POST | `/api/missing-emails/resolve` | Resolver (individual, bulk, todos) |

### Webhooks (público)
| Método | Ruta | Descripción |
|--------|------|------------|
| POST | `/webhooks/resend` | Eventos de Resend (opens, bounces, complaints) |

### WebSocket
| Ruta | Descripción |
|------|------------|
| `/ws` | Métricas en tiempo real (1 update/seg) |

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

## Planes de Resend

| Plan | Precio | Emails/mes | Emails/día | Rate limit |
|------|--------|-----------|------------|------------|
| Free | $0 | 3,000 | 100/día | 2/seg |
| **Pro** | **$20/mes** | **50,000** | Sin límite | 2/seg |
| Scale | $90/mes | 100,000 | Sin límite | 2/seg |

> **Recomendación**: Plan Pro ($20/mes) para ~10,000 facturas mensuales.

## Base de Datos (SQLite)

### Portabilidad
El archivo `hardsend_metrics.db` es **portable**: se puede copiar a otra máquina/servidor y funciona. No requiere instalación de motor de base de datos.

### Tablas
- **jobs**: Lotes de envío (id, filename, total_files, status)
- **invoices**: Facturas individuales (status, email, intentos, engagement)
- **missing_emails**: Emails faltantes/rebotados (reason, email, resolved)

## Compilación y Ejecución

```bash
# Frontend
cd frontend
npm install
npm run build

# Backend
cd backend
go build -o hardsend.exe .
./hardsend.exe
```

El servidor arranca en `http://localhost:8080`.

## Template del Email

Cada email incluye:
- **Asunto**: "FACTURA MENSUAL VIDEO DIGITAL S.R.L"
- **Cuerpo HTML**: Template profesional con tema oscuro
- **Fecha de vencimiento**: La fecha seleccionada manualmente en el frontend
- **PDF adjunto**: La factura correspondiente
- **Aviso de corrección**: Mensaje de disculpas por fecha de vencimiento incorrecta en envíos previos

> ⚠️ El aviso de corrección se puede quitar del template cuando ya no sea necesario (en `email/client.go`).

## Autores

- **Fernando Hirschfeld** — Desarrollo y arquitectura
- **Devrow** — Plataforma

---

© 2026 Fernando Hirschfeld & Devrow. All rights reserved. Closed Source.
