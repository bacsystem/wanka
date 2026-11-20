# CLAUDE.md

Guidance for coding agents in this repository: Claude Code reads it directly; Gemini CLI and Antigravity through the root `GEMINI.md` (see "Agent context files").

## Project

SaaS de gestión empresarial multirubro para Perú (restaurantes, veterinarias, odontologías, retail): clientes, catálogo, inventario multialmacén, ventas/POS, facturación electrónica SUNAT (boleta, factura, nota de crédito, proforma, guía de remisión) y reportes. Código, identificadores y mensajes de log y de error en inglés; comentarios en español (los existentes se pasan al tocar el archivo); textos de UI en español (Perú), moneda `S/`, fechas `dd/mm/aaaa`.

pnpm workspace monorepo. Only app so far: `apps/web` (Next.js 16 App Router, React 19, TypeScript, Tailwind v4, shadcn/ui on Base UI, Phosphor icons (lucide-react in legacy screens, see Icons), Recharts). Design system: the pinned package `khipu-design-system` (see Design system).

## Project memory: `wanka-memory`

**Before the day's work or any development, consult `wanka-memory`** (the project's NotebookLM notebook) on the project status and the topic at hand, via the `gemini-notebook-mcp` MCP server or the `nlm` CLI, and say in one line what was consulted and what came back.

