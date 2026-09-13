# Prompt UI/UX — Google Stitch

Herramienta: https://stitch.withgoogle.com (proyecto nuevo, plataforma **Web**)

## Prompt 1 — Base del proyecto + Dashboard

```
Aplicación web responsive (desktop, tablet y móvil) para un SaaS de gestión empresarial multirubro en Perú. Los tenants son pequeñas y medianas empresas: restaurantes, veterinarias, clínicas odontológicas y tiendas de venta de productos. El producto cubre clientes, productos/servicios, inventario y stock, almacenes, ventas (POS y órdenes de servicio), facturación electrónica peruana (SUNAT: boleta, factura, nota de crédito, proforma, guía de remisión) y reportes.

Todo el texto de la UI DEBE estar en español (Perú). Moneda S/ (soles). Fechas dd/mm/aaaa. Usa datos de ejemplo peruanos realistas (RUC 20xxxxxxxxx, DNI de 8 dígitos, nombres como "Clínica Dental Sonrisa", "Veterinaria San Borja").

Dirección visual: SaaS operativo limpio, denso pero legible, sin aspecto de landing de marketing. Tema claro. Superficies neutras (blanco y grises fríos), un solo color primario (teal profundo #0F766E) para acciones, colores semánticos únicamente para estados (verde pagado/emitido, ámbar pendiente, rojo rechazado/stock bajo). Tipografía sans-serif (Inter). Grilla de espaciado de 8px. Tarjetas con borde de 1px y radio de 8px, sin sombras pesadas. Las tablas son el objeto principal: filas compactas, cabecera fija, números alineados a la derecha, estados como pills pequeños.

Iconografía: usa EXCLUSIVAMENTE Lucide Icons (lucide.dev), sin mezclar con Material, Heroicons ni emojis. Trazo 1.5px, esquinas redondeadas, tamaño 20px en navegación y 16px dentro de tablas, botones e inputs; color gris-700 por defecto, teal primario en el ítem activo y en acciones primarias. Cada ítem del menú, KPI, acción de tabla y estado lleva su ícono; ícono + etiqueta siempre, nunca ícono solo excepto en el sidebar colapsado (con tooltip). Ítem de navegación activo: mismo ícono de línea (Lucide no tiene variantes rellenas) sobre fondo teal-50 con barra lateral de 3px teal.
Nombres Lucide a usar — Navegación: Inicio=layout-dashboard, Ventas=shopping-cart, Nueva venta=plus-circle, Comprobantes=receipt, Proformas=file-text, Guías de remisión=truck, Clientes=users, Catálogo=package, Productos=box, Servicios=briefcase, Categorías=tags, Inventario=warehouse, Stock=layers, Almacenes=building-2, Movimientos=arrow-left-right, Kardex=clipboard-list, Reportes=bar-chart-3, Configuración=settings. Barra superior: búsqueda=search, notificaciones=bell, empresa=building, usuario=circle-user, cambiar empresa=chevrons-up-down. KPI: ventas=trending-up, comprobantes=receipt, por cobrar=wallet, stock bajo=alert-triangle. Estados SUNAT: Aceptado=check-circle-2, Pendiente=clock, Rechazado=x-circle, Reintentar=refresh-cw. Acciones de fila: ver=eye, PDF=file-down, XML=file-code, correo=mail, WhatsApp=message-circle, nota de crédito=file-minus, anular=ban, editar=pencil, eliminar=trash-2, más=more-horizontal. Pagos: efectivo=banknote, tarjeta=credit-card, Yape/Plin/transferencia=smartphone. Inventario: ingreso=arrow-down-to-line, salida=arrow-up-from-line, transferencia=arrow-right-left, ajuste=sliders-horizontal. Genéricos: filtrar=filter, exportar=download, ordenar=arrow-up-down, calendario=calendar, cerrar=x, colapsar sidebar=panel-left-close, command palette=command.

Componentes (deben verse ricos e interactivos, no estáticos): tabla de datos con ordenamiento por columna, selector de columnas, paginación y fila de resumen; tarjetas KPI con sparkline y variación en color semántico; skeleton loaders en tarjetas y tablas; estados vacíos con ilustración lineal, mensaje y CTA; toasts de confirmación (arriba a la derecha); command palette (Ctrl+K) para búsqueda global; drawers/side sheets para crear y editar sin salir del listado; modales solo para confirmaciones destructivas; inputs con etiqueta flotante, ayuda contextual y validación en línea; selectores con búsqueda (combobox) para cliente, producto y almacén; date range picker con atajos (Hoy, Ayer, Esta semana, Este mes); segmented controls para alternar vistas; badges de estado con punto de color + texto; menús contextuales (⋯) en cada fila; tooltips en todo ícono y valor truncado. Estados hover, focus visible (anillo teal), activo y disabled en todos los controles.

Responsive (obligatorio en cada pantalla, muestra las tres variantes): 
- Desktop ≥1280px: sidebar expandido, layout multi-columna como se describe.
- Tablet 768–1279px: sidebar colapsado a íconos, columnas se apilan a 2 o 1, tablas con scroll horizontal y primera columna fija.
- Móvil ≤767px: sidebar se convierte en bottom navigation (5 ítems: Inicio, Ventas, Clientes, Inventario, Más) con botón flotante "Nueva venta"; barra superior compacta con búsqueda expandible; las tablas se convierten en listas de tarjetas (título, subtítulo, monto, pill de estado); los KPI se muestran en carrusel horizontal de 2 por vista; drawers pasan a hojas de fondo (bottom sheets) a pantalla completa; áreas táctiles mínimas de 44px.

Layout: sidebar izquierdo fijo (240px, colapsable a íconos) con navegación agrupada: Inicio · Ventas (Nueva venta, Comprobantes, Proformas, Guías de remisión) · Clientes · Catálogo (Productos, Servicios, Categorías) · Inventario (Stock, Almacenes, Movimientos, Kardex) · Reportes · Configuración. Barra superior: selector de empresa mostrando nombre del negocio + rubro, búsqueda global, campana de notificaciones, menú de usuario.

Pantalla a generar ahora: DASHBOARD ("Inicio") en desktop, tablet y móvil.
- Fila de 4 tarjetas KPI: Ventas de hoy (S/), Comprobantes emitidos hoy, Por cobrar (S/), Productos con stock bajo — cada una con variación vs. ayer.
- Izquierda 2/3: gráfico de barras "Ventas últimos 30 días" y debajo tabla "Últimos comprobantes" (Tipo, Serie-Número, Cliente, Total, Estado SUNAT: Aceptado / Pendiente / Rechazado, Fecha).
- Derecha 1/3: tarjeta "Alertas de stock" (producto, almacén, stock, mínimo) y tarjeta "Estado SUNAT" mostrando envíos pendientes/fallidos con acción de reintentar.
- Botón primario arriba a la derecha: "Nueva venta".

El diseño DEBE funcionar para los cuatro rubros: no incluyas imágenes ni íconos específicos de una industria; el rubro se muestra solo como una etiqueta de texto junto al nombre del negocio.
```

