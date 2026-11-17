# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

SaaS de gestión empresarial multirubro para Perú (restaurantes, veterinarias, odontologías, retail): clientes, catálogo, inventario multialmacén, ventas/POS, facturación electrónica SUNAT (boleta, factura, nota de crédito, proforma, guía de remisión) y reportes. Código e identificadores en inglés; textos de UI en español (Perú), moneda `S/`, fechas `dd/mm/aaaa`.

pnpm workspace monorepo. Only app so far: `apps/web` (Next.js 16 App Router, React 19, TypeScript, Tailwind v4, shadcn/ui on Base UI, lucide-react, Recharts).

## Commands

Run from the repo root (they delegate to `apps/web`) or inside `apps/web`:

```bash
pnpm install
pnpm dev                 # next dev (http://localhost:3000)
pnpm build               # production build
pnpm lint                # eslint .
pnpm typecheck           # next typegen && tsc --noEmit  (typegen generates PageProps/LayoutProps globals)
pnpm test                # vitest run (unit, jsdom)
pnpm test:e2e            # playwright (desktop 1440 / tablet 1024 / mobile Pixel 7); starts its own dev server on :3100
```

Single tests: `pnpm --filter web vitest run src/lib/format.test.ts` · `pnpm --filter web exec playwright test --project=mobile -g "sidebar"`. First E2E run needs `pnpm --filter web exec playwright install chromium`.

Note: `next dev` re-writes `apps/web/AGENTS.md`; that file points to the Next 16 docs in `node_modules/next/dist/docs/` — check them before assuming Next API from memory.

## Two interfaces

- **App de la empresa (tenant)** — everything under `app/(app)` and `app/(auth)`; teal theme "Andes Teal v2" (tokens in `app/globals.css` `:root`/`.dark`).
- **Consola de administración del SaaS (super-admin)** — `app/(admin)/admin/*` (dashboard, empresas/[id], planes, facturacion, monitoreo, usuarios, soporte, modulos, auditoria, configuracion) and `app/(admin-auth)/admin/login`. Own shell `features/platform-admin/components/admin-shell.tsx` (dark sidebar, indigo "Platform Indigo" theme via the `.theme-admin` class that overrides the CSS tokens), own navigation `config/admin-navigation.ts`, mocks in `features/platform-admin/mocks/`. No tenant cookie/context. Designs live in `design/stitch/admin/` (separate Stitch project `18424961025192484487`); the tenant design system v2 is `design/stitch/screens/t01-sistema-diseno-v2.html`.

## UI conventions (from the Stitch v2 kit)

- All controls are 40 px tall by default (`Button` default/`icon`, `Input`, `SelectTrigger`, `Toggle`); use `size="sm"`/`xs` only inside dense rows — for inputs in table rows that is `Input size="sm"` (32 px). Don't add `h-7`/`h-8`/`h-9`/`h-10` overrides. Quantities/prices typed by the user go through `components/shared/number-input.tsx` (`NumberInput value onValueChange max`): it keeps the raw text so decimals and a decimal comma can be typed, commits `parseNumber(text)` on each change and normalizes on blur — never bind a controlled `<Input>` straight to a number.
- `DataTable` ships column controls by default: "Columnas" menu (hide/show, locked columns marked "Fijo"), "Restablecer", density toggle. Mark columns `hidden`/`locked`/`label` when needed; keys `select`/`sel`/`actions` are locked automatically; pass `controls={false}` to opt out.
- Dynamic Lucide icons: `components/shared/dynamic-icon.tsx` (`StateIcon` ok/warning/error/pending/syncing/offline, `ConnectionIcon`, `LockIcon`, `ButtonSpinner`). Prefer them over static icons for statuses.
- One component per pattern (audit: `docs/audit-ui-2026-09-13.md`): list of records → `DataTable`; matrices, document/edit lines and printed previews → `ui/table` `Table` (base size 13 px, no `text-*` override); filters above a table → `FilterBar` (`stack` for two rows); titled section → `SectionCard`; detail header (cliente, mascota, comprobante, OC, proveedor, tenant) → `EntityHeader`; list header → `PageHeader`; tabs on a card → `TabsList variant="card"`, filter pills inside a bar → `variant="pills"`, tabs that are routes → `SectionNav`; search → `SearchInput`; status → `StatusBadge`, label/counter chip → `Tag`, CPE type → `DocumentTypeBadge`, initials → `InitialsAvatar`; pagination → `PaginationBar`. `Badge variant="outline"` is only a neutral label. `size="sm"` buttons are allowed only in table rows, `SectionCard action`, bulk-selection bars, `p-1` segmented groups and "volver" ghost links.

