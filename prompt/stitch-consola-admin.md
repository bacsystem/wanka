# Stitch · Consola de Administración del SaaS (proyecto separado) + Tema v2 de la app de empresa

Dos interfaces claramente separadas:

1. **Consola de Administración (super-admin de la plataforma Wanka)** — proyecto Stitch propio, tema distinto.
2. **App de la empresa (tenant)** — proyecto existente (`3747912429457694633`); se le pide un **tema v2** y mejoras de componentes.

## Reglas comunes (van en cada prompt)

- Idioma español (Perú). Moneda `S/`, fechas `dd/mm/aaaa`.
- Iconos **Lucide** exclusivamente, **dinámicos**: cambian según estado (ej. `wifi`/`wifi-off`, `lock`/`lock-open`, `check-circle`/`alert-triangle`), con micro-animación en hover/estado (rotación de `refresh-cw` mientras sincroniza, pulso en alertas).
- **Altura uniforme de controles: 40 px** para input, select, combobox, date picker, botones y chips de filtro; radio de 8 px; misma tipografía (Inter 14 px).
- **Tablas**: cabecera con menú "Columnas" (`columns-3`) para mostrar/ocultar columnas con casillas, botón "Restablecer" para volver al orden/visibilidad por defecto, densidad compacta/cómoda, columnas fijables, ordenamiento por cabecera, paginación numerada con selector de filas.
- Responsive: desktop ≥1280, tablet 768–1279 (sidebar colapsado), móvil ≤767 (tablas → tarjetas, bottom nav).
- Estados vacíos, cargando y error diseñados.

## Prompt 1 — Consola de Administración (nuevo proyecto)

Tema **"Platform Indigo"**: primario índigo `#4F46E5`, acento violeta `#7C3AED`, superficies `#F5F6FA`/blanco, texto `#111827`, éxito `#059669`, alerta `#D97706`, error `#DC2626`; sidebar **oscuro** `#0F172A` con texto claro para diferenciarse a simple vista de la app de empresa (teal, sidebar claro).

Pantallas:
1. **Dashboard de plataforma**: MRR, empresas activas, nuevas altas del mes, churn, comprobantes emitidos por todos los tenants hoy, salud de OSE/SUNAT por proveedor, incidentes abiertos, mapa/lista de tenants por rubro (odontología, veterinaria, restaurante, retail).
2. **Empresas (tenants)**: tabla con razón social, RUC, rubro, plan, estado (activa/suspendida/trial), sedes, usuarios, uso (comprobantes/mes), OSE, último acceso; acciones: ver, impersonar, suspender; drawer "Alta de empresa" (RUC → SUNAT, rubro, plan, sede inicial, admin invitado).
3. **Detalle de empresa**: cabecera con estado y plan, pestañas Resumen · Suscripción y facturación · Sedes y cajas · Usuarios · Certificado y OSE · Consumo · Auditoría · Soporte.
4. **Planes y suscripciones**: catálogo de planes (Básico/Pro/Enterprise) con límites (comprobantes, usuarios, sedes, módulos por rubro), precios PEN/USD, add-ons; suscripciones activas, próximos cobros, morosidad.
5. **Facturación del SaaS**: facturas emitidas a los tenants (nuestra propia facturación electrónica), pagos, conciliación, reintentos de cobro, notas de crédito.
6. **Monitoreo SUNAT / OSE global**: estado por proveedor (SUNAT directo, Bizlinks, Nubefact, Efact), latencia, colas, tasa de rechazo por tenant, certificados por vencer, alertas.
7. **Usuarios de plataforma y roles**: staff interno (soporte, ventas, finanzas, ingeniería), roles, 2FA, sesiones.
8. **Soporte / tickets**: bandeja por prioridad, SLA, tenant afectado, conversación, macros.
9. **Feature flags y módulos por rubro**: activar módulos (agenda, salón, mascotas, variantes) por plan/tenant, lanzamientos graduales.
10. **Auditoría global y seguridad**: eventos críticos multi-tenant, impersonaciones, exportaciones, IPs.
11. **Configuración de plataforma**: dominios, correo transaccional, WhatsApp API, pasarelas de pago, backups.
12. **Login de la consola** (distinto del login de empresa): SSO Google Workspace, 2FA obligatorio, aviso de acceso restringido.

## Prompt 2 — Tema v2 + kit de componentes (proyecto de empresa existente)

Genera una pantalla "Sistema de diseño v2 · App de empresa" y aplica el tema a Dashboard y Comprobantes:
- Tema **"Andes Teal v2"**: primario `#0D9488` con acento `#F59E0B`, fondo `#F7F9FB`, tarjetas blancas con borde `#E5E9F0`, tipografía Inter; modo oscuro equivalente (`#0B1220`).
- Kit: botones (primario, secundario, ghost, destructivo, icono) todos 40 px; input/select/combobox/date/número con prefijo y sufijo, todos 40 px; chips de filtro 40 px; tabs píldora; badges píldora; KPI card; tarjeta de sección; **tabla con menú de columnas (ocultar/mostrar, restablecer), densidad y columnas fijas**; paginación; estados; toasts; iconos Lucide dinámicos con estados.
