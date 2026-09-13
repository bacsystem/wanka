# Frontend: monorepo + Dashboard (fase 1)

Fecha: 2026-09-12. Estado: aprobado.

## Objetivo

Arrancar el frontend Next.js del SaaS multirubro (Perú) con el layout de aplicación y el Dashboard ("Inicio") completo y responsive, usando datos mock, tomando como referencia el diseño generado en Stitch (`design/stitch/`) y el prompt `prompt/ui-ux-stitch.md`.

## Decisiones

- Monorepo con **pnpm workspaces** (sin Turborepo hasta que exista una segunda app). App en `apps/web`.
- **Next.js 16 (App Router) + TypeScript + Tailwind v4 + shadcn/ui + lucide-react + Recharts**.
- Código e identificadores en inglés; textos de UI en español (Perú). Sin i18n.
- Datos solo mock (`src/mocks/dashboard.ts`), con la forma que luego tendrá la API. Sin auth ni multi-tenant real.
- Tokens tomados de `design/stitch/DESIGN.md`: primario `#0F766E`, superficies neutras, radio 4–8px, Inter + JetBrains Mono para cifras.

## Estructura de `apps/web/src`

- `app/layout.tsx` (fuentes, globals), `app/(app)/layout.tsx` (AppShell), `app/(app)/page.tsx` (Dashboard).
- `components/ui/*` shadcn; `components/layout/*` (app-shell, sidebar, topbar, tenant-switcher, bottom-nav); `components/dashboard/*` (kpi-card, sales-chart, recent-documents-table, stock-alerts, sunat-status, sunat-status-badge).
- `config/navigation.ts` única fuente de verdad de la navegación (label, href, ícono Lucide, hijos).
- `lib/format.ts` (moneda S/, fechas dd/mm/aaaa, serie-número), `lib/utils.ts`.
- `types/domain.ts` (Tenant, SalesDocument, SunatStatus, StockAlert, Kpi…).

## Responsive

- ≥1280px: sidebar 240px expandido; grid 2/3 + 1/3.
- 768–1279px: sidebar colapsado a íconos con tooltip; grid 1 columna; tabla con scroll horizontal.
- ≤767px: sidebar oculto; BottomNav (Inicio, Ventas, Clientes, Inventario, Más→Sheet); FAB "Nueva venta"; KPIs en scroll horizontal; comprobantes como lista de tarjetas.

## Testing

- Vitest + Testing Library: `lib/format.ts`, `SunatStatusBadge`, `KpiCard`.
- Playwright smoke: `/` muestra sidebar a 1440px y bottom nav a 390px.
- `pnpm lint`, `pnpm typecheck`, `pnpm build` en verde.

## Fuera de alcance

POS, Comprobantes, Inventario, Clientes, Reportes; API/BD; auth; dark mode; i18n.

## Addendum (2026-09-12, misma sesión) — fase 2: todas las pantallas

Ampliación aprobada verbalmente ("exporta todo e impleméntalo", "hazlo como features"):

- Estructura por features (`src/features/<feature>/{components,lib,mocks}`); rutas delgadas en `app/`.
- Tema claro/oscuro/sistema (next-themes) porque los diseños del proyecto Stitch del usuario lo incluyen.
- Pantallas implementadas con mocks a partir de `design/stitch/screens/`: POS, Comprobantes (lista y detalle), Cotizaciones/proformas, Guías de remisión, Clientes, Agenda, Stock por almacén, Compras RCE/SIRE, Cuentas por cobrar, Caja y arqueo, Reportes SUNAT, Configuración SUNAT/certificado, Usuarios y roles.
- Sin pantalla aún (placeholder): Catálogo (productos, servicios, categorías), Almacenes, Movimientos, Kardex, Proveedores.
- Verificación: Vitest (21), Playwright 62 tests en desktop/tablet/móvil (render de cada ruta sin errores de consola ni overflow horizontal, POS end-to-end).

## Addendum 2 (2026-09-12) — pantallas nuevas de Stitch

Sincronizadas 3 pantallas nuevas y 1 modificada del proyecto Stitch del usuario e implementadas: Catálogo de productos y servicios (`/catalogo` con pestañas), Movimientos de almacén (`/inventario/movimientos`), Almacenes y sedes (`/inventario/almacenes`), y Compras con pestañas Registro / Proveedores / Órdenes (`/compras/*`). Solo Kardex sigue como placeholder. QA visual y funcional: 128 tests E2E.

Kardex físico y valorizado SUNAT 13.1 (`/inventario/kardex`) implementado desde el nuevo diseño de Stitch; ya no quedan placeholders.