## Architecture (`apps/web/src`)

Feature-based layout: **`app/` holds thin route files; everything else lives in `features/<feature>/`** (`components/`, `mocks/`, `lib/`). Cross-cutting pieces stay in `components/` (shell, shared, ui), `config/`, `lib/`, `types/`.

- Route groups: `app/(app)/` is the shell (sidebar, topbar, bottom nav); `app/(auth)/` renders without the shell (`/login`, `/seleccionar-empresa`, `/onboarding`). There is no real auth: `(auth)` pages are mock flows; the selected tenant is stored in the `tenant_id` cookie (`features/auth/lib/tenant-cookie.ts`) and read by `app/(app)/layout.tsx` to pick the tenant from `features/auth/mocks/tenants.ts`. The topbar `TenantSwitcher` writes that cookie and calls `router.refresh()`.
- `app/layout.tsx` — root: fonts (Geist → `--font-geist`, Geist Mono → `--font-geist-mono`; Geist is the only sans, Geist Mono only for codes/amounts via `font-mono`), `ThemeProvider` (next-themes, class strategy, light/dark/system), `TooltipProvider`, sonner `Toaster`, `lang="es-PE"`.
- `app/(app)/layout.tsx` — reads the `sidebar_state` cookie and wraps pages in `components/layout/app-shell.tsx` (shadcn `SidebarProvider` + `AppSidebar` + `Topbar` + `BottomNav`). `app/(app)/[...slug]/page.tsx` renders "Módulo en construcción" for any nav entry without a screen (none at the moment; keep it for future modules). `next.config.ts` redirects group routes (`/ventas`, `/inventario`, `/finanzas`, `/configuracion`) to the landing module declared in `config/redirects.ts`; `navigation.test.ts` checks each landing is a child of its group.
- `config/navigation.ts` — single source of truth for navigation (label, href, Lucide icon, children). `getNavigation(industry)` / `getBottomNavHrefs(industry)` add the industry modules (odontología: Agenda; veterinaria: Mascotas + Agenda; restaurante: Salón, Comandas/cocina, Delivery). Sidebar and bottom nav read the tenant industry from `components/layout/tenant-context.tsx` (`useTenant`). `app/(app)/[...slug]/page.tsx` shows a placeholder only for hrefs present in some industry's navigation; anything else is a real 404 (`app/(app)/not-found.tsx`). Add a nav entry here when adding a screen.
- `components/layout/command-palette.tsx` (Ctrl/Cmd+K global search over mocks, opened from `topbar-search.tsx`) and `notifications.tsx` (bell popover). Base UI menus: a `DropdownMenuLabel` MUST be inside `DropdownMenuGroup` or the page crashes at open time.
- `components/layout/*` — shell only. `sidebar-nav.tsx`, `bottom-nav.tsx`, `theme-toggle.tsx` are client components; `sidebar-auto-collapse.tsx` collapses the sidebar to icons on 768–1279px when no cookie is set.
- `components/shared/states.tsx` — reusable EmptyState / ErrorState / NotFoundState / ForbiddenState / SessionExpiredState / LoadingState; `app/(app)/{loading,error,not-found}.tsx` use them and `DataTable` renders EmptyState when there are no rows. `components/shared/field.tsx` auto-wires `<Label htmlFor>` to its single child control (so tests can use `getByLabel`).
- `components/shared/*` — building blocks every screen uses: `PageHeader`, `StatCard`/`StatGrid` (KPI row, horizontal scroller on mobile), `StatusBadge` (tone: success/warning/danger/info/neutral), `DataTable` (desktop table + `mobileCard` renderer for ≤767px; pass `minWidth` so wide tables scroll inside the card), `SectionCard`, `EntityHeader`, `FilterBar`, `SectionNav`, `Toolbar`/`SearchInput`, `Field`, `PaginationBar`, `InitialsAvatar`, `Tag`.
- `components/ui/*` — shadcn generated (Base UI variant): use the `render` prop instead of `asChild`, and add `nativeButton={false}` when a `Button` renders a `Link`. `Select` needs `items={[...]}` so `SelectValue` shows labels.
- `features/*` — one folder per screen/domain: `dashboard`, `pos` (cart logic in `lib/pos.ts`: IGV-inclusive prices, `cartTotals`), `documents` (comprobantes list + detail, `SunatStatusBadge`, `documentTypeLabel`), `quotes` (board + `/ventas/proformas/[id]` editable detail), `documents` also holds the credit-note wizard (`/ventas/comprobantes/[id]/nota-credito`) and the void ("comunicación de baja") dialog, `dispatch-guides`, `customers` (list, `/clientes/[id]` file), `clinical` (`/clientes/[id]/historia`: interactive FDI odontogram, SOAP evolution), `appointments` (its "Cobrar en POS" links to `/ventas/nueva?cliente=<id>&items=<catalogId,…>`, which the POS reads to preselect customer/lines), `catalog` (tabbed route `/catalogo/[[...tab]]`), `inventory` (stock, movements, warehouses, kardex 13.1, `/inventario/recepcion/[guia]` GRE reception, `/inventario/toma-fisica` physical count), `purchases` (RCE, suppliers, orders — tabs navigate between `/compras`, `/compras/proveedores`, `/compras/ordenes`), `receivables`, `cash`, `reports` (RVIE + `/reportes/liquidacion` PDT 621 & consolidated kardex), `settings` (SUNAT, users/roles, invite drawer, `/perfil`), `restaurant` (`/salon` floor map + table panel, `/salon/cocina` KDS, `/salon/delivery`, `/salon/reservas`), `veterinary` (`/mascotas`, `/mascotas/[id]` vaccine card + SOAP, `/hospitalizacion` kennels + vitals, `/grooming`, `NewPetSheet`), `retail` (`/catalogo/variantes/[sku]` size×color matrix; the POS scan field matches sku/id/EAN `77512345…`), `payables` (`/finanzas/cuentas-por-pagar`, `/configuracion/integraciones`), `cash` also has `/finanzas/caja/historial` (Z-reports), `settings` also `/configuracion/auditoria`, `reports` also `/reportes/gestion` (management tabs), `quotes` also `/ventas/proformas/nueva` (3-step wizard), `documents` also `/ventas/comprobantes/[id]/nota-debito` (catálogo 10, serie FD01), `auth`. `dashboard/components/industry-dashboards.tsx` swaps the home page per tenant industry (restaurante / veterinaria; default is the dental dashboard). Pages are server components that import mocks and render a client `*Screen` component holding filter/dialog state.
- `types/domain.ts` — shared domain types (Tenant, SalesDocument/Detail, Customer, CatalogItem, CartLine…). Feature-only types live next to their mocks. Mocks mirror the shape the future API will return; there is no backend yet.
- `lib/format.ts` — currency (`S/`), integer, percent, date (`dd/mm/aaaa`) and `SERIE-NNNNNNNN` formatting. Use these instead of ad-hoc formatting.

