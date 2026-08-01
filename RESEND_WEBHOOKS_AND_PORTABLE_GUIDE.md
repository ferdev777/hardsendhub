# Guía Integral de Arquitectura: Resend Webhooks (Svix), Cloudflare Tunnel & Servidor Portable Windows

Esta documentación detalla las bondades de integración con **Resend API v2**, el modelo criptográfico de **Webhooks seguros con Svix**, la configuración de red con **Hostinger + Cloudflare Tunnel gratuito**, y la arquitectura del **Servidor Portable en Windows** corriendo en segundo plano sin consumo visual ni de recursos excesivos.

---

## 1. Bondades y Funcionalidades de Resend (v2) en Hardsend

Resend es una plataforma de correo transaccional enfocada en la entregabilidad, velocidad y trazabilidad de eventos. Hardsend aprovecha al máximo sus funcionalidades:

### a) Trazabilidad Unívoca por Etiqueta (`invoice_id`)
* Cada factura enviada adjunta un **tag personalizado** (`invoice_id = UUID`) y recibe de Resend un `resend_id` irrepetible.
* Esto permite conectar de forma directa los eventos asincrónicos del servidor con la factura exacta en SQLite (`hardsend_metrics.db`).

### b) Eventos Asincrónicos Soportados vía Webhooks
El endpoint `/api/webhooks/resend` de nuestro backend procesa los siguientes eventos en tiempo real:
* **`email.delivered`**: Confirma que el servidor de correo del cliente recibió el mensaje.
* **`email.opened`**: Registra la apertura del correo por el usuario final.
* **`email.bounced` (Rebote Hard/Soft)**: Marca el fallo en la base de datos y **añade automáticamente el correo a la Blacklist (`blacklist`)** para proteger la reputación del dominio en envíos futuros.
* **`email.complained` (Denuncia por Spam)**: Inmediatamente suprime la dirección receptora en la tabla `blacklist`.

### c) Seguridad Criptográfica con Firma Svix (HMAC-SHA256)
* Los webhooks **no confían ciegamente** en los paquetes recibidos desde Internet.
* Resend utiliza la infraestructura de **Svix** para firmar criptográficamente cada petición con la clave secreta `SVIX_SECRET` (`whsec_...`).
* Nuestro backend verifica nativamente el `HMAC-SHA256(svix-id + "." + svix-timestamp + "." + body)` sin depender de paquetes externos, bloqueando cualquier intento de falsificación (spoofing) o ataque de repetición (replay attack).

---

## 2. Configuración en Hostinger & Cloudflare Tunnel (Opción A: Máxima Seguridad y Sin Abrir Puertos)

Para recibir los eventos de Resend sin exponer la interfaz gráfica ni la API administrativa (`/api/login`, `/api/upload`, etc.) al Internet público, utilizamos un **Cloudflare Tunnel** gratuito.

### a) Configuración DNS en Hostinger
1. En el panel de **Hostinger**, apuntar los servidores de nombre (NS) a Cloudflare o delegar el subdominio elegido (por ejemplo: `webhooks.tuservidor.com`).
2. Configurar los registros **DKIM**, **SPF** y **DMARC** otorgados en el panel de **Resend** para verificar la autenticidad del dominio de envío (`@facturas...`).

### b) Cloudflare Tunnel (`cloudflared` - Gratuito)
1. El software de Cloudflare Tunnel corre como un servicio en la misma máquina Windows.
2. Crea una conexión saliente segura hacia Cloudflare (sin necesidad de abrir el puerto 80 ni 443 en el router de la empresa ni en el Firewall de Windows).
3. En el dashboard de Cloudflare Zero Trust, configurar la regla de enrutamiento del túnel:
   * **Subdominio Público**: `https://webhooks.tuservidor.com/api/webhooks/resend`
   * **Servicio Local (Destino)**: `http://localhost:8080/api/webhooks/resend`
4. **Beneficio clave**: Cualquier petición a la raíz u otros endpoints desde el túnel es denegada o acotada únicamente a `/api/webhooks/resend`. El Dashboard (`localhost:8080`) sigue siendo 100% interno para la LAN o acceso remoto autorizado.

---

## 3. Servidor Portable en Windows (Segundo Plano & Inicio Automático)

El servidor Windows con 32 GB de RAM donde reside la aplicación muchas veces está operando otros sistemas. Para evitar molestar visualmente al usuario o correr riesgo de cierre accidental:

### a) Modo Silencioso (`-H=windowsgui`)
* Al ejecutar el script de compilación `build_windows_portable.sh`, Go compila el ejecutable `hardsend-server.exe` con la bandera de enlazador `-ldflags "-s -w -H=windowsgui"`.
* Esto suprime por completo la ventana negra de consola (MS-DOS / cmd.exe). El servidor opera invisible en el administrador de tareas consumiendo mínimos recursos.

### b) Persistencia e Inicio Automático con Windows
* Dentro de la carpeta portable se incluye el archivo `instalar_autoarranque.bat`.
* Al ejecutarlo, registra el ejecutable en el Registro de Windows:
  `HKCU\Software\Microsoft\Windows\CurrentVersion\Run -> "HardsendServer"`
* Si el servidor se reinicia (por actualización, corte de luz o mantenimiento), **Hardsend arranca automáticamente de forma silenciosa** apenas inicia el sistema operativo.

### c) Acceso al Dashboard Visual
* Para administrar campañas, subir archivos ZIP/RAR o inspeccionar analíticas en tiempo real (con gráficos temporales por día, semana, mes y año en React/Recharts), cualquier operador simplemente abre el acceso directo **`Hardsend_Dashboard.url`** o entra en su navegador a `http://localhost:8080`.
* El propio binario Go se encarga de servir la aplicación React estática sin necesidad de Node.js, IIS ni servidores web adicionales.

---

## 4. Resumen de Flujo de Resiliencia y Auto-Recuperación

1. **Persistencia ante Caídas**: Las facturas se insertan en SQLite con estado `QUEUED`. Solo pasan a `SENT` tras la confirmación real de red de `emailClient.SendInvoiceEmail()`.
2. **Auto-Reanudación**: Si el servidor se reinicia a mitad de un envío masivo de miles de facturas, el método `ResumeActiveCampaign()` detecta automáticamente los ítems pendientes en estado `QUEUED` y retoma el despacho ordenadamente.
3. **Scheduler Diario**: Un hilo en segundo plano comprueba todos los días a la hora configurada (ej. `09:00`) si hay campañas pendientes por reintentar o planificar, ejecutando la reanudación autónoma sin intervención humana.
