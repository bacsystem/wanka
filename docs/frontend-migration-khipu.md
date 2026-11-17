# Frontend migration to khipu-design-system (contract)

Status: proposed · 2026-11-17 · base `main` e1cd481, DS pre-0.6.0 · decision record: [ADR 0010](architecture/adr/0010-frontend-khipu-migration.md) (owner decisions confirmed 2026-11-17) · dependency graph: [`frontend-migration-khipu.mmd`](frontend-migration-khipu.mmd).
Owner rules (fixed): the app only uses DS components; no exceptions; no styles outside DS tokens/recipes; anything missing goes to the DS (PR + tag). Inputs: QA UI audit. Counts re-measured on e1cd481 (source files, tests excluded): `ui/button` 97 files, `shared/status-badge` 90, `section-card` 59, `page-header` 52, `stat-card` 36, `data-table` 30, `ui/tabs` 21 (outside ui/shared/layout), `shared/initials-avatar` 9, `sonner` 71 files / 249 `toast.*` calls, `lucide-react` 130 files, `recharts` 10 files, 16 native `type="date"` fields in 13 files.

## 1. Strategy

- **Primitives (`components/ui/*`): direct codemod, no adapter.** They use the same base (Base UI + cva) and the same variant names (`default|outline|secondary|ghost|destructive|link`). Only `size="icon-sm|icon-lg"` maps to `icon` (all `h-control`). An adapter would add indirection for nothing.
- **Shared building blocks (`components/shared/*`): thin temporary adapters, then direct use per module.** `DataTable`, `StatCard`, `StatusBadge`, `SectionCard` and `PageHeader` have 30–90 call sites spread over every module, so flipping their API in one PR would collide with every module PR. Each adapter keeps Wanka's props, renders only DS components (no classes of its own) and is marked `@deprecated`. The whole app gets the DS look in one PR per family. Modules then move to the DS API one by one, and W8 deletes the adapters. Adapters get no new props.
- **Icons, hardcoded styles and `font-mono` on money are fixed inside each module PR**, not in global sweeps, so each file is touched once. A ratchet baseline (§4) proves progress and blocks regressions from day one.
- **File ownership:**
  - W2–W4 codemods exclude `components/shared/**` (owned by W5) and `components/layout/**`, `app/**/layout.tsx` and `features/platform-admin/components/admin-shell.tsx` (owned by W7).
  - Exception: W4 also swaps the toast import and calls in `components/layout/{tenant-switcher,notifications}.tsx` (two lines each). This is safe because W7 starts only after W4 merges, and it lets W4 delete `ui/sonner`.
  - `components/providers/**` (toast provider, `AparienciaProvider`, theme provider, `app-document.tsx`) belongs to W4.
  - A local `ui/*` file is deleted by the PR that removes its last import.
- **Serial chores, never inside a feature PR** (run by the tech-lead after each merge, so parallel PRs never edit the same shared files):
  1. **DS bump:** `chore(web): khipu-design-system vX` changes `apps/web/package.json` + `pnpm-lock.yaml`. Each DS PR ships its own tag; "waits DS-x" means the first tag that contains it.
  2. **Baseline prune:** `chore(web): prune lint baselines` runs `eslint . --prune-suppressions` and `khipu-ds verificar … --podar`.
- **Baseline semantics:**
  - With ESLint 9 bulk suppressions, a new violation exits 1. An unused suppression exits 2 unless you pass `--pass-on-unpruned-suppressions`.
  - `pnpm lint` passes that flag, so a PR that fixes violations does not need to touch `eslint-suppressions.json` or `ds-baseline.json`.
  - The serial prune shrinks both files after each merge. Until the prune, the window is bounded to the counts already in the baseline.
  - I rejected per-module baseline files: they mean one ESLint run per folder (`--suppressions-location`), and W5/W7/shared paths still overlap.
- **Proving a module is clean:** run `echo '{}' > /tmp/sin-supresiones.json && pnpm --filter web exec eslint src/features/<m> --suppressions-location /tmp/sin-supresiones.json` and `khipu-ds verificar --dir src/features/<m>` with no baseline. Both must exit 0. The suppressions file must exist and be `{}`: ESLint 9.39 exits 2 if the path is missing.
- **No load test.** This is a UI-only change with no server, hot-path or capacity impact. Verification is unit tests, E2E and review screenshots.