## Prompts de seguimiento (mismo proyecto, uno por pantalla)

Pegar uno a la vez para mantener el design system consistente. Cada uno hereda iconografía Lucide, componentes y reglas responsive del Prompt 1; genera siempre desktop, tablet y móvil.

```
Misma app, mismo estilo, mismos componentes e íconos Lucide del Prompt 1; genera desktop, tablet y móvil. Pantalla: NUEVA VENTA (POS). Dos columnas: izquierda = buscador de productos/servicios con chips de categoría y una grilla de tarjetas de ítems (nombre, precio S/, stock); derecha = panel de carrito con líneas (stepper de cantidad, precio unitario, descuento, subtotal), selector de cliente (búsqueda por DNI/RUC con "Cliente genérico" por defecto), selector de tipo de documento (Boleta / Factura / Proforma), método de pago (Efectivo, Tarjeta, Yape, Plin, Transferencia), totales con desglose de IGV 18% (Op. gravada, IGV, Total) y un botón primario grande "Emitir comprobante".
```

```
Misma app, mismo estilo, mismos componentes e íconos Lucide del Prompt 1; genera desktop, tablet y móvil. Pantalla: COMPROBANTES (listado). Filtros: rango de fechas, tipo (Boleta, Factura, Nota de crédito), estado SUNAT, cliente, serie. Tabla: Tipo, Serie-Número, Fecha, Cliente (nombre + RUC/DNI), Moneda, Total, Estado SUNAT (pill), Acciones (Ver PDF, Enviar por correo, Emitir nota de crédito, Reenviar a SUNAT). Barra de acciones masivas al seleccionar filas. Estado vacío y una franja resumen arriba (total emitido, aceptados, rechazados).
```

