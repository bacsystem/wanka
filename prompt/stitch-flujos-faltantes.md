# Prompts Stitch — flujos faltantes

Proyecto: "SaaS ERP Multirubro Perú" (`projects/3747912429457694633`). Pegar **un prompt por pantalla** en ese mismo proyecto para heredar el design system. Todos comienzan con el mismo bloque de contexto.

## Bloque de contexto (va al inicio de cada prompt)

```
Misma app, mismo design system, iconografía, sidebar, topbar y bottom navigation que las 23 pantallas ya existentes de este proyecto (Wanka, tenant "Clínica Dental Sonrisa", rubro Odontología, RUC 20608941235, primario teal #0F766E, Inter, tema claro con variantes claro/oscuro/sistema). Texto de UI en español (Perú), moneda S/, fechas dd/mm/aaaa, datos de ejemplo peruanos realistas. Desktop 1440px; incluir la variante móvil (≤767px) con bottom navigation. Densidad operativa, tablas compactas, estados como pills, sin imágenes decorativas.
```

## 1. Login y recuperación de acceso

```
[contexto]
Pantalla: INICIO DE SESIÓN (fuera del shell, sin sidebar). Layout dividido: izquierda panel de marca oscuro teal con logo "Wanka", claim "Gestión empresarial y facturación electrónica SUNAT" y 3 bullets (Boletas y facturas en segundos · Inventario multisede · Reportes SIRE); derecha tarjeta de acceso con: campo Correo o usuario, Contraseña con mostrar/ocultar, enlace "¿Olvidaste tu contraseña?", botón primario "Ingresar", separador "o continuar con", botón Google Workspace, pie "¿Tu empresa aún no está registrada? Solicitar demo". Debajo de la tarjeta, badge "SUNAT OSE homologado · datos cifrados". Incluir también en la misma pantalla, como segundo estado, el paso de VERIFICACIÓN 2FA: 6 casillas para código de Authenticator/SMS, temporizador 00:45, "Reenviar código", "Usar otro método". Y un tercer estado de RECUPERAR CONTRASEÑA con campo correo y mensaje de éxito "Te enviamos un enlace válido por 15 minutos". Móvil: una sola columna, panel de marca reducido a cabecera.
```

## 2. Selección de empresa y sede

```
[contexto]
Pantalla: SELECCIÓN DE EMPRESA Y SEDE, se muestra tras iniciar sesión (sin sidebar, fondo neutro). Cabecera "Hola, Carlos · elige con qué empresa trabajar". Grilla de tarjetas de empresa (3): "Clínica Dental Sonrisa S.A.C." RUC 20608941235 · Odontología · 2 sedes · rol Administrador; "Veterinaria San Borja E.I.R.L." RUC 20512345678 · Veterinaria · 1 sede · rol Cajero; "Bodega Don Pepe" RUC 10456789012 · Venta de productos · 1 sede · rol Propietario. Cada tarjeta: avatar con iniciales, badge de rubro, estado SUNAT (Conectado / Certificado por vencer en 12 días), último acceso, botón "Entrar". Al seleccionar la primera se despliega debajo el selector de SEDE y CAJA: chips "Sede Miraflores (principal)", "Sede San Isidro"; caja "CAJA-01 · abierta 08:30 · S/ 300.00 fondo" o "Abrir turno". Botón final "Continuar al panel". Enlace "Registrar nueva empresa (RUC)". Móvil: tarjetas apiladas.
```

## 3. Ficha de cliente / paciente

```
[contexto]
Pantalla: DETALLE DE CLIENTE / PACIENTE, ruta Clientes › María Castillo Quispe. Cabecera con avatar, nombre, DNI 72109843, HC-2024-0392, edad 28, teléfono +51 987 654 321 (WhatsApp), correo, segmento "Paciente frecuente · Convenio Rimac EPS", estado "Al día", acciones: Nueva venta (POS), Agendar cita, WhatsApp, Editar, ⋯ (Estado de cuenta, Desactivar). Fila de 4 KPIs: Facturado total S/ 1,450.00 · Deuda S/ 0.00 · Citas 12 (última 25/09/2026) · Ticket promedio S/ 121. Pestañas: Resumen | Comprobantes | Citas | Historia clínica | Cuenta corriente | Documentos. En Resumen: dos columnas: izquierda tabla "Últimos comprobantes" (Tipo, Serie-Número, Fecha, Total, Estado SUNAT); derecha tarjetas "Datos de facturación" (dirección, correo CPE, aseguradora/copago 20%), "Próxima cita" (jueves 18/09 09:30 · Sillón 1 · Dra. Mendoza · botón Reprogramar) y "Alertas clínicas" (alergia a penicilina, hipertensión controlada). Móvil: KPIs en carrusel, pestañas con scroll horizontal, comprobantes como tarjetas.
```

