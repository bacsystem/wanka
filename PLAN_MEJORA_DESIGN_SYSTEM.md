# Plan de Auditoría y Mejora Rigurosa: Migración Completa al Design System de `Capacitate` (`khipu 0.5`) y Desacoplamiento de Código Hardcodeado en `Wanka`

**Fecha:** 16 de Noviembre, 2026  
**Alcance:** `D:\jose\Wanka\apps\web\src` (272 archivos `.ts`/`.tsx` de producción)  
**Referencia Objetivo:** `D:\jose\Capacitate\frontend\src` (**khipu design system 0.5 — Pastel Compacto**)

---

## 1. Diagnóstico Cuantitativo (Auditoría Estática del Repositorio)

La auditoría automatizada sobre los **272 archivos** de `D:\jose\Wanka\apps\web\src` (excluyendo tests) revela que la primera pasada cubrió únicamente la capa base (`tokens.css`, `theme.css`, `globals.css`, `button.tsx`, `card.tsx`, `sidebar.tsx`, `topbar.tsx`), dejando **brechas estructurales y visuales importantes**:

| Categoría Auditada | Ocurrencias Detectadas | Impacto Visual / Técnico |
| :--- | :---: | :--- |
| **Colores de paleta Tailwind hardcodeados** (`bg-emerald-50`, `bg-amber-50`, `bg-rose-50`, `bg-blue-50`, `bg-indigo-500`, `text-emerald-600`, etc.) | **153** | Rompen la consistencia del sistema de paletas dinámicas (Lima/Indigo/Océano/Coral/Violeta) y el contraste en los 3 tonos oscuros (Medianoche/Grafito/Tintado). |
| **Colores HEX literales** (`#...`) | **24** | Colores estáticos en mocks (`floor.ts`, `guides.ts`, `movements.ts`, `variants.ts`, `documents.ts`) y UI (`monitoring-screen.tsx`, `chart.tsx`). |
| **Alturas fuera de escala** (`h-7`, `h-8`, `h-9`, `h-10`, `h-11`, `h-12`) | **117** | Rompen la alineación horizontal uniforme **`h-control` (34 px)** de Capacitate en barras de filtro, formularios, tablas y botones. |
| **Radios y sombras heredadas** (`rounded-md`, `rounded-lg`, `shadow-sm`, `shadow-xs`) | **357** | Impiden el aspecto "pastel compacto" (`rounded-xl border-borde-tarjeta shadow-tarjeta` en tarjetas/paneles y `rounded-full` en cápsulas/botones/filtros). |
| **Datos de negocio y credenciales hardcodeadas en UI** | **25+ archivos** | RUCs (`20608941235`), correos (`carlos.mendoza@clinicasonrisa.pe`, `@perusaas.pe`), IPs (`190.237.44.12`), nombres (`Carlos M. Benavides`, `prod-lima-01`) y arrays de catálogo/horarios embebidos dentro de componentes en lugar de `mocks/` o `TenantContext`. |

---

## 2. Principales Focos de Deuda por Archivo

### A. Colores y Badges Hardcodeados (153 ocurrencias)
* `features/platform-admin/components/admin-dashboard.tsx` (10) y `support-screen.tsx` (9) y `tenant-detail.tsx` (6)
* `features/dashboard/components/sunat-status-card.tsx` (9)
* `components/shared/stat-card.tsx` (8), `status-badge.tsx` (5), `dynamic-icon.tsx` (5)
* `features/restaurant/components/` (`floor-screen.tsx` 6, `delivery-screen.tsx` 5, `kds-board.tsx` 4)
* `features/documents/components/` (`document-type-badge.tsx` 4, `document-detail.tsx` 4)
* `features/dispatch-guides/components/guides-table.tsx` (4)

### B. Alturas Inconsistentes vs `h-control` (117 ocurrencias)
* `features/purchases/components/purchase-order-editor.tsx` (5) y `purchases-screen.tsx` (2)
* `features/auth/components/company-select-screen.tsx` (5), `login-screen.tsx` (4), `brand-panel.tsx` (3)
* `components/shared/data-table.tsx` (4), `components/layout/shell-parts.tsx` (4)
* `features/appointments/components/agenda.tsx` (4), `features/veterinary/components/pet-detail.tsx` (3)
* `components/ui/tabs.tsx` (3), `components/ui/toggle.tsx` (3)