## 2. Stacked PRs

| PR | Scope (paths) | Waits | Parallel with | Acceptance / verification | Size |
|---|---|---|---|---|---|
| W0 | `lib/utils` `cn` fix (B1, in flight) | — | — | buttons visible; `cn` unit test | S |
| W1a | `eslint.config.mjs` (no-restricted-imports §4) + `eslint-suppressions.json`; `lint` script with `--pass-on-unpruned-suppressions`; `e2e/ds-captures.spec.ts` (review artifacts: 20 routes × 3 viewports × light/dark, not asserted) | W0 | DS work | `pnpm lint` green; a new banned import fails (exit 1) | S |
| W1b | `lint:ds` script (`khipu-ds verificar`) + `ds-baseline.json` | W1a (same `lint` script), DS-G | any W PR except W1a (scripts only; coordinate with bump chores) | adding an arbitrary value or `font-mono` on money fails | S |
| W2 | `features/**`, `app/**` pages: button, badge, card, label, separator, avatar, popover, scroll-area, skeleton, progress, tooltip, alert, table → `khipu-design-system/ui/*`, `feedback/{alerta,tooltip,progreso,skeleton}`; `ui/tabs` (21 files) → `navegacion/tabs` (`Tabs estilo="linea"` for card tabs, `"segmentado"` for pills; panels via `children(id)` or `paneles`) | W1a | W5a | typecheck/test/E2E green; tab E2E by role `tab` | M |
| W3 | same paths: input/textarea → `ui/input`, `formularios/campo` (`Entrada`, `AreaTexto`); checkbox/switch/radio → `Casilla`/`Interruptor`/`GrupoOpciones`; select → `ui/select` (filter bars, `CONTROL_FILTRO`) / `SelectCampo` (forms); toggle-group → `GrupoBotones`; 16 `type="date"` fields (13 files) → `EntradaFecha`; `NumberInput` (7) → `EntradaDecimal` | W2, DS-E | W5a | `getByLabel` still resolves; E2E date helper like the portal's `fecha-de-prueba.ts` covers the 16 fields; decimal comma test ported from `number-input.test.tsx` | L |
| W4 | same paths + `components/providers/**` + toast lines in `components/layout/{tenant-switcher,notifications}.tsx`: dialog → `ui/dialog` + `CabeceraDialogo`/`PieDialogo`; sheet → `ui/sheet`/`PanelLateral`; dropdown-menu → `ui/menu`, row "⋯" → `MenuAcciones`; sonner → `toast` (`success→ok`, `warning→aviso`), `Toaster` → `ToastProvider` | W3, DS-D | W5a | no `sonner` import anywhere, `ui/sonner` deleted; E2E toast `getByText` (flows2:27,54, interactions:30) pass | M |
| W5a | `components/shared/**` adapters: SectionCard → `Card`+`CabeceraSeccion`; PageHeader/EntityHeader → `CabeceraPagina`+`EnlaceVolver`; StatusBadge → `PillEstado`, Tag → `Chip`/`ChipCodigo`; InitialsAvatar → `ui/avatar`; Field → `Campo`; FilterBar/Toolbar/SearchInput → `Toolbar`/`Buscador`; PaginationBar → `PieTabla`; states → `EstadoVacio`/`Skeleton`/`Alerta`; dynamic-icon → Phosphor | W1a | W2–W4 | adapters have no class strings except recipes; existing tests green | M |
| W5b | adapters: `DataTable` → `TablaListado` (`tarjetaMovil`, `anchoMinimo`, `pie`, `filaActiva`); `StatCard`/`StatCompact` → `Metrica`/`Kpi` (§3.1); `StatGrid` → `FilaMetricas desplazable`; `SectionNav` → `TabsRuta` | W5a, DS-A, DS-C, DS-F | W2–W4 | "Columnas"/"Restablecer" E2E (admin:69-73, interactions:119) pass; mobile cards at Pixel 7; every StatCard footer/hint text still rendered | M |
| W6.1–14 | one per module, `features/<m>/**` + its `app/**` pages: dashboard · pos · documents+dispatch-guides · quotes · customers+clinical+appointments · catalog+retail · inventory · purchases+payables · receivables+cash · reports · settings+auth · restaurant · veterinary · platform-admin (screens, not shell). Direct DS API (no adapter imports), Phosphor icons, charts → `datos/grafico` (DS-F), no hardcoded styles, money in Inter `tabular-nums` | W4, W5b (charts: DS-F) | each other and W7 | module clean with no baseline (§1); module E2E green | M–L |
| W7 | `components/layout/**`, `app/(tenant)/(app)/layout.tsx`, `config/navigation.ts` (→ `GrupoNav[]`), admin-shell: `Shell`, `SidebarContent`+`SidebarNav` (rail), `SelectorContexto` (tenant), `MenuUsuario`+`PanelApariencia` (replaces theme-toggle), `TopBar migaComoTitulo={false}`, `Paleta` (⌘K), `CentroNotificaciones`, `BarraInferior`; delete `ui/sidebar`, `sidebar-auto-collapse`, the `sidebar_state` cookie; vitest `server.deps.inline: ["khipu-design-system"]` | W4, DS-0, DS-B, DS-C | W6.x | E2E rewritten to roles (§5); responsive contract §2.1 passes at 1440/1024/Pixel 7; on mobile the bottom nav lists exactly `getBottomNavHrefs(industry)` + Más for a default, a restaurante and a veterinaria tenant | L |
| W8 | delete `components/shared`, `components/ui`, adapters; drop deps `lucide-react`, `sonner`, `recharts`, `cn`, `class-variance-authority`; baselines empty and deleted, `--pass-on-unpruned-suppressions` removed (rules strict); CLAUDE.md (§4); `docs/frontend-structure.md` | all, DS-0 | — | `pnpm lint/typecheck/test/test:e2e/build` green; no peer warnings; GEMINI budget test ≤ 22 000 B | M |