## 4. Detalle y edición de cotización

```
[contexto]
Pantalla: DETALLE DE COTIZACIÓN COT-2026-00128 (Ventas › Cotizaciones). Cabecera: código, cliente "Rimac Seguros y Reaseguros EPS" RUC 20100041953, estado "Aprobada por cliente", emitida 10/09/2026, vence en 3 días, vendedor, acciones: Enviar por correo, WhatsApp, PDF, Duplicar, botón primario "Aprobar y emitir factura F001". Línea de tiempo horizontal: Borrador → Enviada → Vista por cliente → Aprobada → Facturada (paso actual "Aprobada"). Cuerpo dos columnas: izquierda editor de líneas: tabla editable (Código, Descripción, Cant., U.M., V. unit., Desc. %, Afectación IGV, Total) con 4 ítems odontológicos, fila "+ Agregar ítem del catálogo" con buscador, y bloque de totales (Op. gravada, IGV 18%, Descuento global, Total S/ 15,930.00); derecha tarjetas: "Condiciones comerciales" (validez 15 días, forma de pago crédito 30 días, moneda PEN, detracción SPOT 12% aplicable), "Historial" (creada, enviada, abierta por el cliente 3 veces, aprobada por Juan Rojas – Compras), "Notas internas" con textarea. Móvil: líneas como tarjetas con stepper de cantidad, totales fijos al pie.
```

## 5. Nota de crédito y comunicación de baja

```
[contexto]
Pantalla: EMITIR NOTA DE CRÉDITO sobre la factura F001-00000842 (Ventas › Comprobantes › F001-00000842 › Nota de crédito). Layout wizard de 3 pasos en la parte superior: 1 Motivo · 2 Detalle · 3 Confirmación. Paso 1 visible: selector de tipo de nota con tarjetas seleccionables según catálogo 09 SUNAT (01 Anulación de la operación, 02 Anulación por error en el RUC, 04 Descuento global, 06 Devolución total, 07 Devolución por ítem, 09 Disminución en el valor) con descripción corta; campo "Sustento / descripción" obligatorio; panel lateral derecho fijo "Comprobante afectado" con resumen de la factura (cliente, fecha, total S/ 1,280.00, estado Aceptado, CDR). Paso 2 (mostrar como segundo estado): tabla de ítems de la factura con checkbox y cantidad a devolver, recálculo automático de Op. gravada / IGV / Total de la nota. Paso 3: resumen, serie FC01 – correlativo 00000105, checkbox "Enviar PDF al cliente", botón "Emitir y transmitir a SUNAT". Incluir además, como segundo bloque de la misma pantalla, el diálogo "COMUNICACIÓN DE BAJA" para boletas: comprobante, motivo, advertencia "Solo dentro de los 7 días de emisión", botón "Enviar resumen de bajas". Móvil: wizard vertical, panel del comprobante afectado colapsable.
```

## 6. Recepción de traslado y toma de inventario físico

```
[contexto]
Pantalla: RECEPCIÓN DE TRASLADO GRE T001-000428 (Inventario › Almacenes › San Isidro). Cabecera: guía, origen Sede Miraflores → destino Sede San Isidro, transportista, placa, fecha de despacho, estado "En tránsito", acciones: Ver GRE, Reportar incidencia, botón primario "Confirmar ingreso a almacén". Tabla de ítems: SKU, Producto, Lote / venc., Cant. enviada, Cant. recibida (input), Diferencia (calculada, en rojo si ≠ 0), Ubicación destino (select), Estado (OK / Faltante / Dañado). Barra de escaneo superior "Escanear código QR o de barras" con último escaneo. Panel derecho: resumen (14 ítems · 12 conformes · 1 faltante · 1 dañado), firma del custodio con nombre y checkbox "Declaro conformidad", observaciones. Segundo bloque en la misma pantalla: TOMA DE INVENTARIO FÍSICO: selector de almacén y categoría, tabla con Stock sistema, Conteo físico (input), Diferencia, Costo de la diferencia, botón "Aplicar ajuste (código 99 SUNAT)" y resumen "Diferencia valorizada S/ −184.00". Móvil: filas como tarjetas con teclado numérico grande para el conteo.
```

## 7. Historia clínica y odontograma

