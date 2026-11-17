# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

SaaS de gestión empresarial multirubro para Perú (restaurantes, veterinarias, odontologías, retail): clientes, catálogo, inventario multialmacén, ventas/POS, facturación electrónica SUNAT (boleta, factura, nota de crédito, proforma, guía de remisión) y reportes. Código e identificadores en inglés; textos de UI en español (Perú), moneda `S/`, fechas `dd/mm/aaaa`.

pnpm workspace monorepo. Only app so far: `apps/web` (Next.js 16 App Router, React 19, TypeScript, Tailwind v4, shadcn/ui on Base UI, Phosphor icons (lucide-react in legacy screens, see Icons), Recharts). Design system: the pinned package `khipu-design-system` (see Design system).

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

- **App de la empresa (tenant)** — everything under `app/(tenant)/(app)` and `app/(tenant)/(auth)`; the design system's default palette is Lima, and the user picks another in the appearance menu.
- **Consola de administración del SaaS (super-admin)** — `app/(admin)/admin/*` (dashboard, empresas/[id], planes, facturacion, monitoreo, usuarios, soporte, modulos, auditoria, configuracion) and `app/(admin-auth)/admin/login`. Own shell `features/platform-admin/components/admin-shell.tsx`, own navigation `config/admin-navigation.ts`, mocks in `features/platform-admin/mocks/`. No tenant cookie/context. Its look is the design system's `indigo` palette: `app/(admin)/layout.tsx` and `app/(admin-auth)/layout.tsx` are their own root layouts (`AppDocument palette="indigo"`: `<html data-paleta="indigo">` from the server HTML, no flash, nothing written to localStorage; moving between console and tenant app is a full page load) and `AdminPalette` hides the palette choice in the appearance menu. There are no local token overrides (`.theme-admin` is gone). Designs live in `design/stitch/admin/`.

## UI conventions (from the Stitch v2 kit)

- All controls are `h-control` (32 px, `--spacing-control: 2rem` from the design system) in every size (`Button` default/`sm`/`lg`/`icon`, `Input`, `SelectTrigger`); only `xs` / `icon-xs` (24 px) exist, and only inside another control. Don't add `h-7`/`h-8`/`h-9`/`h-10` overrides (`lib/design-system.test.ts` guards `ui/button|input|select`). Quantities/prices typed by the user go through `components/shared/number-input.tsx` (`NumberInput value onValueChange max`): it keeps the raw text so decimals and a decimal comma can be typed, commits `parseNumber(text)` on each change and normalizes on blur — never bind a controlled `<Input>` straight to a number.
- `DataTable` ships column controls by default: "Columnas" menu (hide/show, locked columns marked "Fijo"), "Restablecer", density toggle. Mark columns `hidden`/`locked`/`label` when needed; keys `select`/`sel`/`actions` are locked automatically; pass `controls={false}` to opt out.
- Dynamic status icons: `components/shared/dynamic-icon.tsx` (`StateIcon` ok/warning/error/pending/syncing/offline, `ConnectionIcon`, `LockIcon`, `ButtonSpinner`). Prefer them over static icons for statuses.
- One component per pattern (audit: `docs/audit-ui-2026-09-13.md`): list of records → `DataTable`; matrices, document/edit lines and printed previews → `ui/table` `Table` (base size 13 px, no `text-*` override); filters above a table → `FilterBar` (`stack` for two rows); titled section → `SectionCard`; detail header (cliente, mascota, comprobante, OC, proveedor, tenant) → `EntityHeader`; list header → `PageHeader`; tabs on a card → `TabsList variant="card"`, filter pills inside a bar → `variant="pills"`, tabs that are routes → `SectionNav`; search → `SearchInput`; status → `StatusBadge`, label/counter chip → `Tag`, CPE type → `DocumentTypeBadge`, initials → `InitialsAvatar`; pagination → `PaginationBar`. `Badge variant="outline"` is only a neutral label. `size="sm"` buttons are allowed only in table rows, `SectionCard action`, bulk-selection bars, `p-1` segmented groups and "volver" ghost links.

## Architecture (`apps/web/src`)

Feature-based layout: **`app/` holds thin route files; everything else lives in `features/<feature>/`** (`components/`, `mocks/`, `lib/`). Cross-cutting pieces stay in `components/` (shell, shared, ui), `config/`, `lib/`, `types/`.