**DS PRs** (bacsystem/khipu-design-system), parallel with each other and with W1–W5a, one tag each: **DS-0** = PR #57 `feat/next-16`, 0.6.0 (published; adopted in Wanka by PR #133): Next 16 peer, plus removing `lucide-react` from `peerDependencies` (no DS source imports it; `IconoComponente` stays structurally typed) and a Next 16 smoke app (M) · DS-G `verificar` CLI (S) · DS-A tables (M) · DS-B shell (L) · DS-C notifications + route tabs (M) · DS-D toast (S) · DS-E numeric input (S) · DS-F charts + KPI (M). Each adds its case to the DS example app and docs/design-system.md and passes `npm run lint` (contrast, heights, typography).

### 2.1 Responsive contract after W7 (replaces CLAUDE.md "Responsive contract")

- ≥1280: DS floating sidebar 248 px; dashboard grid 2/3 + 1/3.
- 768–1279: DS rail 64 px with tooltips. It is automatic; the user can expand it and the choice is remembered. Tables scroll inside their container (`anchoMinimo`).
- ≤767: no sidebar, no hamburger. `BarraInferior` gets its entries as data: Wanka passes the four items whose hrefs come from `getBottomNavHrefs(industry)` (`config/navigation.ts:161-165`) and whose labels and icons come from `getNavigation(industry)`, plus Más (a drawer with the full `SidebarContent`) and the "Nueva venta" FAB. The entries are:
  - default: Inicio, Ventas, Clientes, Inventario;
  - restaurante: Inicio, Salón y mesas, Comandas / cocina, Ventas;
  - veterinaria: Inicio, Ventas, Mascotas, Agenda.
- ≤767, all industries: KPIs use `FilaMetricas desplazable`; lists use `tarjetaMovil`.
- The tenant selector, theme/appearance and the user menu live in the sidebar (DS), not in the top bar. Page titles stay in `CabeceraPagina`, so page actions keep their client state. The top bar holds search, notifications and global actions.

## 3. DS gaps: decision and API contract (Spanish names, DS conventions)

**Adopt the DS behavior** (owner decisions, ADR 0010): dialogs do not close on an outside click (`ui/dialog.tsx:12-17`); no row-density toggle; no tinted box behind KPI icons; NotFound/Forbidden/SessionExpired/Error composed in Wanka with `EstadoVacio` + `Alerta`; DS toast look and position. Two more follow from the rule "everything khipu has": lists gain the DS search/CSV/print, and `StatCompact` becomes `Metrica`. The area chart gradient becomes `area` (below).