```
[contexto]
Pantalla: HISTORIA CLÍNICA DEL PACIENTE Juan Carlos Pérez Huamán (Clientes › ficha › Historia clínica). Cabecera compacta del paciente (DNI, edad, HC, alergias en badge rojo, aseguradora). Cuerpo dos columnas: izquierda ODONTOGRAMA interactivo: 32 piezas permanentes en 4 cuadrantes numeradas FDI (18–11, 21–28, 48–41, 31–38), cada pieza con 5 superficies, leyenda de estados con colores (Sano, Caries, Restaurado, Ausente, Endodoncia, Corona, Implante, Por extraer), pieza 3.6 marcada con caries y 1.8/2.8 marcadas "por extraer"; botones "Odontograma inicial / evolutivo", "Guardar", "Imprimir". Derecha: pestañas Evoluciones | Plan de tratamiento | Presupuesto | Consentimientos | Imágenes. En Evoluciones: lista cronológica (fecha, profesional, procedimiento, piezas, notas SOAP, precio S/, estado facturado/pendiente) y editor "Nueva evolución" con campos Motivo, Diagnóstico CIE-10 (buscador), Procedimiento del catálogo, Piezas, Notas, checkbox "Generar cargo para cobro en POS". Plan de tratamiento: tabla de procedimientos planificados con sesión, prioridad, costo y estado. Móvil: odontograma con zoom horizontal, pestañas debajo.
```

## 8. Onboarding de empresa nueva

```
[contexto]
Pantalla: ASISTENTE DE CONFIGURACIÓN INICIAL (primer ingreso de una empresa nueva), sin sidebar, con stepper lateral izquierdo de 6 pasos: 1 Empresa y RUC · 2 Rubro y módulos · 3 Certificado y SUNAT · 4 Sedes y almacenes · 5 Series de comprobantes · 6 Usuarios. Mostrar el paso 2 activo: título "¿A qué se dedica tu negocio?", 4 tarjetas grandes seleccionables con ícono: Restaurante (mesas, comandas, cocina), Veterinaria (mascotas, historia clínica, vacunas), Odontología / salud (citas, odontograma, EPS), Venta de productos (variantes, tallas, códigos de barras); debajo, lista de módulos que se activarán según el rubro elegido con switches (Agenda de citas ON, Historia clínica ON, Mesas y comandas OFF, Inventario ON, Compras ON, Cuentas por cobrar ON). Panel derecho "Resumen de configuración" con lo completado en el paso 1 (razón social, RUC validado en SUNAT, régimen MYPE, domicilio fiscal) y progreso 2 de 6. Botones "Atrás" y "Continuar". Incluir como estados secundarios el paso 3 (subir .pfx, clave SOL, probar conexión, elegir OSE/SUNAT, entorno beta/producción) y el paso 5 (tabla de series F001/B001/FC01/T001 por sede con correlativo inicial). Móvil: stepper horizontal compacto arriba.
```

## 9. Búsqueda global y notificaciones (complementos del shell)

```
[contexto]
Pantalla: DASHBOARD con dos overlays abiertos para diseñar componentes del shell. 1) COMMAND PALETTE (Ctrl+K) centrado: campo "Buscar comprobante, cliente, producto o acción…", resultados agrupados: Acciones rápidas (Nueva venta, Emitir GRE, Registrar abono, Agendar cita), Clientes (2 resultados con DNI/RUC), Comprobantes (F001-00000842 · S/ 1,280.00 · Aceptado), Productos (SKU, stock), Ir a… (Reportes SUNAT, Configuración); navegación con teclado ↑↓ ↵, pie con atajos. 2) PANEL DE NOTIFICACIONES desplegado desde la campana: pestañas Todas | SUNAT | Stock | Cobranzas; ítems con ícono, título, detalle y hora ("Factura F001-00000841 aceptada por SUNAT · hace 5 min", "Nota de crédito FC01-00000104 rechazada · código 2335", "Stock crítico: Resina A2 agotada en Almacén Central", "Vence hoy: F001-000835 · Consorcio Minero · S/ 2,800.00", "Certificado digital vence en 245 días"); acciones "Marcar todo como leído" y "Ver todas". Móvil: command palette a pantalla completa y notificaciones como bottom sheet.
```

## Orden sugerido de generación

1 Login → 2 Selección de empresa → 3 Ficha de cliente → 4 Cotización → 5 Nota de crédito → 8 Onboarding → 6 Recepción/toma física → 7 Historia clínica → 9 Shell.