Interactions are mock-backed: actions show a sonner toast (`toast.success(...)`) instead of calling an API. Keep that convention until the API exists.

Visual language follows the Stitch designs (`design/stitch/screens/*.html`, render them with Playwright at 1440px to compare): KPI `StatCard` with tinted icon box (`tone`), `suffix`, `deltaLabel`, `footer` slot, `emphasizeValue`/`highlight`; `StatCompact` for the icon-left variant; `SectionCard` with `icon`/`iconTone`/`description`; `StatusBadge` bordered pills (`toneClass` exported); `DocumentTypeBadge` uppercase chips; tabs rendered as pill strips (`InventoryTabs`, `PurchasesTabs`) or `TabsList` inside a card; page actions live in `PageHeader actions` (client screens receive the KPI grid via a `kpis` prop and render the header themselves when the actions need state). Pass client-component elements to client screens as data/props, not as pre-built JSX from a server page (it triggers a React key warning).

Never nest a `Button` inside a clickable tile rendered as `<button>` (hydration error, and the Next dev overlay dialog then breaks E2E `getByRole("dialog")` queries) — render the tile as a `div` when it holds actions (see `KennelTile` in `features/veterinary/components/hospital-screen.tsx`). Absolute dropdowns inside `SectionCard` get clipped; render search results inline instead.