## Addendum 3 (2026-09-12) — flujos faltantes

Nueve pantallas diseñadas en Stitch a partir de `prompt/stitch-flujos-faltantes.md` e implementadas: login/2FA/recuperación, selección de empresa y sede (cookie `tenant_id`, cambio de empresa funcional desde el topbar), onboarding de 6 pasos, ficha de cliente, historia clínica con odontograma FDI interactivo, detalle de cotización con líneas editables, wizard de nota de crédito + comunicación de baja, recepción de traslado GRE y toma de inventario físico, command palette (Ctrl+K) y notificaciones. Agenda y ficha de cliente pasan contexto al POS por query params. E2E: 189 tests.

## Addendum 4 (2026-09-12) — barrido E2E + segunda tanda Stitch

Barrido de huecos (acciones que solo mostraban un aviso, módulos prometidos por el onboarding). Se enviaron 8 prompts a Stitch vía MCP, se descargaron los diseños y se implementaron: agendar/reprogramar cita + lista de espera; nueva orden de compra + ficha de proveedor; detalle de GRE con seguimiento e incidencia; invitar colaborador + nuevo almacén/sede; restaurante (salón de mesas, panel de mesa, KDS) con navegación por rubro y tenant "Cevichería El Muelle"; veterinaria (pacientes/mascotas, ficha con carné de vacunas y SOAP) con navegación por rubro; liquidación PDT 621 + kardex consolidado; mi perfil y estados del sistema (vacío, carga, error, 404, 403, sesión expirada). Field con label asociado automáticamente. E2E: 240 tests.

## Addendum 5 (2026-09-12) — tercera tanda Stitch (diseños 20–27) implementada

- Nuevas rutas: `/salon/delivery`, `/salon/reservas`, `/hospitalizacion`, `/grooming`, `/catalogo/variantes/[sku]`, `/reportes/gestion`, `/finanzas/cuentas-por-pagar`, `/finanzas/caja/historial`, `/configuracion/auditoria`, `/configuracion/integraciones`, `/ventas/proformas/nueva` (asistente 3 pasos con plantillas y enlace de aprobación), `/ventas/comprobantes/[id]/nota-debito` (catálogo 10 SUNAT, serie FD01).
- Nuevas features: `retail`, `payables`; `dashboard` gana `industry-dashboards.tsx` (Inicio por rubro restaurante/veterinaria). POS: campo de escaneo de código de barras (F2).
- QA: 297 E2E (screens + flows3 + audit-dark) en 3 viewports, 0 desbordes, 0 errores de consola; lint/typecheck/vitest/build limpios. Bug real corregido: `<button>` anidado en los kennels de hospitalización.

## Addendum 6 (2026-09-12) — paridad visual con Stitch

- Shell: sidebar Wanka (marca, tarjeta de empresa, secciones Módulos core / Operaciones, grupos desplegables, tarjeta de usuario), topbar con píldora SUNAT, ⌘K, campana con punto, avatar y "+ Nueva Venta".
- Componentes: StatCard (icon box por tono, suffix, delta, footer, highlight), StatCompact, SectionCard con icono/subtítulo, badges píldora con borde, chips de tipo de comprobante, cabeceras de tabla en mayúsculas, paginación numerada.
- ~30 pantallas re-maquetadas a partir de los renders de Stitch (dashboard, POS, comprobantes/detalle, cotizaciones maestro-detalle, GRE + trazabilidad, clientes/ficha, agenda, catálogo, stock/almacenes/movimientos/kardex, compras/OC, CxC, caja, reportes, configuración, auth, salón/KDS/delivery, ficha de mascota, hospitalización).
- QA: 333 E2E en 3 viewports, lint/typecheck/vitest/build limpios.

## Addendum 7 (2026-09-12) — Consola de administración separada + kit v2

- Nuevo proyecto Stitch (`18424961025192484487`) con 12 pantallas de la consola super-admin (tema Platform Indigo, sidebar oscuro) → implementadas en `app/(admin)/admin/*` con shell propio, navegación propia y login separado (`/admin/login`, SSO + 2FA).
- Kit v2 de la app de empresa (`t01-sistema-diseno-v2`): tema "Andes Teal" (#0D9488 / ámbar #F59E0B / canvas #F7F9FB), controles uniformes de 40 px, tablas con menú de columnas (ocultar/mostrar, fijas, restablecer) y densidad, iconos Lucide dinámicos con estado y micro-animación.
- QA: 374 E2E (incl. `e2e/admin.spec.ts`) en 3 viewports; lint/typecheck/vitest/build limpios.
