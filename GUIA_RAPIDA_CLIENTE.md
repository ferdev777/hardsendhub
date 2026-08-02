# Guía Rápida de Hardsend Desktop (Para el Cliente)

Esta guía paso a paso explica cómo instalar y operar **Hardsend Desktop** en Windows de manera rápida, segura y sin complicaciones técnicas.

---

## 1. Instalación en 3 Clics

1. Conectá el pendrive en tu PC con Windows y abrí el archivo **`hardsend-desktop-amd64-installer.exe`**.
2. Seguí el asistente ("Siguiente" → "Siguiente" → "Instalar").
   - *Nota: Si tu Windows no tiene Microsoft WebView2 instalado, el propio instalador lo descargará e instalará automáticamente en este paso.*
3. Al finalizar, verás el icono de **Hardsend** en tu Escritorio y en tu Menú de Inicio.

> [!TIP]
> **Arranque Automático:** Hardsend Desktop queda configurado para iniciarse automáticamente con Windows en segundo plano, por lo cual siempre estará listo cuando prendas tu computadora.

---

## 2. Cómo Enviar Facturas (Modo Carpeta Local)

El flujo normal de trabajo se realiza en **4 pasos sencillos**:

```
[ 1. Seleccionar Carpeta PDF ]  ➔  [ 2. Seleccionar TXT ]  ➔  [ 3. Revisar Análisis ]  ➔  [ 4. Iniciar Envío ]
```

1. **Abrir la Aplicación**:
   - Hacé doble clic en el icono de **Hardsend Desktop** en tu Escritorio (o hacé clic en el icono de Hardsend cerca del reloj de Windows y seleccioná *"Abrir Hardsend Desktop"*).
2. **Seleccionar Carpeta y Archivo TXT**:
   - En la pantalla principal (**Modo Carpeta Local**), hacé clic en el botón para **Seleccionar Carpeta con PDFs**.
   - Hacé clic en el botón para **Seleccionar Archivo TXT** (el listado generado por tu sistema administrativo).
3. **Revisar el Análisis del Lote**:
   - El sistema escaneará los archivos al instante y te mostrará un resumen claro:
     - ✅ **Facturas listas para enviar**
     - ⏭️ **Facturas ya enviadas previamente** (se omiten para evitar duplicados)
     - ⚠️ **Facturas tipo X o sin email asignado** (se excluyen automáticamente)
4. **Iniciar Envío**:
   - Seleccioná la **Fecha de Vencimiento** en el calendario.
   - Hacé clic en **Iniciar Envío**. Podrás ver el progreso en tiempo real y el estado de entrega de cada email.

---

## 3. Funcionamiento en Segundo Plano (El Reloj de Windows)

> [!IMPORTANT]
> **¿Qué pasa si cierro la ventana con la cruz (`X`)?**  
> ¡No te preocupes! El programa **no se cierra ni interrumpe los envíos**. Simplemente se oculta en el área de notificación (al lado del reloj de Windows).

| Acción | Resultado |
| :--- | :--- |
| **Hacer clic en la cruz (`X`)** | La ventana desaparece del escritorio y el servicio sigue corriendo en segundo plano en el reloj. |
| **Clic derecho en el icono del reloj → "Abrir Hardsend Desktop"** | Vuelve a mostrarse la interfaz principal inmediatamente. |
| **Clic derecho en el icono del reloj → "Salir completamente"** | Cierra definitivamente el programa y detiene el servidor local. |

---

## 4. Preguntas Frecuentes y Solución de Problemas

### ¿Qué pasa si reinicio o apago la computadora?
Al encender la PC nuevamente, Hardsend iniciará en modo silencioso al lado del reloj. No necesitas hacer nada para que vuelva a estar disponible.

### ¿Qué sucede si el sistema de facturación agrega ceros a la izquierda?
Hardsend cuenta con un algoritmo de **normalización inteligente de IDs** (por ejemplo, interpreta indistintamente `00123` y `123`), garantizando que tus facturas siempre se vinculen correctamente al email del cliente sin importar cómo el sistema contable formatee los números.

### ¿Puedo corregir un envío fallido manualmente?
Sí. En la sección **Historial** puedes filtrar por facturas con error o pendientes y reintentar el envío o marcar una resolución manual con un solo clic.

---

## Lista de Verificación Rápida (Checklist)

- [ ] Instalé el archivo `hardsend-desktop-amd64-installer.exe` exitosamente.
- [ ] Veo el icono de Hardsend al lado del reloj de Windows en la esquina inferior derecha.
- [ ] Al seleccionar la carpeta de facturas y el TXT, el sistema analiza las facturas en menos de 2 segundos.
- [ ] Al cerrar la ventana con la X, el icono permanece al lado del reloj.