### C. Radios y Sombras Legacy (357 ocurrencias)
* `features/documents/components/document-detail.tsx` (18)
* `features/purchases/components/purchase-order-editor.tsx` (13) y `purchases-screen.tsx` (9)
* `features/appointments/components/agenda.tsx` (13)
* `features/pos/components/cart-panel.tsx` (10)
* `features/quotes/components/quotes-board.tsx` (10) y `new-quote-wizard.tsx` (7)
* `features/auth/components/onboarding-screen.tsx` (9)
* `features/restaurant/components/delivery-screen.tsx` (8)
* `features/catalog/components/catalog-screen.tsx` (7), `features/customers/components/customer-detail.tsx` (7)

### D. Código y Datos Hardcodeados en Componentes (No centralizados)
1. **Identidad de Empresa / Emisor SUNAT embebida:** RUC `20608941235`, correo `@dentalsonrisa.pe`, razones sociales fijas dentro de `document-detail.tsx`, `customer-detail.tsx`, `warehouses-screen.tsx` y `onboarding-screen.tsx` en vez de leerlos desde `useTenant()` / `TenantContext` o un objeto `DEFAULT_ISSUER` en `mocks/`.
2. **Identidad Admin y Entorno fija:** `Carlos M. Benavides`, `prod-lima-01`, `190.237.44.12` y `@perusaas.pe` repetidos en `admin-shell.tsx`, `admin-user-menu.tsx`, `admin-login-screen.tsx`, `platform-config-screen.tsx` y `staff-screen.tsx`.
3. **Credenciales y Usuario Demo acoplados en Auth:** `carlos.mendoza@clinicasonrisa.pe` repetido en `login-screen.tsx`, `company-select-screen.tsx` y `onboarding-screen.tsx`.
4. **Arrays de dominio embebidos en vistas:** `services`, `slots`, `professionals`, `waitlist` en `new-appointment-sheet.tsx`; `hourly` en `industry-dashboards.tsx`; `warehouses` en `kardex-screen.tsx`; `creditNoteTypes` en `credit-note-wizard.tsx`.

---

## 3. Plan de Implementación por Fases (De Mayor a Menor Impacto)

### Fase 1: Primitivos UI y Componentes Compartidos (`components/ui/*` y `components/shared/*`)
**Objetivo:** Garantizar que el 80 % de la interfaz herede automáticamente el diseño de `Capacitate` sin tocar lógica de negocio.
1. **`status-badge.tsx`, `document-type-badge.tsx` y `dynamic-icon.tsx`:** Reemplazar todas las clases `emerald/amber/rose/blue` por tokens semánticos (`bg-success text-success-foreground`, `bg-warning text-warning-foreground`, `bg-destructive-suave text-destructive-texto`, `bg-info text-info-foreground`, `bg-acento-suave text-acento-texto`) y bordes `border-0` (píldoras pastel limpias).
2. **`stat-card.tsx` y `section-card.tsx`:**
   - Unificar `highlight` de `StatCard` a `bg-acento-superficie text-acento-superficie-texto` (el bloque destacado pastel Lima/Lavanda característico de Capacitate).
   - Aplicar `border-border/60` en divisores internos, `text-etiqueta` en labels, `text-metrica` en cifras y `ENTRADA` (`motion-safe:animate-entrada`).
3. **`tabs.tsx`, `toggle.tsx`, `dropdown-menu.tsx`, `dialog.tsx`, `sheet.tsx`, `table.tsx`:**
   - Aplicar el estilo `SEGMENTADO` / `SEGMENTO_BASE` (`rounded-full bg-secondary p-[3px]`) a `TabsList` (variante `pills`).
   - Migrar `rounded-lg` -> `rounded-xl` en contenedores/modales y `h-9`/`h-10` -> `h-control` en controles de tabla y filtros (`data-table.tsx`, `filter-bar.tsx`, `search-input`, `pagination.tsx`).
4. **Selector de Apariencia (`PanelApariencia`):**
   - Exponer `<PanelApariencia />` en `UserMenu`, `AdminUserMenu` y/o `Topbar` para que el usuario pueda cambiar en vivo entre las 5 paletas, 3 tonos oscuros y 5 fuentes de Capacitate.