### Design tokens

`app/globals.css` defines the palette as CSS vars consumed by shadcn (`--primary: #0f766e` teal, cool-neutral surfaces, `--radius: 0.5rem`) plus `--success`/`--warning` for SUNAT and stock states. Tokens come from the Stitch design system in `design/stitch/DESIGN.md`; the generated reference screen is `design/stitch/dashboard-desktop.html`. Icons: Lucide only, `strokeWidth={1.5}` for nav/decorative, default for status badges.

### Responsive contract

- ≥1280px: sidebar expanded, dashboard grid 2/3 + 1/3.
- 768–1279px: sidebar collapsed to icons with tooltips; tables scroll horizontally (`min-w` on the table).
- ≤767px: sidebar hidden, `BottomNav` (Inicio, Ventas, Clientes, Inventario, Más → Sheet), FAB "Nueva venta", KPIs in a horizontal scroller, tables rendered as card lists (`md:hidden` / `hidden md:block` pairs).

## Docs and design assets

`design/` is git-ignored: the Stitch exports referenced below live only on the local machine (re-export them from the Stitch projects if missing).

- `docs/superpowers/specs/` — approved design specs (one per phase).
- `prompt/ui-ux-stitch.md` — Stitch prompts and project ids; `prompt/stitch-flujos-faltantes.md` — prompts for the second batch. Generating from here: call the Stitch MCP tool `generate_screen_from_text` (projectId `3747912429457694633`); the response's `outputComponents[].design.screens[0]` carries the HTML/PNG download URLs (`list_screens` is capped/stale — do not rely on it to discover new screens; `get_screen` by id works). Beware: the design-system JSON contains `on_error`, so never grep responses for "error" to detect failures. `design/stitch/screens/` — the 48 reference screens exported from the user's Stitch project (`index.json` maps file → screen id/title); `design/stitch/DESIGN.md` — design tokens. The Stitch HTML uses Material Symbols; the app uses Lucide only.
- Mermaid diagrams: write `.mmd` files and validate syntax; do not hand-edit diagrams managed by Mermaid Chart Sync (frontmatter with `id:`). See `.github/instructions/mermaid.instructions.md`.

## Backend architecture (`docs/architecture`)

The backend is not built yet. Its design lives in `docs/architecture/`; `README.md` there is the index. Follow these rules when you design or implement backend work, and read the linked doc before changing anything in its area.

