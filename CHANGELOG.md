# Changelog

All notable changes to this project are documented in this file.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project adheres to [Semantic Versioning](https://semver.org/).

## [0.1.1] - 2026-11-17

### Changed
- Adopted the **khipu 0.5 ("Pastel Compacto")** design system in `apps/web` (`styles/tokens.css`, `styles/theme.css`, `#eceef1` canvas, borderless `shadow-card` surfaces, Lima default palette + 4 optional palettes, and Lavanda `.theme-admin` console theme with full derived tokens in light and dark mode).
- Replaced `Geist` / `Geist Mono` with `Inter` (+ optional `Plus Jakarta Sans`, `Manrope`, `DM Sans`, `Figtree`) and `JetBrains Mono`, with `<AppearanceScript />` to prevent FOUC and a theme/palette/font switcher in `ThemeToggle`.
- Migrated `Sidebar` to floating `rounded-xl` cards with a spring-animated active pill (`m.span` + `layoutId` + `SPRING`) and `Tabs` to `TabsPrimitive.Indicator` sliding transitions.
- Standardized control heights to `h-control` (`2.25rem` / 36 px) with `size="sm"` (`h-7` / 28 px) for dense rows, `rounded-full` buttons with tactile press feedback, and `StatGrid` mobile horizontal scrolling (`min-w-44 shrink-0 snap-start`).
- Introduced `@phosphor-icons/react` (`duotone` default, `fill` active, `bold` functional) across navigation, shell, pagination, and search inputs.
- Added automated design-system guardrails in `apps/web/src/lib/design-system.test.ts` (22 Tailwind palettes, `.theme-admin` derived tokens, and `h-control` / `h-7` assertions) and updated `CLAUDE.md`.

## [0.1.0] - 2026-09-13

### Added
- Tenant app (`apps/web`, Next.js 16): 48 mock-backed screens — POS, SUNAT electronic documents (detail, void, credit/debit notes), quotes board and wizard, dispatch guides and reception, customers and clinical record (FDI odontogram, SOAP), appointments, catalog and retail variants, inventory (stock, movements, warehouses, kardex 13.1, physical count), purchases (RCE, suppliers, orders), receivables/payables, cash and Z-reports, RVIE/settlement/management reports, settings (SUNAT, users/roles, audit, integrations, profile), restaurant (floor, kitchen, delivery, reservations) and veterinary (pets, hospitalization, grooming) modules.
- Industry-aware dashboards and navigation (dental, veterinary, restaurant, retail) driven by the selected tenant cookie.
- SaaS admin console (`/admin`): dashboard, tenants, plans, billing, monitoring, staff, support, feature flags, audit, settings, profile and login, with its own indigo theme and a shell shared with the tenant app.
- Standardized UI kit: 40 px controls, `DataTable` with column visibility / density / reset controls, `EntityHeader`, `PageHeader`, `FilterBar`, `SectionNav`, `SectionCard`, `StatCard`, `StatusBadge`/`Tag`, `SearchInput`, `PaginationBar`, `InitialsAvatar`, dynamic state icons; "Andes Teal v2" theme, dark mode, Geist + Geist Mono.
- Test suites: Vitest unit tests and Playwright E2E across desktop / tablet / mobile (no fixed sleeps; overflow checks are polled).
- `Input size="sm"` for dense table rows, `NumberInput` (raw text + `parseNumber`) for typed quantities and prices, and `config/redirects.ts` as the single source for group landings (checked by a unit test).
- Prompts (`prompt/`), design spec and UI standardization audit (`docs/`); `design/` (Stitch exports) is kept local and git-ignored.