### Fase 2: Pantallas de Autenticación y Onboarding (`features/auth/*` y `admin-login-screen.tsx`)
**Objetivo:** Eliminar el look heredado (`bg-brand-panel`, `bg-indigo-500`, `rounded-2xl border shadow-sm`) y replicar la estética exacta de `Capacitate`.
1. Refactorizar `brand-panel.tsx`, `login-screen.tsx`, `company-select-screen.tsx`, `onboarding-screen.tsx` y `admin-login-screen.tsx`:
   - Lienzo `bg-background` (`#eceef1`) con tarjetas flotantes `rounded-xl border border-borde-tarjeta bg-card shadow-tarjeta`.
   - Panel izquierdo con tarjeta clara/oscura coherente con `--acento` y barra de acento `rounded-full bg-acento`.
   - Steps y pills `rounded-full`, inputs `h-control`, botones `rounded-full`.

### Fase 3: Barrido de Pantallas de Negocio (`features/*`)
**Objetivo:** Llevar las **153** ocurrencias de colores duros, **117** alturas y **357** radios/sombras a **0** violaciones.
1. **Dashboard y Reportes:** `industry-dashboards.tsx`, `sunat-status-card.tsx`, `sales-chart.tsx`, `reports/*` (usar `--chart-1`, `--chart-base`, `--chart-destacado`, `--chart-trazo-*`).
2. **Comercial y SUNAT:** `pos/*` (`cart-panel.tsx`), `documents/*` (`document-detail.tsx`, `credit-note-wizard.tsx`, `debit-note-form.tsx`), `quotes/*` (`quotes-board.tsx`, `new-quote-wizard.tsx`), `dispatch-guides/*`.
3. **Operaciones y Verticales:** `catalog/*`, `inventory/*`, `purchases/*` (`purchase-order-editor.tsx`), `customers/*`, `appointments/*` (`agenda.tsx`), `restaurant/*` (`floor-screen.tsx`, `kds-board.tsx`, `delivery-screen.tsx`), `veterinary/*` (`pet-detail.tsx`, `hospital-screen.tsx`).
4. **Consola Super-Admin (`features/platform-admin/*`):** `admin-dashboard.tsx`, `support-screen.tsx`, `monitoring-screen.tsx`, `tenant-detail.tsx`, `staff-screen.tsx`, `tenants-table.tsx`.

### Fase 4: Desacoplamiento de Código y Datos Hardcodeados
**Objetivo:** Centralizar la configuración y mocks para facilitar la futura conexión a API real.
1. Crear `src/config/demo-session.ts` (credenciales demo de tenant y super-admin, IP simulada, cluster `prod-lima-01`, datos del usuario autenticado).
2. Mover arrays de negocio embebidos en componentes (`new-appointment-sheet.tsx`, `kardex-screen.tsx`, `industry-dashboards.tsx`, `credit-note-wizard.tsx`) hacia `features/<feature>/mocks/`.
3. Hacer que `document-detail.tsx`, `warehouses-screen.tsx`, `customer-detail.tsx` y `onboarding-screen.tsx` consuman RUC, razón social y correo del `TenantContext` o de `mocks/tenants.ts` en lugar de strings literales.
4. Reemplazar los 24 colores HEX en mocks (`floor.ts`, `guides.ts`, `movements.ts`, `variants.ts`, `documents.ts`) por tokens semánticos (`tone: "ok" | "warn" | "danger" | "info"`).

### Fase 5: Reglas Automatizadas de Diseño (Guardrails en Vitest + Playwright)
**Objetivo:** Evitar regresiones futuras, igual que `alturas.test.ts` en `Capacitate`/`khipu`.
1. Añadir test unitario de auditoría (`src/lib/design-system.test.ts`) que falle si aparecen nuevos colores Tailwind hardcodeados (`bg-emerald-*`, `bg-indigo-*`, etc.) o alturas fuera de escala (`h-10`, `h-9` fuera de tablas densas) en `features/` o `components/`.
2. Ejecutar suite completa de verificación:
   - `pnpm typecheck`
   - `pnpm lint`
   - `pnpm test` (Vitest)
   - `pnpm test:e2e` (Playwright en Desktop 1440px, Tablet 1024px y Mobile 390px).
