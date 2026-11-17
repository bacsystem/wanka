# Changelog

All notable changes to this project are documented in this file.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Fixed
- Clean builds no longer fail at random with `next/font/google queries have exactly one entry` (#125): the fonts are self-hosted with `next/font/local` (`apps/web/src/app/fonts/`: latin woff2 of Inter, JetBrains Mono, Plus Jakarta Sans, Manrope, DM Sans and Figtree, all variable fonts, with their SIL OFL licences). Same CSS variables and rendering; JetBrains Mono is declared with weight `100 600` so `font-bold` still renders as 600, as before. Known difference: only the `latin` subset is shipped, so latin-ext characters (e.g. Ł ř Ş ₡ ₲) fall back to the system font (adding latin-ext costs ~165 KB). `pnpm build` needs no network.

### Changed
- `khipu-design-system` bumped from `v0.5.23` to `v0.6.0` (Next 16 peer support; `lucide-react` is no longer a peer of the package, Wanka keeps its own dependency until #123).

### Removed
- The `pnpm.peerDependencyRules.allowedVersions` override for `next` in the root `package.json`: it existed only because the package declared Next 15.

## [0.2.0] - 2026-11-17

### Added
- `khipu-design-system` (https://github.com/bacsystem/khipu-design-system), pinned at `v0.5.23`, as the source of the tokens, the Tailwind theme, the recipes (`lib/estilos`, `lib/iconos`, `lib/movimiento`, `cn`) and the appearance preferences (`AparienciaProvider`, `ScriptDiseno`). `app/globals.css` imports `khipu-design-system/styles.css`; the vendored `tokens.css` / `theme.css` are gone.
- User-selectable palette (Lima default, Indigo, Oceano, Coral, Violeta) and typography (Inter, Plus Jakarta Sans, Manrope, DM Sans, Figtree) in the theme menu, stored in localStorage and applied before the first paint. The palette choice is hidden inside the admin console, which forces the `indigo` palette.
- `@phosphor-icons/react` in navigation, shell, pagination and status icons (`duotone` default, `fill` active, `bold` functional); remaining Lucide screens tracked in #123.
- Design-system guardrails: `lib/design-system.test.ts` (22 Tailwind palettes hardcoded, `globals.css` does not override DS tokens, controls are `h-control`, admin palette) and `lib/design-tokens.test.ts` (no self-referencing custom properties, every `var()` and every token class used in TSX resolves against the package CSS). E2E `design-system.spec.ts` (mobile KPI scroller, palettes, dark tokens, FAB elevation).

### Changed
- Visual migration to khipu 0.5 "Pastel Compacto": `#eceef1` canvas, borderless `TARJETA` cards, floating sidebar with a spring-animated active pill, sliding `Tabs` indicator, `rounded-full` buttons, Inter + JetBrains Mono instead of Geist.
- Controls measure `h-control` (32 px, from the design system) in every size; no local height overrides.
- The admin console uses the design system's `indigo` palette (`AdminPalette` sets `data-paleta="indigo"` while an admin page is mounted and restores the user's palette on leaving) instead of the local `.theme-admin` token overrides.
- `StatGrid` is a `<section aria-label="Indicadores">`; on mobile its cards keep `min-w-44` and scroll horizontally.
- Demo session data moved to `features/auth/mocks/demo-session.ts`.
- `CLAUDE.md` describes the pinned package, how to upgrade it, the Phosphor transition rule and the Spanish-identifier exception for DS names.

### Fixed
- Transitions of buttons, tabs and the sidebar use the design system's `--duracion-*` tokens (the old `--duration-*` names did not exist and gave 0 s).
- KDS tickets keep their border (`border border-transparent` base) so the on-time and late states show; the "pending" lane dot is visible again.
- No `shadow-xs` mixed with the card shadow.
- Tokens that pointed to undeclared variables (`shadow-lg`, `shadow-elevada`, `--color-code`, `accent-3-suave`, the `acento-superficie-*` classes) now resolve; `--accent-border` and `--acento-borde` are two tokens again.

## [0.1.0] - 2026-09-13

### Added
- Tenant app (`apps/web`, Next.js 16): 48 mock-backed screens — POS, SUNAT electronic documents (detail, void, credit/debit notes), quotes board and wizard, dispatch guides and reception, customers and clinical record (FDI odontogram, SOAP), appointments, catalog and retail variants, inventory (stock, movements, warehouses, kardex 13.1, physical count), purchases (RCE, suppliers, orders), receivables/payables, cash and Z-reports, RVIE/settlement/management reports, settings (SUNAT, users/roles, audit, integrations, profile), restaurant (floor, kitchen, delivery, reservations) and veterinary (pets, hospitalization, grooming) modules.
- Industry-aware dashboards and navigation (dental, veterinary, restaurant, retail) driven by the selected tenant cookie.
- SaaS admin console (`/admin`): dashboard, tenants, plans, billing, monitoring, staff, support, feature flags, audit, settings, profile and login, with its own indigo theme and a shell shared with the tenant app.
- Standardized UI kit: 40 px controls, `DataTable` with column visibility / density / reset controls, `EntityHeader`, `PageHeader`, `FilterBar`, `SectionNav`, `SectionCard`, `StatCard`, `StatusBadge`/`Tag`, `SearchInput`, `PaginationBar`, `InitialsAvatar`, dynamic state icons; "Andes Teal v2" theme, dark mode, Geist + Geist Mono.
- Test suites: Vitest unit tests and Playwright E2E across desktop / tablet / mobile (no fixed sleeps; overflow checks are polled).
- `Input size="sm"` for dense table rows, `NumberInput` (raw text + `parseNumber`) for typed quantities and prices, and `config/redirects.ts` as the single source for group landings (checked by a unit test).
- Prompts (`prompt/`), design spec and UI standardization audit (`docs/`); `design/` (Stitch exports) is kept local and git-ignored.