- Only where that server is set up (the notebook is the owner's); elsewhere, skip it.
- If the query fails (expired Google session, server down), say so and don't carry on as if it had been consulted. `nlm login` renews the session.
- It holds copies of repo docs and can lag: the repo and GitHub win.
- Only public content goes in (the repo is public): no secrets, customer data, pricing or commercial strategy, nor anything git-ignored (`design/`).
- **Log each closed block of work** (merged PR, decision, end of session) as a text **source** (not a note: queries read sources) titled `sesion-YYYY-MM-DD`, or `-2`, `-3` for later blocks that day, under ~1000 characters:

  ```
  sesion-YYYY-MM-DD[-n] · <topic in a few words>
  Hecho: #PR → #issue, one line each (what changed, not how it was reviewed)
  Decisiones: the decision and why (only what no issue, PR or ADR records)
  Pendiente: what · who · blocked by
  Lecciones: a trap in the environment or the workflow (optional)
  ```

  Leave out what GitHub keeps (test counts, review details, measurements): the PR number is enough.
- **Consolidate weekly.** The first session of an ISO week merges the previous week's `sesion-*` into one `semana-YYYY-Www` source with what is still true (decisions, open pending, lessons) and deletes them.

## Team rules

- Commits and PRs (agents' too) go through `/git-flow`, never a hand-run `gh pr create`.
- Delivery and done: `docs/delivery-process.md`; working with Claude or Gemini and handing work over: `docs/ai-collaboration.md`. Story PRs from `main`, no version bump, a `CHANGELOG` `[Unreleased]` entry; one release PR per batch, tagged by `release-tag.yml`.
- Every PR gets `/pr-review` posted as a PR comment (re-reviews too) and QA on its final SHA.
- No merge or release without the owner's explicit approval; no new work during an owner's halt.
- State docs and the GitHub board stay current on every change (`delivery-process.md` «Tablero», skill `docs-sync`); status answers come from `gh`.
- Frontend by wave (owner, 2026-11-19): `apps/web` is rebuilt as a new app on khipu-design-system; only what a wave finished end to end on the real API is reachable, through the enabled-features list in `config/navigation.ts` (menu, Ctrl+K and routes; the rest is 404, test-guarded; never commented-out code). New UI uses DS components only, no hardcoded styles (`[...]`, hex/rgb, palette colors, inline `style`); a missing component goes to the DS first. Detail: `docs/frontend-migration-khipu.md` §0.

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

Whole local stack: `make up` (PostgreSQL, migrations, API, web). Single tests, first E2E run and URLs: `docs/local-environment.md`.

Note: `next dev` re-writes `apps/web/AGENTS.md`, which points to the Next 16 docs in `node_modules/next/dist/docs/`: check them, not memory.

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
- Root layouts (three, none at the top of `app/`): `app/(tenant)/layout.tsx` (tenant app + auth), `app/(admin)/layout.tsx` and `app/(admin-auth)/layout.tsx` (console, `palette="indigo"`). All three render `components/providers/app-document.tsx`: fonts from `app/fonts.ts` (`next/font/local`, woff2 + OFL licences in `app/fonts/`, no network at build: Inter → `--font-sans`, JetBrains Mono → `--font-mono`; Plus Jakarta Sans / Manrope / DM Sans / Figtree → `--fuente-*`, chosen with `data-fuente`), `ScriptDiseno` in `<head>` (applies the saved palette / dark tone / font before the first paint), `ThemeProvider` (`components/providers/theme-provider.tsx`: next-themes class strategy + the design system's `DisenoProvider`, `AparienciaProvider`, `ProveedorMovimiento`), `TooltipProvider`, sonner `Toaster`, `lang="es-PE"`.
- `app/(tenant)/(app)/layout.tsx` — reads the `sidebar_state` cookie and wraps pages in `components/layout/app-shell.tsx` (shadcn `SidebarProvider` + `AppSidebar` + `Topbar` + `BottomNav`). `app/(tenant)/(app)/[...slug]/page.tsx` renders "Módulo en construcción" for any nav entry without a screen (none at the moment; keep it for future modules). `next.config.ts` redirects group routes (`/ventas`, `/inventario`, `/finanzas`, `/configuracion`) to the landing module declared in `config/redirects.ts`; `navigation.test.ts` checks each landing is a child of its group.
- `config/navigation.ts` — single source of truth for navigation (label, href, icon, children). `getNavigation(industry)` / `getBottomNavHrefs(industry)` add the industry modules (odontología: Agenda; veterinaria: Mascotas, Agenda, Hospitalización, Grooming, and Clientes becomes Propietarios; restaurante: Salón y mesas, Comandas / cocina, Delivery, Reservas). Sidebar and bottom nav read the tenant industry from `components/layout/tenant-context.tsx` (`useTenant`). `app/(tenant)/(app)/[...slug]/page.tsx` shows a placeholder only for hrefs present in some industry's navigation; anything else is a real 404 (`app/(tenant)/(app)/not-found.tsx`). Add a nav entry here when adding a screen.
- `components/layout/command-palette.tsx` (Ctrl/Cmd+K global search over mocks, opened from `topbar-search.tsx`) and `notifications.tsx` (bell popover). Base UI menus: a `DropdownMenuLabel` MUST be inside `DropdownMenuGroup` or the page crashes at open time.
- `components/layout/*` — shell only. `sidebar-nav.tsx`, `bottom-nav.tsx`, `theme-toggle.tsx` are client components; `sidebar-auto-collapse.tsx` collapses the sidebar to icons on 768–1279px when no cookie is set.
- `components/shared/states.tsx` — reusable EmptyState / ErrorState / NotFoundState / ForbiddenState / SessionExpiredState / LoadingState; `app/(tenant)/(app)/{loading,error,not-found}.tsx` use them and `DataTable` renders EmptyState when there are no rows. `components/shared/field.tsx` auto-wires `<Label htmlFor>` to its single child control (so tests can use `getByLabel`).
- `components/shared/*` — building blocks every screen uses: `PageHeader`, `StatCard`/`StatGrid` (KPI row, horizontal scroller on mobile), `StatusBadge` (tone: success/warning/danger/info/neutral), `DataTable` (desktop table + `mobileCard` renderer for ≤767px; pass `minWidth` so wide tables scroll inside the card), `SectionCard`, `EntityHeader`, `FilterBar`, `SectionNav`, `Toolbar`/`SearchInput`, `Field`, `PaginationBar`, `InitialsAvatar`, `Tag`.
- `components/ui/*` — shadcn generated (Base UI variant): use the `render` prop instead of `asChild`, and add `nativeButton={false}` when a `Button` renders a `Link`. `Select` needs `items={[...]}` so `SelectValue` shows labels.
- `features/*` — one folder per screen/domain; the per-feature inventory (routes, screens, special flows) is in `docs/frontend-structure.md`.
- `types/domain.ts` — shared domain types (Tenant, SalesDocument/Detail, Customer, CatalogItem, CartLine…). Feature-only types live next to their mocks. Mocks mirror the shape the API returns; no screen calls `apps/api` yet.
- `lib/format.ts` — currency (`S/`), integer, percent, date (`dd/mm/aaaa`) and `SERIE-NNNNNNNN` formatting. Use these instead of ad-hoc formatting.

Enabled screens call `apps/api` through the BFF (Next route handlers) with a client typed from the OpenAPI; unreachable mock screens keep their toasts.

Visual language: the khipu design system (tokens, palettes, type scale) plus the Stitch designs as reference for screen layouts (`design/stitch/screens/*.html`, render them with Playwright at 1440px to compare): KPI `StatCard` with tinted icon box (`tone`), `suffix`, `deltaLabel`, `footer` slot, `emphasizeValue`/`highlight`; `StatCompact` for the icon-left variant; `SectionCard` with `icon`/`iconTone`/`description`; `StatusBadge` bordered pills (`toneClass` exported); `DocumentTypeBadge` uppercase chips; tabs rendered as pill strips (`InventoryTabs`, `PurchasesTabs`) or `TabsList` inside a card; page actions live in `PageHeader actions` (client screens receive the KPI grid via a `kpis` prop and render the header themselves when the actions need state). `StatGrid` is a `<section aria-label="Indicadores">` that scrolls horizontally on mobile (cards `min-w-44`, `shrink-0`, `snap-start`). Pass client-component elements to client screens as data/props, not as pre-built JSX from a server page (it triggers a React key warning).

Never nest a `Button` inside a clickable tile rendered as `<button>` (hydration error, and the Next dev overlay dialog then breaks E2E `getByRole("dialog")` queries) — render the tile as a `div` when it holds actions (see `KennelTile` in `features/veterinary/components/hospital-screen.tsx`). Absolute dropdowns inside `SectionCard` get clipped; render search results inline instead.

### Design system (pinned package)

The tokens, the Tailwind theme, the recipes and the appearance preferences come from the **`khipu-design-system` package** (https://github.com/bacsystem/khipu-design-system), pinned by tag in `apps/web/package.json` (`github:bacsystem/khipu-design-system#v0.10.0`). Nothing of it is vendored in the repo: `src/styles/tokens.css` / `theme.css` no longer exist.

- `app/globals.css` is `@import "tailwindcss"; @import "khipu-design-system/styles.css";` (tw-animate-css, shadcn variants, `dark` variant, tokens, theme, `@source` of the package) plus Wanka-only CSS. Never redefine a DS token in a bare `:root` or declare `.dark` there: it would override the palette the user chose (only Wanka's own `--brand-panel` is allowed; `lib/design-system.test.ts` guards it).
- **Fix it in the DS first, then bump the tag.** A change to a token, recipe or shared component goes to the DS repo, gets released as a tag, and reaches Wanka by changing the version in `apps/web/package.json` and running `pnpm install` (the package builds `dist/` in its `prepare` script; pnpm 9 runs it without extra approval; with `cacheComponents` the DS `nav/*` components must be wrapped in `<Suspense>`).
- `lib/estilos.ts`, `lib/iconos.ts`, `lib/movimiento.ts` and `lib/utils.ts` are one-line facades over `khipu-design-system/lib/*` and `/utils` (`cn` knows the named type scale).
- **Identifiers that come from the DS stay in Spanish** (documented exception to the "code in English" rule): tokens and utilities (`--acento*`, `--sombra-*`, `--codigo*`, `--borde-tarjeta`, `bg-acento-suave`, `shadow-tarjeta`, `text-etiqueta` / `text-cuerpo` / `text-titulo` / `text-metrica`, `h-control`, `duracion-*`), recipes (`TARJETA`, `ENTRADA`, `FOCO`...), icon weights (`PESO_ICONO*`), `RESORTE`, `useApariencia`, `data-paleta` / `data-oscuro` / `data-fuente`. Wanka's own code (components, hooks, files, mocks, e2e) stays in English.
- Cards use the DS recipe `TARJETA` from `@/lib/estilos` (never the string pasted by hand, never combined with `shadow-xs`).
- Appearance (palette, dark tone, font) is the DS's `AparienciaProvider`; `components/layout/theme-toggle.tsx` is the menu over `useApariencia()`. localStorage keys are the DS's (`khipu-paleta`, `khipu-oscuro`, `khipu-fuente`).
- Fonts: Inter (default) + optional Plus Jakarta Sans, Manrope, DM Sans, Figtree + JetBrains Mono for codes and amounts.
- Icons: transition rule — navigation, shell and any new or touched component use `@phosphor-icons/react` (`PESO_ICONO` duotone default, `PESO_ICONO_ACTIVO` fill for the active state, `PESO_ICONO_FUNCIONAL` bold for functional glyphs); the remaining `lucide-react` imports in feature screens are migrated incrementally, tracked in https://github.com/bacsystem/wanka/issues/123. Don't add new Lucide imports.

### Responsive contract

- ≥1280px: sidebar expanded, dashboard grid 2/3 + 1/3.
- 768–1279px: sidebar collapsed to icons with tooltips; tables scroll horizontally (`min-w` on the table).
- ≤767px: sidebar hidden, `BottomNav` (four entries from `getBottomNavHrefs(industry)`, default Inicio, Ventas, Clientes, Inventario, plus Más → Sheet), FAB "Nueva venta", KPIs in a horizontal scroller, tables rendered as card lists (`md:hidden` / `hidden md:block` pairs).

## Docs and design assets

`design/` is git-ignored: the Stitch exports referenced below live only on the local machine (re-export them from the Stitch projects if missing).

- `docs/superpowers/specs/` — approved design specs (one per phase).
- `prompt/ui-ux-stitch.md` — Stitch prompts and project ids; `prompt/stitch-flujos-faltantes.md` — prompts for the second batch. How to generate with the Stitch MCP tool: `prompt/ui-ux-stitch.md`. `design/stitch/screens/` — the 48 reference screens exported from the user's Stitch project (`index.json` maps file → screen id/title); `design/stitch/DESIGN.md` — design tokens. The Stitch HTML uses Material Symbols; the app uses Phosphor (see Icons).
- Mermaid diagrams (docs and `docs/architecture`): `.mmd` files validated before committing; never hand-edit diagrams managed by Mermaid Chart Sync (frontmatter `id:`). See `.github/instructions/mermaid.instructions.md`.

## Backend architecture (`docs/architecture`)

Kernel (wave 0) built in `apps/api`; business modules start in wave 1. The design is in `docs/architecture/` (index and full detail: `README.md`; decisions: `adr/`). Rules:

- **Stack** (ADR 0002): Go modular monolith on PostgreSQL 16. Libraries: only those ADR 0002 lists as approved; any other needs owner approval first.
- **ADRs**: `docs/architecture/adr/NNNN-*.md`; to add, change or supersede one, follow `docs/architecture/README.md` §9.
- **Multi-tenancy** (ADR 0001): every tenant table has `tenant_id` + `FORCE ROW LEVEL SECURITY` with `tenant_id = app.current_tenant()` (`NULLIF`); uniques and FKs include `tenant_id`; data access only via `InTenantTx(Actor{..., Kind})`; no `BYPASSRLS`; `SECURITY DEFINER` functions only owned by `wanka_definer` with a pinned `search_path` (one owner-approved exception: `audit.ensure_partitions`, ADR 0001 «Excepción: DDL de particiones»); extra policies only those whitelisted in ADR 0001 and ADR 0012.
- **RLS check**: any change to policies, roles, grants or definer functions updates `docs/architecture/data/rls-check/` and must pass `sh docs/architecture/data/rls-check/run.sh` (Docker, exit 0).
- **API**: `apps/api/openapi/openapi.yaml` is the contract source (rules in `api/conventions.md`); permission codes only from the catalog in `api/resources.md`. The frontend mocks mirror it until replaced by clients typed from the spec.
- **Money and numbering** (ADR 0008): `numeric` with explicit scale, never float; `currency` (PEN/USD) on every amount, `exchange_rate numeric(6,3)`; correlatives per series per cash register, inside the document's transaction.
- **E-invoicing** (ADR 0003): only through Khipu behind `InvoicingProvider`, never SUNAT directly; the Khipu BETA checks gate #56.
- **Deployment** (ADR 0007): Railway; only `web` and `webhooks` are public; write secret names, never values.

## Agent context files

`CLAUDE.md` is the single source of truth: edit only this file. `GEMINI.md` (root) only imports it, and `apps/web/CLAUDE.md` / `apps/web/GEMINI.md` only import `apps/web/AGENTS.md` (regenerated by `next dev`). How Claude Code, Gemini CLI and Antigravity load them: `docs/agent-context.md`. Keep this file small: `apps/web/src/test/agent-context.test.ts` (`pnpm test`) fails if the expanded root `GEMINI.md` exceeds 22 000 B (Antigravity truncates at 24 000 B), so move detail into linked docs instead. Never write Antigravity's include syntax (an at-sign followed by a Markdown link) in this file.