- **Stack**: Go modular monolith (hexagonal, one package per bounded context, facades between modules) on PostgreSQL 16. One image runs as `serve --plane=tenant|platform|webhooks`, `worker` and `migrate`. Libraries — River (jobs/outbox), sqlc, goose, oapi-codegen, shopspring/decimal, pquerna/otp, OpenTelemetry, AWS S3 SDK — are **pending owner approval**: do not add them before that (ADR 0002).
- **Decisions are ADRs** in `docs/architecture/adr/NNNN-kebab-title.md`, numbered in sequence. Each has context, options, decision, consequences (the bad ones too), status and date. Add a new ADR instead of rewriting an accepted one; a reversal is a new ADR that supersedes the old one.
- **Multi-tenancy** (ADR 0001):
  - every tenant table has `tenant_id`, `ENABLE` + `FORCE ROW LEVEL SECURITY` and the policy `tenant_id = app.current_tenant()`, where `app.current_tenant()` is `NULLIF(current_setting('app.tenant_id', true), '')::uuid`;
  - every natural `UNIQUE`/PK starts with `tenant_id`, and FKs are composite with `tenant_id`;
  - all tenant data access goes through `InTenantTx(ctx, Actor{TenantID, UserID, Kind, ImpersonatorID})`, which always sets `app.tenant_id`, `app.user_id` and `app.session_kind`; the tenant comes from the session, never from body, URL or header;
  - no role has `BYPASSRLS`, and application roles own no tables;
  - `SECURITY DEFINER` functions are owned only by `wanka_definer`, have `SET search_path = pg_catalog, pg_temp` and `REVOKE ALL ... FROM PUBLIC`; when they touch RLS tables for a tenant the caller did not set, they set it from their own data and restore the caller's value;
  - the only extra policies allowed are the named, read-only ones listed in ADR 0001.
- **RLS check**: `sh docs/architecture/data/rls-check/run.sh` runs the policies and functions on a disposable PostgreSQL 16 container (needs Docker) with the real roles. Any change to policies, roles, grants or `SECURITY DEFINER` functions must update that SQL and pass with **exit 0**. `MUTATION=naive_session_lookup` shows a failing run.
- **API contract**:
  - `docs/architecture/api/openapi.yaml` is the spec-first source (ADR 0009); `api/conventions.md` gives REST `/v1`, `camelCase`, RFC 9457 errors with `code`, cursor pagination, a required `Idempotency-Key` on creating `POST`s, and money as decimal strings;
  - permissions use the catalog `modulo.recurso:accion` in `api/resources.md` (tenant plane plus a separate `plataforma.*` catalog), and stories and routes must cite catalog codes;
  - the frontend mocks still mirror the future API, and will be replaced by `lib/api/*` functions typed from `openapi.yaml`.
- **Money and numbering** (ADR 0008):
  - money is `numeric` with explicit scale (amounts `(14,2)`, unit prices `(14,4)`, costs `(18,6)`, quantities `(14,4)`), never float;
  - PEN and USD from release 1: every money-bearing record has `currency`, and USD documents carry `exchange_rate numeric(6,3)`, which is Khipu's format;
  - correlatives come from a row counter per series, one series per cash register, assigned in the same transaction as the document, with no network call inside.
- **E-invoicing** (ADR 0003):
  - always through Khipu behind the `InvoicingProvider` port, async with outbox, a single in-order emitter per series, and webhooks plus reconciliation; never call SUNAT directly;
  - the Khipu BETA checks (rounding against `POST /v1/facturas/calcular`, N+1 before N) are acceptance criteria of issue #56.
- **Deployment** (ADR 0007, `deployment/`):
  - Railway services `web`, `api`, `platform-api`, `worker` and `webhooks`; only `web` and `webhooks` have public domains, and `/v1` is reachable only over the private network;
  - config is 12-factor; write secret **names** only, never values, in docs, code or issues.
- **Diagrams**: Mermaid `.mmd` files only, validated before committing (`npx -y @mermaid-js/mermaid-cli -i file.mmd -o /tmp/x.svg`), following `.github/instructions/mermaid.instructions.md`.