```
Misma app, mismo estilo, mismos componentes e íconos Lucide del Prompt 1; genera desktop, tablet y móvil. Pantalla: DETALLE DE COMPROBANTE. Cabecera con tipo + Serie-Número, pill de estado SUNAT e información del CDR (hash, fecha de aceptación). Cuerpo en dos columnas: izquierda = vista previa del documento con aspecto de factura impresa (emisor, cliente, ítems, totales, QR, importe en letras); derecha = línea de tiempo (creado, enviado a SUNAT, aceptado / rechazado con código y mensaje de error), acciones (Descargar PDF, XML, CDR; Enviar por correo/WhatsApp; Anular; Nota de crédito).
```

```
Misma app, mismo estilo, mismos componentes e íconos Lucide del Prompt 1; genera desktop, tablet y móvil. Pantalla: STOCK POR ALMACÉN. Pestañas o selector de almacén (Almacén principal, Tienda 2, Consultorio). Tabla: Producto, SKU, Categoría, Stock actual, Stock mínimo, Costo promedio, Valorizado S/, Estado (Normal / Bajo / Agotado). Botones: Ingreso, Salida, Transferencia entre almacenes, Ajuste. Panel lateral (side sheet) de ejemplo para "Nueva transferencia" (origen, destino, ítems, cantidades, motivo).
```

```
Misma app, mismo estilo, mismos componentes e íconos Lucide del Prompt 1; genera desktop, tablet y móvil. Pantalla: CLIENTES. Tabla con Tipo doc (DNI/RUC/CE), Número, Nombre / Razón social, Teléfono, Correo, Total comprado, Última compra, Estado. Drawer "Nuevo cliente" con búsqueda automática por RUC/DNI (botón "Consultar"), dirección, contacto, y un bloque opcional específico por rubro mostrado como una sección genérica "Datos adicionales" (por ejemplo, mascotas para veterinarias, historia clínica para odontologías) renderizado como pestañas dentro del detalle del cliente.
```

```
Misma app, mismo estilo, mismos componentes e íconos Lucide del Prompt 1; genera desktop, tablet y móvil. Pantalla: REPORTES. Lista izquierda de tipos de reporte (Ventas por período, Ventas por producto/servicio, Ventas por vendedor, Cuentas por cobrar, Kardex, Registro de ventas SUNAT, Inventario valorizado). Área principal: barra de filtros (fechas, almacén, vendedor, tipo de comprobante), un gráfico resumen y una tabla de datos, botones de exportación (Excel, PDF).
```

## Estado en Stitch

- Proyecto vigente (del usuario): `projects/3747912429457694633` — "SaaS ERP Multirubro Perú" (Text to UI Pro), 40 pantallas desktop exportadas a `design/stitch/screens/` (ver `index.json`). Para re-sincronizar: `list_screens` → descargar `htmlCode.downloadUrl` / `screenshot.downloadUrl` de cada pantalla y comparar hash contra el archivo local.
- Proyecto inicial `projects/3911888296951345087` (creado desde este repo el 2026-09-12) fue eliminado; su dashboard quedó en `design/stitch/dashboard-desktop.{html,png}` y el design system en `design/stitch/design-system.json` / `DESIGN.md`.
- Conexión: servidor MCP `stitch` (HTTP, `https://stitch.googleapis.com/mcp`, header `X-Goog-Api-Key`) registrado con `claude mcp add`; herramientas: `list_projects`, `list_screens`, `get_screen`, `generate_screen_from_text`, `edit_screens`, `create_design_system`…
