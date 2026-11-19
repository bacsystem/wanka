# Changelog

All notable changes to this project are documented in this file.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project adheres to [Semantic Versioning](https://semver.org/).

## [0.5.0] - 2026-11-18

### Added
- `apps/api`: HTTP base of the kernel in `internal/platform/httpx`. Every error is RFC 9457 `application/problem+json` with `type`, `title`, `status`, `code` and `requestId`; the HTTP of each `code` comes from one table that a test compares with the 31 codes of `api/conventions.md` §4. `httpx.Router` answers `404 not_found` for undeclared routes and methods (including `OPTIONS`; no `405`, no CORS); a handler that exceeds its 10 s budget answers `503 overloaded` with `Retry-After: 1` after cancelling its context (the transaction rolls back and the connection returns to the pool) and discarding its buffered output; `httpx.Streaming()` routes skip that budget and get a 25 s context from arrival. `httpx.NewServer` disables net/http's own `OPTIONS *` answer and `Router.Handle` rejects subtree patterns (`/x/`, `{p...}`) so no response comes from `ServeMux`. `Content-Length` over 1 MB is `413 payload_too_large` without reading the body, `httpx.DecodeJSON` answers `400 malformed_request`, and a `panic` is `500 internal` without details (#193, #165).
- `apps/api`: in-memory token-bucket rate limiter (implemented as GCRA, exact integer arithmetic) with an injectable key and clock (`httpx.NewLimiter`), `RateLimit-Limit`/`-Remaining`/`-Reset` headers, no token spent by a rejected request across chained limiters, a cap on keys (`503 overloaded`) and `Sweep` of full buckets. Wiring it to session and tenant keys stays in #174 (#193, #165).
- `apps/api`: `internal/platform/clock` (system and fake clocks) and `internal/platform/metrics` (metrics port; the OpenTelemetry adapter comes later); `db.Pool.AcquiredConns` (#193, #165).

### Fixed
- `apps/api`: a lost database connection (SQLSTATE `57P01`-`57P03` or class `08`, or a connection pgx reports closed; never the shape of the error, so a foreign `io.EOF`, a `url.Error` or `conn busy` stay `500`) with the caller's context alive is now `db.ErrUnavailable` and answers `503 dependency_unavailable` instead of `500` when a pool connection dies after a restart or failover; a streaming handler that writes its own RFC 9457 problem after the budget expired now delivers it; the access log marks a connection cut with `http.ErrAbortHandler` as `aborted` (status 0 if nothing was sent) instead of `200`; a connection lost during `COMMIT` also carries `db.ErrCommitUncertain` (log `phase=commit`) and every database outage logs one `warn` line and counts `db_unavailable_total` (#193, #165). `httpx.NewServer` sets `MaxHeaderBytes` to 64 KiB.

### Changed
- `docs/architecture/api/conventions.md` §4: requests that `net/http` rejects before the API chain (invalid escape in the path, garbage request line, bare LF in a header) are answered by Go's server as `400` `text/plain`, a documented exception to the RFC 9457 rule (#193, #165).
- `docs/architecture/api/conventions.md` §3: `RateLimit-*` are not sent on the kernel's `503` for an expired handler deadline nor on the `503` for the limiter's key cap (no bucket to report), nor on the `500` for a `panic`; the deadline and panic responses drop the handler's headers. Clarification only; no behaviour change (#193, #165).
- `apps/api`: `httpx.WriteDBError` answers `db.ErrStatementTimeout` as `503 dependency_unavailable` with `Retry-After: 1` and the generic body, logs one `warn` line (route, phase, module, `sqlstate=57014`, elapsed; no SQL) and counts it by phase and route; with the request context already finished it writes nothing. `httpx.Chain` takes a metrics recorder, `httpx.WriteProblem` takes the `code` and derives the HTTP from the table, and `httpx.NewMux` returns an `httpx.Router` (#193, #165).
- `apps/api/README.md`: `WriteDBError` no longer described as translating everything to `overloaded`; the three database cases and the HTTP base are documented (#193, #165).
## [0.4.1] - 2026-11-18

### Changed
- API contract, ahead of #165: a handler that exceeds its 10 s budget answers `503 overloaded` with `Retry-After: 1` and an RFC 9457 body (streaming routes excluded); `RateLimit-Limit`, `RateLimit-Remaining` and `RateLimit-Reset` keep three separate fields with integer values, reporting the bucket with fewer tokens; an invalid `X-Request-Id` (empty, over 128 characters or outside `[A-Za-z0-9_-]`) is replaced, never answered with `400`; an undeclared method is `404` and a non-JSON `Content-Type` is `400 malformed_request` (`api/conventions.md` §3, §4 and §7, `openapi.yaml`, C4 components).


## [0.4.0] - 2026-11-18

### Added
- `apps/api`: `db.Pool.InTenantTx` and `InTenantReadTx` open every tenant transaction with `app.tenant_id`, `app.user_id`, `app.session_kind` and `app.impersonator_id` set locally (`set_config(..., true)`), commit on `nil` and roll back on error or panic. `Actor{TenantID, UserID, Kind, ImpersonatorID}` has no valid zero `Kind`; an empty tenant returns `ErrNoTenant`, an incoherent actor `ErrInvalidActor`, and a `statement_timeout` arrives as `ErrStatementTimeout`. RLS isolation, connection reuse and rollback are tested on a real PostgreSQL 16 with a test-only table (#190, #163).
- CI: `api.yml` runs `rls-check/run.sh` (must pass) and `MUTATION=naive_session_lookup` (must fail), also when `docs/architecture/data/rls-check/` changes; go-arch-lint rejects `pgxpool` and `pgx/stdlib` outside `internal/platform/db` and `scripts/archlint.sh` proves it (#190, #163).
- `docs/architecture/capacity.md`: measured overhead of `BEGIN` + `set_config` (one round trip, 0.26 ms in the measurement) (#190, #163).

### Changed
- `apps/api`: `db.Pool.Acquire` is no longer exported: the pool no longer hands out raw connections, and a test fails if code outside `internal/platform/db` calls `pgx.Connect`, `pgconn.Connect*`, `pgxpool.New*` or `stdlib.Open*` (#190, #163).

## [0.3.1] - 2026-11-18

### Changed
- API contract: a statement cancelled by `statement_timeout` (SQLSTATE `57014`) answers `503 dependency_unavailable` with `Retry-After: 1` on every route, with the same body as any dependency failure; the kernel/module distinction goes only to the log. `GET /v1/documents` now declares its `503` (`api/conventions.md` §4 and §7, `openapi.yaml`).

## [0.3.0] - 2026-11-18

### Added
- `apps/api`: one `pgxpool` per process with its own database role (`DATABASE_URL_APP`, `DATABASE_URL_WORKER`, `DATABASE_URL_WEBHOOKS`, `DATABASE_URL_PLATFORM`, `DATABASE_URL_MIGRATOR`; size from `DB_POOL_MAX_CONNS`), `wanka migrate up` with goose migrations embedded in the binary (version table in its own `goose` schema), the idempotent role bootstrap `apps/api/bootstrap/roles.sql`, `/readyz` backed by the database (`503 dependency_unavailable` / `503 overloaded`), and a CI check that keeps merged migrations forward-only (#184, #162).

### Changed
- `DATABASE_URL` is no longer read: each command needs the URL of its own role (#184).

## [0.2.2] - 2026-11-18

### Changed
- Losing a company membership no longer ends the session: the session is unbound from that company, tenant routes answer `403 tenant_access_denied` and the user goes to `/seleccionar-empresa` (to `/login` only with no active membership left). ADR 0011, `api/conventions.md` §0 and §4, sequences 01 and 08 (#185).

## [0.2.1] - 2026-11-18

### Changed
- Every PR bumps the version: `feat` and breaking changes raise the minor (also before 1.0), every other type the patch (`CLAUDE.md` › Team rules).
- Code comments are written in Spanish; code, identifiers, logs and error messages stay in English (#183).
- Commits and PRs, including agents', go through the `/git-flow` skill (#182).
- CI: the `e2e` job skips its steps on PRs that do not touch the frontend (#186).

### Fixed
- Clean builds no longer fail at random with `next/font/google queries have exactly one entry` (#125): the fonts are self-hosted with `next/font/local` (`apps/web/src/app/fonts/`: latin woff2 of Inter, JetBrains Mono, Plus Jakarta Sans, Manrope, DM Sans and Figtree, all variable fonts, with their SIL OFL licences). Same CSS variables and rendering; JetBrains Mono is declared with weight `100 600` so `font-bold` still renders as 600, as before. Known difference: only the `latin` subset is shipped, so latin-ext characters (e.g. Ł ř Ş ₡ ₲) fall back to the system font (adding latin-ext costs ~165 KB). `pnpm build` needs no network.

### Changed
- `khipu-design-system` bumped from `v0.8.0` to `v0.10.0` (adds DS-C notifications and route tabs, DS-D imperative `toast`, DS-E `EntradaDecimal`, DS-F charts and KPI props, DS-G `khipu-ds verificar`, DS-I `PillEstado` info / `Chip` tones / `PieTabla`, DS-J `Tabs` counter tone and scroll, and the `EntradaDecimal` integer thousands-dot fix). Visible change for Wanka: `Tabs estilo="segmentado"` no longer wraps into several rows on phones (single scrollable row) and tab counters follow the new tone recipe. No Wanka code change needed.
- `khipu-design-system` bumped from `v0.6.0` to `v0.8.0` (tables with mobile cards, shell rail, `BarraInferior`; a runtime no-op for Wanka, which does not use `TablaListado` or `Shell` from the package yet: `tokens.css` and `shadcn.css` are unchanged, `theme.css` only adds the `rail:` variant and the `--khipu-reserva-inferior` rules). Two real but harmless effects: `ScriptDiseno` now injects the rail script on every page (reads `localStorage["khipu-sidebar-rail"]`, writes `html[data-rail]`; no-op without the key) and the compiled CSS grows about 8.4 KB (+4.7 %) from the package's `@source "./components"`. `docs/frontend-migration-khipu.md` records the DS-0/A/B status, W1a, W1c and the new DS-H.
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