**Add to the DS:**
- **DS-A `TablaDatos`/`TablaListado`** (new optional props, backwards compatible):
  - `tarjetaMovil?: (fila: T) => ReactNode`. Below `md` it renders `<ul role="list" aria-label={titulo}>` with one card per row instead of the table. Search, filters, sort, pagination and `seleccion` still apply. With `onFila`, the card is a `<button>` only if it has no interactive children; otherwise it is a `div` with a primary link.
  - `anchoMinimo?: string` (rem) sets the table `min-width` inside an `overflow-x-auto` container.
  - `pie?: Partial<Record<idColumna, ReactNode>>` adds a `<tfoot>` totals row on the visible columns (on mobile, a summary card at the end).
  - `filaActiva?: string` (a `clave`) sets `aria-current="true"` on the active row (table rows and mobile cards) + the `accent` band (§12). Not `aria-selected`: it is only meaningful inside grid/treegrid, and the table already uses "selected" for checkbox selection (khipu PR #60 review).
  - Columns menu: `ocultable: false` shows "Fijo" (`textos.fijo`); a new item "Restablecer" (`textos.restablecer`) restores `ocultaInicial`; `claveAlmacenamiento?: string` persists the visible columns in `localStorage` (try/catch).
- **DS-B shell:**
  - **Rail:** `Shell rail?: "auto" | "nunca"` (default `"nunca"`).
    - `"auto"` gives a CSS-driven 64 px rail at 768–1279, so there is no hydration flash.
    - Toggle: `BotonRail` in the sidebar header (`aria-expanded`, `aria-controls`, "Contraer menú"/"Expandir menú").
    - The preference is `localStorage["khipu-sidebar-rail"] = "rail" | "expandido"`, applied before paint by `ScriptDiseno` as `html[data-rail]`.
    - In the rail: items are icon-only with `Tooltip side="right"` and `aria-label`; group titles are hidden and collapsible groups flattened; `selector` and `usuario` become avatar triggers; the active item uses `PESO_ICONO_ACTIVO`.
  - **Bottom nav:** `BarraInferior({ items: ItemNav[] /* ≤ 4 */, mas?: { etiqueta: string; contenido: ReactNode }, accion?: { href: string; etiqueta: string; icono: IconoComponente }, etiqueta?: string /* "Navegación principal" */ })`.
    - `items` is data supplied by the app (no fixed list in the DS); `accion` is optional.
    - A `<nav aria-label>` fixed to the bottom, shown only below `md`.
    - Items are ≥ 44 px (`OBJETIVO_TACTIL`); the active one is matched by prefix and gets `aria-current="page"`.
    - "Más" opens the `MobileNav` drawer with `contenido`.
    - `accion` is the FAB (`ACCION_PRINCIPAL`, round, above the bar, with `aria-label`).
    - Respects the safe-area inset; `Shell barraInferior?: ReactNode` pads `main` and the toasts.
  - **Top bar:** `TopBar migaComoTitulo?: boolean` (default `true`). With `false` the breadcrumb renders as `<nav aria-label={etiquetaUbicacion}>` text, not an `h1`, which also lets the portal's admin top bar drop its copy.
- **DS-C:**
  - **Notifications:** `CentroNotificaciones({ notificaciones: Notificacion[]; onLeer?: (id) => void; onLeerTodas?: () => void; verTodas?: { href; etiqueta }; textos?: Partial<TextosNotificaciones> })`, where `Notificacion = { id: string; titulo: string; descripcion?: string; fecha: string /* ISO */; tono?: Tono; icono?: IconoComponente; href?: string; leida: boolean }`.
    - Trigger: a `size-control` bell with an unread dot and an `aria-label` like "Notificaciones, 3 sin leer".
    - Panel: `Popover` 360 px wide (full width below `md`) with a `ScrollArea`. Items are links; empty shows a compact `EstadoVacio`; dates use `formatearFechaHora`.
  - **Route tabs:** `TabsRuta({ items: { href: string; etiqueta: ReactNode; contador?: number; exacto?: boolean }[]; etiqueta: string; estilo?: "linea" | "segmentado" })`.
    - A `<nav aria-label>` of `Link`s with `aria-current="page"`, not `role="tablist"`, because they are routes.
    - Same look as `Tabs`; on mobile it scrolls horizontally with the active tab scrolled into view.
- **DS-D `feedback/toast`:** export `toast = { ok, aviso, error, info, promise, cerrar }`.
  - Same signatures as `useToast()`, which returns this same object.
  - Backed by a module store that `ToastProvider` subscribes to, so it can be called from handlers and column renderers outside hooks. Calls made before the provider mounts are queued (max 5).
  - Max 3 visible: bottom-right at `≥md`, bottom-center above `BarraInferior` below `md`.
  - `error` is `aria-live="assertive"`, the rest `polite`.
- **DS-E:** `EntradaDecimal({ id; valor: number | null; onCambio: (v: number | null) => void; min?; max?; decimales?: number /* 2; 0 = entero */; sufijo?: ReactNode /* "%", "kg" */; alinear?: "izquierda" | "derecha"; disabled?; "aria-label"? })`.
  - Keeps the raw text while focused and accepts `.` or `,` as the single decimal separator.
  - Commits the parsed value on each change; on blur it normalizes and clamps to `max`.
  - `inputMode` is decimal or numeric; height `h-control`; Inter `tabular-nums`.
  - DS-E also fixes `EntradaMonto`: `replace(/[^\d.]/g, "")` (`src/components/formularios/entrada-monto.tsx:52`) turns "12,5" into 125.
- **DS-F charts:**
  - `GraficoBarras apiladas?: boolean` (the tooltip shows the total).
  - `GraficoLineas area?: boolean` + `Serie.discontinua?: boolean` (for goals and projections).
  - `GraficoDona({ partes: { nombre; valor; detalle? }[] /* ≤ 5, caller groups "Otros" */; centro?; titulo?; formato?; etiquetadoPor?; descripcion?; tamano? })`. Uses `--chart-1..5` with `--chart-trazo-*` borders; value and % go in the legend and in `<title>`.
- **DS-F KPI** (`TarjetaMetrica`, `Metrica`, `Kpi`, all backwards compatible):
  - `pie?: ReactNode | { etiqueta: ReactNode; valor: ReactNode }` is a bottom band (`border-border/60`, `text-etiqueta`, muted). The object form lays out label on the left and value on the right, so apps don't write `flex justify-between`.
  - `valorConTono?: boolean` paints the value and `ayuda` in the tone's text color (`success-foreground`, `warning-foreground`, `destructive-texto`); `check:contraste` verifies it.
  - On `Kpi`: `sufijo?: ReactNode` and `ayuda?: ReactNode`. `Metrica.ayuda` becomes optional.
  - `FilaMetricas desplazable?: boolean`: below `md` a snap scroller, cards min 11 rem, `role="region"` + `aria-label`, `tabIndex=0`.
- **DS-G:** ship the checks as `khipu-ds verificar <tipografia|alturas|estilos|iconos> --dir <path>... [--base <json>] [--podar]`, exiting 1 with `file:line`.
  - New rule `estilos` flags: arbitrary values `[...]`; hex/rgb/hsl; Tailwind palette colors; `style={{`; `z-*`; `shadow-*` outside the DS scale; `rounded-*` other than `md|lg|xl|full`; `duration-*`; and `font-mono` on a line that formats money (`--monto <regex>`; for Wanka, `formatCurrency\(|S\/`).
  - `--base` holds per-file counts and fails only when a count goes up; `--podar` rewrites the file.

### 3.1 `StatCard` → `Metrica` / `Kpi` (W5b adapter, then W6.x)

The rule: a card with `deltaPercent` becomes `Kpi`; everything else becomes `Metrica`. No content is dropped.

| Wanka prop (uses) | DS | Note |
|---|---|---|
| `label` | `etiqueta` | |
| `value` | `Metrica` children / `Kpi valor` | |
| `suffix` (29) | `sufijo` | `Kpi sufijo` is DS-F |
| `icon` | `icono` (Phosphor) | no box (owner decision 3) |
| `tone` (88) | `tono` | default→`neutro`, success→`ok`, warning→`aviso`, danger→`error`, info→`acento` |
| `deltaPercent` (15) | `Kpi variacion` | |
| `deltaLabel` (8) | `Kpi periodo` | |
| `hint` (71) | `ayuda` | `Kpi ayuda` is DS-F |
| `footer` (70) | `pie` (DS-F) | label/value pairs use the object form |
| `emphasizeValue` (33) | `valorConTono` (DS-F) | |
| `highlight` (2) | `destacada` | |
| `className` | — | no overrides; layout belongs to `FilaMetricas` |
| `StatCompact` (4) | `Metrica` | owner rule |

## 4. Guard rails

- **ESLint `no-restricted-imports`** (W1a, with a bulk-suppressions baseline, see §1) bans `lucide-react`, `sonner`, `recharts`, `cn`, `class-variance-authority`, `@base-ui/react/*` (outside the DS), `@/components/ui/*`, `@/components/shared/*`, and `next-themes` outside `components/providers`.
- **`pnpm lint`** = `eslint . --pass-on-unpruned-suppressions && khipu-ds verificar tipografia alturas estilos iconos --dir src --base ds-baseline.json` (the second part arrives with W1b). Keep `lib/design-system.test.ts` (palette colors, `:root`). The baselines only shrink, through the serial prune chore; W8 deletes them and drops the flag.
- **No CI exists in Wanka** (`.github/` has no workflows), so the guards are advisory until the tech-lead adds a workflow that runs lint, typecheck, test and E2E on PRs.
- **CLAUDE.md (W8).** Replace "UI conventions", "Visual language" and "Responsive contract" (with §2.1). In "Design system (pinned package)", also drop the stale lines: sonner toasts (`toast.success(...)`), "JetBrains Mono for codes and amounts", the Lucide transition rule, and "Replacing `components/ui/*` … is follow-up work". The net size goes down (measured in review: 18 361 → 16 283 B before the extra removals). New text:
  > **UI = khipu design system only.** Every UI piece is imported from `khipu-design-system/*`; Wanka code composes DS components with data and text. Never a local copy, a look-alike component or a `className` that overrides a DS component. Missing component or variant → PR to bacsystem/khipu-design-system, tag, bump. Styles only with DS tokens, the named type scale and `@/lib/estilos` recipes: no `[...]` values, hex/rgb, palette colors, inline `style`, `z-*`, own shadows/radii/durations or Tailwind text sizes. `font-mono` only for identifiers (series-correlativo, SKU, ids); amounts, dates, RUC and quantities are Inter `tabular-nums`. Icons: Phosphor (`PESO_ICONO*`; server components from `@phosphor-icons/react/dist/ssr`). Toasts: `toast` from `khipu-design-system/feedback/toast`. `pnpm lint` enforces it.

## 5. Risks

- **Next 16 peer.** Resolved: DS 0.6.0 (PR #57) accepts `^15.5 || ^16` and no longer lists `lucide-react` as a peer; Wanka adopted it in PR #133 and the `peerDependencyRules` override is gone. Remaining: with `cacheComponents` the DS nav (`next/link`, `usePathname`) must be wrapped in `<Suspense>`, and the DS nav is only smoke-tested on Next 16, so W7 verifies it on Wanka.
- **Changes users will notice** (all confirmed by the owner, ADR 0010; a release note goes with each PR): dialogs ignore outside clicks (Escape, X and the footer still close them); toasts look different and move from top-right (`app-document.tsx:33`) to bottom; the rail is stored in `localStorage` instead of the `sidebar_state` cookie; the tenant switcher and the user menu move to the sidebar; the density toggle is gone; amounts switch from mono to Inter; KPI icon boxes are gone; lists gain search/CSV/print; date fields become the DS calendar.
- **E2E that will break:**
  - `data-slot` selectors in `dashboard.spec.ts:15,20,37`, `pos.spec.ts:8` and `admin.spec.ts:55`. W7 rewrites them to `getByRole("navigation", { name })` + `html[data-rail]`.
  - The bottom nav name (`dashboard.spec.ts:16`) and "Más" (`interactions.spec.ts:155`) keep their names through `BarraInferior`.
  - Native date fills: 16 fields in W3.
  - "Columnas"/"Restablecer" keep their names through the DS `textos`.
- **DS bottleneck.** W5b, W6.x and W7 wait for DS tags; W2–W5a proceed meanwhile. A DS gap found in the middle of a module goes to the DS, never to a local workaround.
- **Bug to avoid now.** Do not use `EntradaMonto` until DS-E fixes the decimal comma.
- **Effort:** S: W0, W1a, W1b, DS-D, DS-E, DS-G · M: W2, W4, W5a, W5b, W8, most W6, DS-0, DS-A, DS-C, DS-F · L: W3, W7, the W6 PRs for pos/documents/reports/platform-admin, DS-B.
