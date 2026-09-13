# Changelog

All notable changes to this project are documented in this file.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project adheres to [Semantic Versioning](https://semver.org/).

## [0.1.0] - 2026-09-13

### Added
- Tenant app (`apps/web`, Next.js 16): 48 mock-backed screens — POS, SUNAT electronic documents (detail, void, credit/debit notes), quotes board and wizard, dispatch guides and reception, customers and clinical record (FDI odontogram, SOAP), appointments, catalog and retail variants, inventory (stock, movements, warehouses, kardex 13.1, physical count), purchases (RCE, suppliers, orders), receivables/payables, cash and Z-reports, RVIE/settlement/management reports, settings (SUNAT, users/roles, audit, integrations, profile), restaurant (floor, kitchen, delivery, reservations) and veterinary (pets, hospitalization, grooming) modules.
- Industry-aware dashboards and navigation (dental, veterinary, restaurant, retail) driven by the selected tenant cookie.
- SaaS admin console (`/admin`): dashboard, tenants, plans, billing, monitoring, staff, support, feature flags, audit, settings, profile and login, with its own indigo theme and a shell shared with the tenant app.
- Standardized UI kit: 40 px controls, `DataTable` with column visibility / density / reset controls, `EntityHeader`, `PageHeader`, `FilterBar`, `SectionNav`, `SectionCard`, `StatCard`, `StatusBadge`/`Tag`, `SearchInput`, `PaginationBar`, `InitialsAvatar`, dynamic state icons; "Andes Teal v2" theme, dark mode, Geist + Geist Mono.
- Test suites: Vitest unit tests and Playwright E2E across desktop / tablet / mobile (no fixed sleeps; overflow checks are polled).
- `Input size="sm"` for dense table rows, `parseNumber` for typed quantities, and `config/redirects.ts` as the single source for group landings (checked by a unit test).
- Prompts (`prompt/`), design spec and UI standardization audit (`docs/`); `design/` (Stitch exports) is kept local and git-ignored.
