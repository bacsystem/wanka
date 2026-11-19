# Changelog

All notable changes to this project are documented in this file.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- `apps/api`: `jobs.Enqueuer` and the `wanka worker` command on River v0.44.1 (the last release that supports Go 1.25). `InsertTx(ctx, tx, args, opts...)` inserts the job in the caller's `InTenantTx` transaction, so it exists only if that transaction commits; the exported API names no River type (a test checks it), so the in-house outbox of ADR 0006 can replace River. Queues `cpe` (20), `webhooks` (10), `notifications` (10), `reports` (2) and `maintenance` (2), tunable with `JOBS_CONCURRENCY_<QUEUE>`; `SIGTERM` waits for running jobs for up to `JOBS_SHUTDOWN_TIMEOUT` (30 s) and then cancels their context. A handler returns `nil`, an error (retried with exponential backoff, 10 s doubling up to 1 h, with jitter, 10 attempts, then `discarded`), `jobs.Cancel(err)` or `jobs.Snooze(d)`. `Job.InTenantTx` opens `InTenantTx(Actor{TenantID, Kind: system})` without a `UserID` (ADR 0001, «Workers»); a job with no tenant gets `db.ErrNoTenant` and is cancelled on its first attempt. Unique jobs (`WithUnique`), periodic jobs run once per cycle by the River leader, and the job cleaner keeps completed jobs 7 days, cancelled 7 and discarded 30. No handler holds a pool connection during a network call. The River tables live in the `ops` schema, created by the goose migration `20261119143700_kernel_river_jobs.sql` (the final schema of River migrations 1 to 7, checked against `rivermigrate` by a test); `wanka_app`, `wanka_webhooks` and `wanka_platform` only `INSERT` into `ops.river_job`, `wanka_worker` has all DML. New `rls-check` cases and six mutations cover the grants (#169).
- `apps/api`: `audit.events`, the append-only audit log, and its writer `audit.Writer` (`internal/platform/audit`). The migration `20261119160000_kernel_audit_events.sql` creates the table partitioned by month on `at` (UTC bounds), with `tenant_id NOT NULL`, `FORCE ROW LEVEL SECURITY` and the standard `tenant_isolation` policy; `wanka_app` and `wanka_worker` get only `SELECT` and `INSERT`, and nobody else gets anything. Per the architect, `CHECK (actor_type <> 'user' OR actor_user_id IS NOT NULL)` makes a `user` row without a user impossible, a `system` row never carries one (a job's origin goes in `action` or `diff`), a `platform_staff` row always has an `impersonator_id` and no company user, and only `platform_staff` rows have an `impersonator_id`. `Writer.Record(ctx, tx, Event{Action, EntityType, EntityID, Diff})` inserts in the caller's `InTenantTx` transaction, so the event commits or rolls back with the change, and a failed insert (for example no partition for the month) aborts the whole transaction instead of dropping the event. `Event` has no tenant field: tenant, user, impersonator and actor type come from the `app.*` variables of the transaction (a user session is `user`, an impersonated one `platform_staff` with `impersonator_id`, a job `system`), and `request_id` and `ip` from the context (`audit.WithClientIP`). `Writer.RecordDenied` writes a `denegado` event in its own transaction and returns the insert error so the middleware can log it and answer `403` anyway. Before saving, `diff` is masked by a list of forbidden field names (passwords, tokens, API keys, secrets, certificates, hashes, at any depth) and token-shaped values (`wk_at_`, `wk_rt_`, `fk_`), and capped at 64 KiB. The migration creates the current month and the next 3 partitions with `audit.ensure_partitions(date, int)`, idempotent and serialized between workers; the `wanka worker` command runs it at startup and as the daily periodic job `audit.ensure_partitions` in the `maintenance` queue. `audit.ensure_partitions` is owned by `wanka_migrator`, the bounded exception to ADR 0001 for partition DDL. `rls-check` has new cases and thirteen mutations (its platform functions now record the staff user as `impersonator_id`). A `Record` adds no measurable time to a write transaction (a benchmark ships with it) (#173).

### Changed
- `apps/api`: `TestCatalogRules` now also covers partitioned tables (`audit.events`) and skips their partitions, which grant nothing to the application roles (#173).
- `apps/api`: the `wanka worker` command no longer is a stub: with the database down it exits with code 1 instead of waiting for the signal. `go-arch-lint` confines River to `internal/platform/jobs` (and its pgx driver to `internal/platform/db`) (#169).
- `apps/api`: `db.Actor` comment no longer mentions a `systemUser` that never existed; ADR 0001 «Workers» already says a worker job runs as `Kind: system` without `UserID` (#169).

## [0.8.0] - 2026-11-19

### Added
- `apps/api`: migration `20261118140000_kernel_iam_org_core.sql` creates the minimum core of `iam` and `org` that the session lookup needs: `org.tenants` (`id`, `status`, `closed_at`), `iam.users` (`id`, `status`), `iam.roles`, `iam.memberships` (with `FORCE ROW LEVEL SECURITY` and `tenant_isolation`) and `iam.sessions` (no RLS, no foreign key to memberships, tenants, sites or cash registers; six named `CHECK`s, among them new ones: a session without a membership has no `authz_version`, an impersonated one has no refresh and lives at most 60 minutes, and `user_id` is `NULL` exactly when `impersonator_id` is set; `wanka_app` may `UPDATE` only some columns (never `id`, `user_id`, `impersonator_id`, `refresh_family` or `created_at`) and the trigger `iam.guard_session_writes` lets only `wanka_definer` or a superuser create an impersonated session or change it, except to revoke it, never lets `revoked_at` be undone in any session (revoking again is a no-op that keeps the old instant) and sets `created_at` from the server on `INSERT`; `iam.session_lookup` reads the membership by its primary key and treats it as absent unless it is the session user's), plus `iam.session_lookup(bytea)` owned by `wanka_definer` and the ADR 0001 grants for these tables (`wanka_worker` may `DELETE` sessions and read only `id`, `created_at`, `revoked_at`, `access_expires_at` and `refresh_expires_at`, never the hashes). Identity (#24) and companies (#31) extend them forward-only (#200, #195).
- `apps/api`: `dbtest.DB.SeedSession` seeds a tenant, user, role, membership and a bound, tenant-less, half-login, expired, revoked or impersonated session (the latter without `user_id`), and returns the `wk_at_` token and the ids; only the `sha256` is stored. Tests with the real `wanka_app` role cover the lookup (A1, A8), RLS (A2), unbinding (A9), the session `CHECK`s (A7), the grant matrix, the catalog rules and the index use with 100 000 sessions (#200, #195).

### Changed
- `docs/architecture/adr/0004-in-house-auth.md`: erratum in the text of `iam.session_lookup`: the membership is read only if `m.user_id = s.user_id`; a session crossed with another user's membership sees it as absent. The decision does not change (README §9) (#200, #195).
- `docs/architecture/data/rls-check`: `run.sh` now applies the Up section of the real kernel migrations from `apps/api/migrations` (like goose: the `Down` section is not run and `NO TRANSACTION` is honoured; a negative case checks it) instead of a copy of their tables and `iam.session_lookup`, so the check cannot drift from what is deployed; `02_migrate.sql` keeps only what identity, companies and platform still add. New cases (a new superuser step for the impersonated-session CHECKs) and fourteen new mutations (`drop_authz_check`, `drop_impersonated_check`, `drop_guard_trigger`, `drop_user_check`, `grant_app_update_all_sessions`, `lookup_without_user_check`, `guard_allows_unrevoke`, `guard_no_server_created_at`, `guard_rerevoke_raises`, `lookup_not_distinct`, `revoke_worker_session_columns`, `drop_force_rls`, `revoke_app_sessions_insert`, `grant_app_update_tenants`); `scripts/rls-check.sh` checks every mutation against the step that must catch it (#200, #195).

## [0.7.0] - 2026-11-19

### Added
- `apps/api`: session middleware and the `auth.SessionResolver` port (`internal/platform/auth`), tested with a double and no PostgreSQL. Modules register `/v1` routes with `auth.Routes.Handle(pattern, class, h)`, where `auth.RouteClass` is `Public`, `NoTenant` or `Tenant` (no valid zero value); startup panics when a class is missing or breaks the rules of `api/conventions.md` §2. A request goes through: token check without touching the database (one `Authorization` header, `Bearer`, `wk_at_` plus exactly 43 characters of `[A-Za-z0-9_-]` checked one by one before a strict base64url decode; the hash is `sha256` of the whole token, and neither is ever logged), lookup with a 1 s deadline in phase `session_lookup`, `MFASatisfied` and row coherence checks, unbinding of a session whose membership was lost without revoking it (ADR 0011, counter `auth_session_detach_total{outcome}`), `403 tenant_access_denied` on a tenant route without a company, the per-session limiter (10/s, burst 20, 50 000 keys) and, on tenant routes only, the per-tenant limiter (100/s, burst 200, 10 000 keys), with `Routes.Sweep()` for full buckets. No row or an unfinished 2FA is `401 session_expired`; a database failure or the lookup deadline is `503`, never `401`; an untyped error or an inconsistent row is `500`. An impersonated session arrives with an empty `UserID` and its `Actor.UserID` is the staff `ImpersonatorID`. Every kernel `401` carries `WWW-Authenticate: Bearer` (#199, #174).
- `apps/api`: `httpx.Logger`, `httpx.Metrics`, `httpx.Route` and `httpx.LogDBFailure` for kernel packages that write their own failures (#199, #174).

## [0.6.0] - 2026-11-19

### Added
- `apps/api`: `db.Pool.InNoTenantTx` and `InNoTenantReadTx` (`BEGIN READ ONLY`) with `db.NoTenantScope{UserID}`: a transaction without tenant, the door for every database access with no tenant set, from any service (session lookup and unlink, login, `iam.list_my_memberships()` and impersonation redemption in `api`; webhook route resolution; `platform_fn.*`; session purge in `worker`). Its first statement is one `set_config` that clears `app.tenant_id`, `app.session_kind` and `app.impersonator_id` and sets `app.user_id` to the scope's user or `''`, so a session-level value left on the pooled connection is never inherited (a `SECURITY DEFINER` function that saves and restores `app.tenant_id`, like `iam.session_lookup`, restores `''`). Same four round trips and the same typed errors as `InTenantTx` (shared transaction cycle `runTx`); `set_config(..., false)` stays forbidden in both doors. Overhead recorded in `docs/architecture/capacity.md` (#198, #194).

## [0.5.1] - 2026-11-18

### Changed
- Kernel story #174 split into #174 (session middleware with a test double), #194 (`InNoTenantTx`), #195 (`iam`/`org` tables and `iam.session_lookup`) and #196 (PostgreSQL resolver and wiring). API contract: access tokens are a prefix plus 43 base64url characters and are stored as the `sha256` of the full token; every route declares its class (`Public`, `NoTenant`, `Tenant`); every kernel `401` carries `WWW-Authenticate: Bearer`; default limiter values per session and per tenant (`api/conventions.md` §2, §3, §4 and §7, sequence 08, `data-model.md` §6).
- ADR 0011 status set to accepted (owner decision on PR #185).
- TOTP enrollment split into `mfa/totp/enroll` (`NoTenant`, from the profile) and the new `mfa/enrollment/totp` (`Public`, login, token in the body): one route, one class. Login step tokens fixed: `challengeToken` (`wk_ch_`, 5 min, consumed on the first good `verify`) and `enrollmentToken` (`wk_en_`, 10 min, valid until the first good `confirm`), 32 random bytes in base64url, stored as `sha256`.
- Impersonated sessions locked to the database: no refresh and at most 60 min (CHECK), `user_id` is NULL with the staff in `impersonator_id`, a trigger reserves inserting or changing them to `wanka_definer` (only setting `revoked_at` is allowed), revocation goes through wave-6 definer functions. `wanka_app` gets column-level `UPDATE` on `iam.sessions` (no `created_at`, `user_id`, `impersonator_id`); `wanka_worker` column-level `SELECT` without the hashes (ADR 0001 matrix, `data-model.md` §6, er-01).

## [0.5.0] - 2026-11-18

### Added
- `apps/api`: HTTP base of the kernel in `internal/platform/httpx`. Every error is RFC 9457 `application/problem+json` with `type`, `title`, `status`, `code` and `requestId`; the HTTP of each `code` comes from one table that a test compares with the 31 codes of `api/conventions.md` §4. `httpx.Router` answers `404 not_found` for undeclared routes and methods (including `OPTIONS`; no `405`, no CORS); a handler that exceeds its 10 s budget answers `503 overloaded` with `Retry-After: 1` after cancelling its context (the transaction rolls back and the connection returns to the pool) and discarding its buffered output; `httpx.Streaming()` routes skip that budget and get a 25 s context from arrival. `httpx.NewServer` disables net/http's own `OPTIONS *` answer and `Router.Handle` rejects subtree patterns (`/x/`, `{p...}`) so no response comes from `ServeMux`. `Content-Length` over 1 MB is `413 payload_too_large` without reading the body, `httpx.DecodeJSON` answers `400 malformed_request`, and a `panic` is `500 internal` without details (#193, #165).
- `apps/api`: in-memory token-bucket rate limiter (implemented as GCRA, exact integer arithmetic) with an injectable key and clock (`httpx.NewLimiter`), `RateLimit-Limit`/`-Remaining`/`-Reset` headers, no token spent by a rejected request across chained limiters, a cap on keys (`503 overloaded`) and `Sweep` of full buckets. Wiring it to session and tenant keys stays in #174 (#193, #165).
- `apps/api`: `internal/platform/clock` (system and fake clocks) and `internal/platform/metrics` (metrics port; the OpenTelemetry adapter comes later); `db.Pool.AcquiredConns` (#193, #165).

### Fixed
- `apps/api`: a lost database connection (SQLSTATE `57P01`-`57P03` or class `08`, or a connection pgx reports closed; never the shape of the error, so a foreign `io.EOF`, a `url.Error` or `conn busy` stay `500`) with the caller's context alive is now `db.ErrUnavailable` and answers `503 dependency_unavailable` instead of `500` when a pool connection dies after a restart or failover; a streaming handler that writes its own RFC 9457 problem after the budget expired now delivers it; the access log marks a connection cut with `http.ErrAbortHandler` as `aborted` (status 0 if nothing was sent) instead of `200`; `httpx.NewServer` sets `MaxHeaderBytes` to 64 KiB; a connection lost during `COMMIT` also carries `db.ErrCommitUncertain` (log `phase=commit`) and every database outage logs one `warn` line and counts `db_unavailable_total` (#193, #165).

### Changed
- `docs/architecture/api/conventions.md` §4: requests that `net/http` rejects before the API chain (invalid escape in the path, garbage request line, bare LF in a header or missing `Host`: `400`; oversized headers: `431`; unsupported `Expect`: `417`; unsupported `Transfer-Encoding`: `501`; unsupported HTTP version: `505`) are answered by Go's server as `text/plain`, a documented exception to the RFC 9457 rule (#193, #165).
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