- Route groups: `app/(tenant)/(app)/` is the shell (sidebar, topbar, bottom nav); `app/(tenant)/(auth)/` renders without the shell (`/login`, `/seleccionar-empresa`, `/onboarding`). There is no real auth: `(auth)` pages are mock flows; the selected tenant is stored in the `tenant_id` cookie (`features/auth/lib/tenant-cookie.ts`) and read by `app/(tenant)/(app)/layout.tsx` to pick the tenant from `features/auth/mocks/tenants.ts`. The topbar `TenantSwitcher` writes that cookie and calls `router.refresh()`.
- Root layouts (three, none at the top of `app/`): `app/(tenant)/layout.tsx` (tenant app + auth), `app/(admin)/layout.tsx` and `app/(admin-auth)/layout.tsx` (console, `palette="indigo"`). All three render `components/providers/app-document.tsx`: fonts from `app/fonts.ts` (`next/font`: Inter → `--font-sans`, JetBrains Mono → `--font-mono`; Plus Jakarta Sans / Manrope / DM Sans / Figtree → `--fuente-*`, chosen with `data-fuente`), `ScriptDiseno` in `<head>` (applies the saved palette / dark tone / font before the first paint), `ThemeProvider` (`components/providers/theme-provider.tsx`: next-themes class strategy + the design system's `DisenoProvider`, `AparienciaProvider`, `ProveedorMovimiento`), `TooltipProvider`, sonner `Toaster`, `lang="es-PE"`.
- `app/(tenant)/(app)/layout.tsx` — reads the `sidebar_state` cookie and wraps pages in `components/layout/app-shell.tsx` (shadcn `SidebarProvider` + `AppSidebar` + `Topbar` + `BottomNav`). `app/(tenant)/(app)/[...slug]/page.tsx` renders "Módulo en construcción" for any nav entry without a screen (none at the moment; keep it for future modules). `next.config.ts` redirects group routes (`/ventas`, `/inventario`, `/finanzas`, `/configuracion`) to the landing module declared in `config/redirects.ts`; `navigation.test.ts` checks each landing is a child of its group.
- `config/navigation.ts` — single source of truth for navigation (label, href, icon, children). `getNavigation(industry)` / `getBottomNavHrefs(industry)` add the industry modules (odontología: Agenda; veterinaria: Mascotas + Agenda; restaurante: Salón, Comandas/cocina, Delivery). Sidebar and bottom nav read the tenant industry from `components/layout/tenant-context.tsx` (`useTenant`). `app/(tenant)/(app)/[...slug]/page.tsx` shows a placeholder only for hrefs present in some industry's navigation; anything else is a real 404 (`app/(tenant)/(app)/not-found.tsx`). Add a nav entry here when adding a screen.
- `components/layout/command-palette.tsx` (Ctrl/Cmd+K global search over mocks, opened from `topbar-search.tsx`) and `notifications.tsx` (bell popover). Base UI menus: a `DropdownMenuLabel` MUST be inside `DropdownMenuGroup` or the page crashes at open time.
- `components/layout/*` — shell only. `sidebar-nav.tsx`, `bottom-nav.tsx`, `theme-toggle.tsx` are client components; `sidebar-auto-collapse.tsx` collapses the sidebar to icons on 768–1279px when no cookie is set.
- `components/shared/states.tsx` — reusable EmptyState / ErrorState / NotFoundState / ForbiddenState / SessionExpiredState / LoadingState; `app/(tenant)/(app)/{loading,error,not-found}.tsx` use them and `DataTable` renders EmptyState when there are no rows. `components/shared/field.tsx` auto-wires `<Label htmlFor>` to its single child control (so tests can use `getByLabel`).
- `components/shared/*` — building blocks every screen uses: `PageHeader`, `StatCard`/`StatGrid` (KPI row, horizontal scroller on mobile), `StatusBadge` (tone: success/warning/danger/info/neutral), `DataTable` (desktop table + `mobileCard` renderer for ≤767px; pass `minWidth` so wide tables scroll inside the card), `SectionCard`, `EntityHeader`, `FilterBar`, `SectionNav`, `Toolbar`/`SearchInput`, `Field`, `PaginationBar`, `InitialsAvatar`, `Tag`.
- `components/ui/*` — shadcn generated (Base UI variant): use the `render` prop instead of `asChild`, and add `nativeButton={false}` when a `Button` renders a `Link`. `Select` needs `items={[...]}` so `SelectValue` shows labels.
- `features/*` — one folder per screen/domain: `dashboard`, `pos` (cart logic in `lib/pos.ts`: IGV-inclusive prices, `cartTotals`), `documents` (comprobantes list + detail, `SunatStatusBadge`, `documentTypeLabel`), `quotes` (board + `/ventas/proformas/[id]` editable detail), `documents` also holds the credit-note wizard (`/ventas/comprobantes/[id]/nota-credito`) and the void ("comunicación de baja") dialog, `dispatch-guides`, `customers` (list, `/clientes/[id]` file), `clinical` (`/clientes/[id]/historia`: interactive FDI odontogram, SOAP evolution), `appointments` (its "Cobrar en POS" links to `/ventas/nueva?cliente=<id>&items=<catalogId,…>`, which the POS reads to preselect customer/lines), `catalog` (tabbed route `/catalogo/[[...tab]]`), `inventory` (stock, movements, warehouses, kardex 13.1, `/inventario/recepcion/[guia]` GRE reception, `/inventario/toma-fisica` physical count), `purchases` (RCE, suppliers, orders — tabs navigate between `/compras`, `/compras/proveedores`, `/compras/ordenes`), `receivables`, `cash`, `reports` (RVIE + `/reportes/liquidacion` PDT 621 & consolidated kardex), `settings` (SUNAT, users/roles, invite drawer, `/perfil`), `restaurant` (`/salon` floor map + table panel, `/salon/cocina` KDS, `/salon/delivery`, `/salon/reservas`), `veterinary` (`/mascotas`, `/mascotas/[id]` vaccine card + SOAP, `/hospitalizacion` kennels + vitals, `/grooming`, `NewPetSheet`), `retail` (`/catalogo/variantes/[sku]` size×color matrix; the POS scan field matches sku/id/EAN `77512345…`), `payables` (`/finanzas/cuentas-por-pagar`, `/configuracion/integraciones`), `cash` also has `/finanzas/caja/historial` (Z-reports), `settings` also `/configuracion/auditoria`, `reports` also `/reportes/gestion` (management tabs), `quotes` also `/ventas/proformas/nueva` (3-step wizard), `documents` also `/ventas/comprobantes/[id]/nota-debito` (catálogo 10, serie FD01), `auth`. `dashboard/components/industry-dashboards.tsx` swaps the home page per tenant industry (restaurante / veterinaria; default is the dental dashboard). Pages are server components that import mocks and render a client `*Screen` component holding filter/dialog state.
- `types/domain.ts` — shared domain types (Tenant, SalesDocument/Detail, Customer, CatalogItem, CartLine…). Feature-only types live next to their mocks. Mocks mirror the shape the future API will return; there is no backend yet.
- `lib/format.ts` — currency (`S/`), integer, percent, date (`dd/mm/aaaa`) and `SERIE-NNNNNNNN` formatting. Use these instead of ad-hoc formatting.

Interactions are mock-backed: actions show a sonner toast (`toast.success(...)`) instead of calling an API. Keep that convention until the API exists.

Visual language: the khipu design system (tokens, palettes, type scale) plus the Stitch designs as reference for screen layouts (`design/stitch/screens/*.html`, render them with Playwright at 1440px to compare): KPI `StatCard` with tinted icon box (`tone`), `suffix`, `deltaLabel`, `footer` slot, `emphasizeValue`/`highlight`; `StatCompact` for the icon-left variant; `SectionCard` with `icon`/`iconTone`/`description`; `StatusBadge` bordered pills (`toneClass` exported); `DocumentTypeBadge` uppercase chips; tabs rendered as pill strips (`InventoryTabs`, `PurchasesTabs`) or `TabsList` inside a card; page actions live in `PageHeader actions` (client screens receive the KPI grid via a `kpis` prop and render the header themselves when the actions need state). `StatGrid` is a `<section aria-label="Indicadores">` that scrolls horizontally on mobile (cards `min-w-44`, `shrink-0`, `snap-start`). Pass client-component elements to client screens as data/props, not as pre-built JSX from a server page (it triggers a React key warning).

Never nest a `Button` inside a clickable tile rendered as `<button>` (hydration error, and the Next dev overlay dialog then breaks E2E `getByRole("dialog")` queries) — render the tile as a `div` when it holds actions (see `KennelTile` in `features/veterinary/components/hospital-screen.tsx`). Absolute dropdowns inside `SectionCard` get clipped; render search results inline instead.

### Design system (pinned package)

The tokens, the Tailwind theme, the recipes and the appearance preferences come from the **`khipu-design-system` package** (https://github.com/bacsystem/khipu-design-system), pinned by tag in `apps/web/package.json` (`github:bacsystem/khipu-design-system#v0.5.23`). Nothing of it is vendored in the repo: `src/styles/tokens.css` / `theme.css` no longer exist.

- `app/globals.css` is `@import "tailwindcss"; @import "khipu-design-system/styles.css";` (tw-animate-css, shadcn variants, `dark` variant, tokens, theme, `@source` of the package) plus Wanka-only CSS. Never declare a bare `:root` or `.dark` there: it would override the palette the user chose (`lib/design-system.test.ts` guards it).
- **Fix it in the DS first, then bump the tag.** A change to a token, recipe or shared component goes to the DS repo, gets released as a tag, and reaches Wanka by changing the version in `apps/web/package.json` and running `pnpm install` (the package builds `dist/` in its `prepare` script; pnpm 9 runs it without extra approval; `peerDependencyRules` in the root `package.json` allows Next 16 against the package's `next@^15.5` peer).
- `lib/estilos.ts`, `lib/iconos.ts`, `lib/movimiento.ts` and `lib/utils.ts` are one-line facades over `khipu-design-system/lib/*` and `/utils` (`cn` knows the named type scale).
- **Identifiers that come from the DS stay in Spanish** (documented exception to the "code in English" rule): tokens and utilities (`--acento*`, `--sombra-*`, `--codigo*`, `--borde-tarjeta`, `bg-acento-suave`, `shadow-tarjeta`, `text-etiqueta` / `text-cuerpo` / `text-titulo` / `text-metrica`, `h-control`, `duracion-*`), recipes (`TARJETA`, `ENTRADA`, `FOCO`...), icon weights (`PESO_ICONO*`), `RESORTE`, `useApariencia`, `data-paleta` / `data-oscuro` / `data-fuente`. Wanka's own code (components, hooks, files, mocks, e2e) stays in English.
- Cards use the DS recipe `TARJETA` from `@/lib/estilos` (never the string pasted by hand, never combined with `shadow-xs`).
- Appearance (palette, dark tone, font) is the DS's `AparienciaProvider`; `components/layout/theme-toggle.tsx` is the menu over `useApariencia()`. localStorage keys are the DS's (`khipu-paleta`, `khipu-oscuro`, `khipu-fuente`).
- Fonts: Inter (default) + optional Plus Jakarta Sans, Manrope, DM Sans, Figtree + JetBrains Mono for codes and amounts.
- Icons: transition rule — navigation, shell and any new or touched component use `@phosphor-icons/react` (`PESO_ICONO` duotone default, `PESO_ICONO_ACTIVO` fill for the active state, `PESO_ICONO_FUNCIONAL` bold for functional glyphs); the remaining `lucide-react` imports in feature screens are migrated incrementally, tracked in https://github.com/bacsystem/wanka/issues/123. Don't add new Lucide imports.
- Replacing `components/ui/*`, `DataTable` and the shell with the DS components is follow-up work (stacked PRs), not part of the adoption.

### Responsive contract

- ≥1280px: sidebar expanded, dashboard grid 2/3 + 1/3.
- 768–1279px: sidebar collapsed to icons with tooltips; tables scroll horizontally (`min-w` on the table).
- ≤767px: sidebar hidden, `BottomNav` (Inicio, Ventas, Clientes, Inventario, Más → Sheet), FAB "Nueva venta", KPIs in a horizontal scroller, tables rendered as card lists (`md:hidden` / `hidden md:block` pairs).

## Docs and design assets

`design/` is git-ignored: the Stitch exports referenced below live only on the local machine (re-export them from the Stitch projects if missing).

- `docs/superpowers/specs/` — approved design specs (one per phase).
- `prompt/ui-ux-stitch.md` — Stitch prompts and project ids; `prompt/stitch-flujos-faltantes.md` — prompts for the second batch. Generating from here: call the Stitch MCP tool `generate_screen_from_text` (projectId `3747912429457694633`); the response's `outputComponents[].design.screens[0]` carries the HTML/PNG download URLs (`list_screens` is capped/stale — do not rely on it to discover new screens; `get_screen` by id works). Beware: the design-system JSON contains `on_error`, so never grep responses for "error" to detect failures. `design/stitch/screens/` — the 48 reference screens exported from the user's Stitch project (`index.json` maps file → screen id/title); `design/stitch/DESIGN.md` — design tokens. The Stitch HTML uses Material Symbols; the app uses Lucide only.
- Mermaid diagrams: write `.mmd` files and validate syntax; do not hand-edit diagrams managed by Mermaid Chart Sync (frontmatter with `id:`). See `.github/instructions/mermaid.instructions.md`.
