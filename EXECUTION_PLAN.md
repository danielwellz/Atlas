# Atlas MSV Execution Plan

This plan goes with `GAP_ANALYSIS.md`, dated 2026-10-06 (same date as HEAD `2f8206d`). Every task below is a self-contained prompt for a fresh agent session that has no memory of the analysis.

**How to use it:** copy one task section verbatim, from its `### Txx` heading to the next `---`, and paste it as the agent's prompt. Run one task per session and merge each into `main` before starting a task that depends on it. Tasks marked "parallel-safe" touch disjoint areas and can run on separate branches.

**Rules every task inherits (already repeated inside each prompt):**
- Don't commit secrets.
- Any OpenAPI change must regenerate the Go and TS code and pass `make verify-generated`.
- Integration tests truncate every table, so always point `ATLAS_TEST_POSTGRES_URL` at a throwaway database.

## Order, size, and budget

| # | Task | Size | Gap IDs | Depends on | MSV? |
|---|---|---|---|---|---|
| T01 | Make the API boot and get CI green | M | G01 G02 G03 G33 | — | BLOCKER |
| T02 | Toolchain and dependency security bumps; CI hygiene; Android build in CI | M | G19 G28 G34 | T01 | IMPORTANT |
| T03 | DB transactions for multi-write handlers | M | G13 | T01 | IMPORTANT |
| T04 | Single-source auth policy, config cleanup, billing kill switch | M | G15 G16 | T01 | IMPORTANT |
| T05 | API hardening: rate limits, body limits, timeouts, bounded caches | M | G17 G18 | T01 | IMPORTANT |
| T06 | Account deletion and data export API | M | G07 | T01 (T03 if merged) | BLOCKER |
| T07 | Production reference data and recipe scoping | M | G10 G24 G33 | T01 | BLOCKER |
| T08 | Infra: base/overlays, migration job, Dockerfile | M | G11 | T01 | BLOCKER |
| T09 | Mobile environment config (API URL per build) | M | G05 | — (parallel-safe) | BLOCKER |
| T10 | Mobile token refresh | M | G04 | T09 | BLOCKER |
| T11 | Offline outbox robustness | M | G14 | T10 | IMPORTANT |
| T12 | MSV navigation, feature flags, Settings screen, mock cleanup | M | G23 G31 | T09 | IMPORTANT |
| T13 | Habit creation and nutrition-targets UI | M | G20 G21 | T12 | IMPORTANT |
| T14 | Privacy UI: delete account and export data | S | G07 | T06, T12 | BLOCKER |
| T15 | Real store receipt verification (only if the paywall ships in 1.0) | L | G06 G29 | T04 | BLOCKER if paywall |
| T16 | Crash reporting and error boundary | S | G22 | T12 | IMPORTANT |
| T17 | Android release build: IDs, signing, icons, CI artifact | M | G08 G12 | T09, T12 | BLOCKER |
| T18 | iOS release config and CI build | M | G09 G12 | T09, T12 | BLOCKER if iOS |
| T19 | Infra real values: hosts, TLS, OTel backend, staging deploy | S | G11 G27 | T08 + human values | BLOCKER |
| T20 | Password reset by emailed code (post-MSV) | M | G25 | T04 | IMPORTANT |
| T21 | Store server notifications (post-MSV) | M | G26 | T15 | IMPORTANT |
| T22 | React Native upgrade (post-MSV, or earlier if store rules require it) | L | G35 | all mobile tasks | DEFERRED |

**Budget estimate** (rough; it depends on model, plan, and how much context each session loads; I can't see your billing):

| Size | Cost per session | Tasks | Subtotal |
|---|---|---|---|
| S | ~$2–4 | T14, T16, T19 | ~$10 |
| M | ~$4–8 | the other 15 MSV tasks | ~$90 |
| L | ~$10–15 | T15 | ~$12 |

- All MSV tasks including the paywall: **≈ $85–130**.
- With the paywall deferred to 1.1 (no T15): **≈ $75–115**.
- To stay under $100: defer T15, fold T16 into T12, and run T11 only if offline logging matters to you at launch.
- A task that runs over should stop at a green, committed checkpoint and leave a note in its PR, not start a second large change.

**Your parts (in order):** see `GAP_ANALYSIS.md` §10. T15, T17, T18, and T19 can't be finished without your accounts and keys. They're written so the agent finishes everything else and leaves clear "TODO(owner)" inputs.

---

### T01: Make the API boot and get CI green

**Goal:** The Go API starts successfully, a fresh database migrates cleanly, and `go test ./...` passes deterministically on any weekday, in CI and locally.

**Context:**
- Repo: Atlas monorepo (git root). The API lives in `atlas-api/`: Go module `github.com/atlas/atlas-api`, chi with an oapi-codegen strict server, sqlc with pgx, and goose migrations in `atlas-api/migrations/`. CI is `.github/workflows/ci.yml` (job "API Tests (Go)" runs `make api-migrate-up`, then `go test -v ./...`).
- Local DB: `make dev-up` uses `infra/local/docker-compose.yml`. If the MinIO image fails to pull, run `docker compose -f infra/local/docker-compose.yml up -d postgres`. If something else is listening on localhost:5432, pass `POSTGRES_URL=postgres://atlas:atlas@<host>:<port>/atlas?sslmode=disable` to make targets.
- Known failures, as of the 2026-10-06 audit:
  1. **Startup crash.** The binary exits with `failed initializing OpenTelemetry {"error": "conflicting Schema URL: https://opentelemetry.io/schemas/1.39.0 and https://opentelemetry.io/schemas/1.26.0"}`. `atlas-api/internal/observability/otel.go` imports `go.opentelemetry.io/otel/semconv/v1.26.0` and merges it with `resource.Default()` from otel SDK v1.40.0 (schema 1.39.0). The audit confirmed that switching to `semconv/v1.39.0` and renaming `semconv.DeploymentEnvironment` to `semconv.DeploymentEnvironmentName` fixes it. No test calls `InitOTel`.
  2. **Migrations.** `goose up` fails with "unterminated dollar-quoted string" because `DO $$ … $$;` blocks lack `-- +goose StatementBegin` / `-- +goose StatementEnd` in:
     - `atlas-api/migrations/20260227070000_exercise_substitution_metadata.sql`
     - `20260228040000_exercise_substitution_taxonomy.sql`
     - `20260228080000_monetization_subscriptions_v1.sql` (its block ends as `END` on one line and `$$;` on the next)

     `20260227030000_habit_system_mvp.sql` already does this correctly and is a good reference. GitHub CI run 28324207541 failed here.
  3. **Flaky test.** `TestDashboardSummaryIntegration` (`atlas-api/internal/httpapi/router_integration_test.go`, around line 1522) seeds workouts at `now-2d` and `now-1d` via `seedAnalyticsWorkoutData`. The weekly volume query uses `DATE_TRUNC('week', …)` (`atlas-api/internal/db/queries/analytics.sql:149`), so on Mondays and Tuesdays the fixtures land in different weeks and the assertion `WeeklyVolumeTrend[0] == 1260` fails.
  4. **Tests wipe the dev DB.** The integration helper falls back to `postgres://atlas:atlas@localhost:5432/atlas` when `ATLAS_TEST_POSTGRES_URL` is unset (around line 2695), then `TRUNCATE`s every table, including migration-seeded `muscle_groups` (around line 2729). That silently wipes the developer DB and breaks `make api-seed`.
  5. **gofmt.** `gofmt -l` flags `internal/entitlement/service.go`, `internal/httpapi/billing_handlers.go`, and `internal/httpapi/router_integration_test.go`.

**Steps:**
1. Add the goose StatementBegin/End markers around every `DO $$` block in the three migrations. Don't change any SQL inside the blocks. Then grep all migrations for other `$$` bodies that lack markers.
2. Fix `otel.go` so the semconv import matches the SDK's schema version (or build the custom resource with `resource.NewSchemaless`). Add `internal/observability/otel_test.go`: call `InitOTel` with no OTLP env vars set, assert no error, and call the returned shutdown.
3. Make `TestDashboardSummaryIntegration` deterministic on every weekday. Choose fixture timestamps so the "recent" workouts fall in both the current `DATE_TRUNC('week')` bucket and the last 7 days (for example, both earlier today at different hours), and adjust the expected values only if the semantics really change. If the handler reads `s.currentTime`, you may inject a fixed clock instead. Explain the choice in a code comment.
4. Make integration tests use only `ATLAS_TEST_POSTGRES_URL`: skip with a clear message if it's unset, with no fallback to the dev URL. Keep the CI guard that fails on "skipping integration test". Document this in `atlas-api/README.md`.
5. Run `gofmt -w` on the three files.
6. In `.github/workflows/ci.yml`, job `api-tests`: after the tests, add a step that builds `./cmd/atlas-api`, starts it in the background with `APP_ENV=local` and the CI `POSTGRES_URL`, waits up to 15 s for `curl -fsS http://127.0.0.1:8080/api/v1/health`, and fails the job otherwise.

**Acceptance criteria:**
- On an empty Postgres 16: `make api-migrate-up` succeeds. A goose `reset` followed by `up` also succeeds: `cd atlas-api && go run -tags "no_clickhouse no_libsql no_mssql no_mysql no_sqlite3 no_vertica no_ydb" github.com/pressly/goose/v3/cmd/goose@v3.24.1 -dir migrations postgres "$POSTGRES_URL" reset`, then `up`.
- `cd atlas-api && go build ./... && go vet ./...` pass, and `gofmt -l . | grep -v generated` prints nothing.
- `cd atlas-api && ATLAS_TEST_POSTGRES_URL=<throwaway db url> go test ./...` passes with no skipped integration tests.
- `go test ./...` without `ATLAS_TEST_POSTGRES_URL` skips the integration tests and does not touch any database.
- Running the built binary with `APP_ENV=local` and a migrated DB answers `GET /api/v1/health` with 200 and `"status":"ok"`.
- `make generate && make verify-generated` still pass (no generated drift).

**Out of scope:** Dependency upgrades (T02), behaviour changes to handlers, OpenAPI changes, mobile code, infra manifests.

---

### T02: Toolchain and dependency security bumps; CI hygiene; Android build in CI

**Goal:** Move the API to a supported Go toolchain with no reachable known vulnerabilities in third-party modules, modernise CI, make local dev-up reliable, and prove the Android debug build compiles in CI.

**Context:**
- Repo: Atlas monorepo. API in `atlas-api/`; mobile in `atlas-mobile/` (React Native 0.84 CLI app, Node ≥22.11, Gradle 9.0.0 wrapper, compile/target SDK 36, NDK 27.1.12297006 per `atlas-mobile/android/build.gradle`).
- Current pins: `atlas-api/go.mod` `go 1.24.0`; `.github/workflows/ci.yml` `go-version: '1.24.x'`; `atlas-api/Dockerfile` `golang:1.24-alpine` with `GOARCH=amd64` hardcoded. Go 1.24 is out of support now that 1.26 is out.
- govulncheck (2026-10-06) reported 34 reachable vulns:
  - `github.com/golang-jwt/jwt/v5 v5.2.1` (latest 5.3.1)
  - `github.com/jackc/pgx/v5 v5.7.1` (5.11.0)
  - `github.com/go-chi/chi/v5 v5.2.3` (5.3.2)
  - all `go.opentelemetry.io/otel*` at v1.40.0 (fixed in 1.45.0)
  - `google.golang.org/grpc v1.78.0` (fixed in 1.83.1)
  - `golang.org/x/net v0.49.0`, `golang.org/x/text v0.33.0`
  - stdlib (fixed in go1.26.6)
- Keep `OAPI_CODEGEN_VERSION`, `SQLC_VERSION`, and `OPENAPI_TYPESCRIPT_VERSION` in the root `Makefile` unchanged so generated code doesn't churn.
- `atlas-api/internal/observability/otel.go` imports a `semconv/vX` package whose schema version must match the otel SDK's `resource.Default()`. After bumping otel, update that import, or the API exits at startup with "conflicting Schema URL". `internal/observability/otel_test.go` (added in T01) catches this.
- CI annotations: "Restore cache failed: Dependencies file is not found … go.sum" (setup-go needs `cache-dependency-path: atlas-api/go.sum`) and "Node.js 20 is deprecated" for `actions/checkout@v4`, `actions/setup-go@v5`, `actions/setup-node@v4`.
- `infra/local/docker-compose.yml` problems:
  - `minio/minio:RELEASE.2024-07-16T23-46-41Z` couldn't be pulled during the audit (it went through a registry mirror, so whether Docker Hub still serves it is unverified).
  - `mailhog/mailhog:v1.0.1` is amd64-only.
  - The API uses neither Redis nor MailHog, and MinIO is only used when `ASSET_STORAGE_BACKEND=s3|minio`.

**Steps:**
1. Set `go 1.26` (and a `toolchain` line if useful) in `atlas-api/go.mod`. Run `go get -u` on the vulnerable modules listed above, then `go mod tidy`. Fix any compile errors; for otel, see the semconv note.
2. Run `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` in `atlas-api` until it reports no reachable vulnerabilities in non-stdlib modules. List any residual stdlib findings and the Go patch version that fixes them in the PR.
3. CI:
   - Go `1.26.x` everywhere, with `cache-dependency-path: atlas-api/go.sum` on every `setup-go`.
   - Bump `actions/*` and `docker/*` actions to current majors that run on Node 24 (check each action's README or releases).
   - Add a new job `android-debug-build` (ubuntu, JDK 17 via `actions/setup-java`, Node 22, `npm ci` in `atlas-mobile`, then `cd atlas-mobile/android && ./gradlew assembleDebug --no-daemon`), with Gradle caching.
4. Dockerfile: `golang:1.26-alpine` builder; use `ARG TARGETARCH` with `GOARCH=$TARGETARCH` instead of the hardcoded amd64.
5. `infra/local/docker-compose.yml`:
   - Keep Postgres as the default service.
   - Move `minio`, `mailhog`, and `redis` under `profiles: ["optional"]` so `make dev-up` starts only what the API needs.
   - Pin MinIO to a tag you've verified exists (`docker manifest inspect <image>`), or switch MailHog to a multi-arch equivalent such as `axllent/mailpit`.
   - Update `docs/dev-onboarding.md` and the README to match.
6. Add `atlas-mobile/.nvmrc` containing `22`.

**Acceptance criteria:**
- `cd atlas-api && go build ./... && go vet ./... && ATLAS_TEST_POSTGRES_URL=<throwaway db> go test ./...` pass.
- `govulncheck ./...` shows no reachable third-party module vulns.
- `make generate && make verify-generated` pass with no diff.
- `docker build -t atlas-api:test atlas-api` succeeds, and the image boots against a migrated DB with `APP_ENV=local` and returns 200 on `/api/v1/health`.
- `make dev-up` brings up Postgres on a clean machine.
- CI on the PR: all jobs green, including the new `android-debug-build`. If `assembleDebug` fails for a reason that needs more than a small config fix, keep the job, mark it `continue-on-error: true`, and document the exact error in the PR description for a follow-up.

**Out of scope:** React Native or npm dependency upgrades (T22), handler logic, OpenAPI changes, the oapi-codegen/sqlc/openapi-typescript versions, release signing.

---

### T03: DB transactions for multi-write handlers

**Goal:** Every API handler that performs more than one write does so atomically in a single Postgres transaction, and the crew-invite join can no longer over-use an invite.

**Context:**
- Repo: Atlas monorepo; API in `atlas-api/`, Go, chi, oapi-codegen strict server. The `Server` struct lives in `atlas-api/internal/httpapi/server.go`.
- `NewServer` receives a `db.Querier` built by `db.New(database)` (sqlc, package `atlas-api/internal/db/sqlc`) where `database` is a `*sql.DB` (`atlas-api/cmd/atlas-api/main.go`). `db_ext.go` exposes `DBTX()`, and sqlc generated `(*Queries).WithTx(*sql.Tx)`. Some handlers also run raw SQL through `s.queryDB`.
- There are **no transactions anywhere today.** Multi-write handlers:
  - `PostCrews` (`internal/httpapi/community_handlers.go`: CreateCrew + AddCrewMember)
  - `PostCrewsJoin` (AddCrewMember *before* the guarded `IncrementCrewInviteUse`, so an exhausted invite still adds the member)
  - `PostWorkoutsStart` (`workout_handlers.go`: CreateWorkout + CreateWorkoutExercisesFromProgramSession)
  - `PostMomentumSprintEnroll` (`momentum_sprint_handlers.go`: upsert + 2 deletes)
  - `PostNutritionWeeklyCheckin` (UpsertNutritionTargets + UpsertWeeklyCheckin)
  - `PostNutritionMealPlanGenerate` and `PutNutritionMealPlan` (`nutrition_planning_handlers.go`: UpsertMealPlan + 2 deletes + N inserts)
  - `PostHabitsIdToggleToday`
  - `PostAuthRefresh` (revoke old session + create new one; in `server.go`)

  Check for others with `grep -n "s.queries\.\(Create\|Upsert\|Update\|Delete\|Add\|Increment\|Revoke\)"`.
- Integration tests live in `atlas-api/internal/httpapi/router_integration_test.go` and need `ATLAS_TEST_POSTGRES_URL` pointing at a **throwaway** DB, because they truncate all tables.

**Steps:**
1. Add a small transaction helper, for example `func (s *Server) inTx(ctx context.Context, fn func(q db.Querier, exec db.DBTX) error) error`, that begins a `*sql.Tx`, passes `queries.WithTx(tx)` (and the tx as the raw executor), and commits or rolls back. Wire the `*sql.DB` into `Server` in whatever way fits the code: an extra constructor parameter, or a type assertion on `DBTX()` to `interface{ BeginTx(...) }`. Unit tests that use fake Queriers must keep working; when no `*sql.DB` is available, the helper may fall back to non-transactional execution.
2. Wrap each handler listed above, keeping the request validation and response building outside the transaction.
3. `PostCrewsJoin`: inside the transaction, call `IncrementCrewInviteUse` first, return the existing "invite exhausted/invalid" response if it affects no row, then add the member. Handle "already a member" without consuming a use if that's what the current semantics intend (read the handler and tests).
4. Add integration tests:
   - A meal-plan PUT whose item insert fails mid-way (for example an invalid recipe reference) leaves the previous plan intact.
   - Two concurrent joins on a `max_uses=1` invite produce exactly one new member.
   - A refresh-token race: two concurrent refreshes of the same token yield one success and one 401.

**Acceptance criteria:**
- `cd atlas-api && go build ./... && go vet ./...` pass.
- `ATLAS_TEST_POSTGRES_URL=<throwaway db> go test ./...` passes, including the new tests.
- `grep -rn "BeginTx" atlas-api/internal/httpapi` shows the helper, and every handler listed above uses it.
- `make verify-generated` passes (no OpenAPI or sqlc changes expected; if you add a SQL query, run `make generate`, `git add` the generated files, then `make verify-generated`).

**Out of scope:** New endpoints, OpenAPI changes, rate limiting, mobile code, schema redesign.

---

### T04: Single-source auth policy, config cleanup, billing kill switch

**Goal:** Public versus authenticated routing is derived from, and tested against, the OpenAPI spec; dead or misleading config is removed; and `/billing/verify` can't grant entitlements unless explicitly enabled.

**Context:**
- Repo: Atlas monorepo; API in `atlas-api/`.
- Auth enforcement currently has three independent layers:
  1. The public path allowlist in `atlas-api/internal/httpapi/router.go` (`publicPathAllowlist`, including the wildcard `/api/v1/exercises/*`, which also makes the bearer-protected `GET /api/v1/exercises/{id}/biomechanics` public at the router level).
  2. Per-operation `security: [bearerAuth]` in `atlas-api/openapi/openapi.yaml`. The spec has no global security; public ops simply omit `security`.
  3. A hand-written operationId switch in `atlas-api/internal/httpapi/entitlement_middleware.go`.
- The middleware is `atlas-api/internal/httpapi/middleware/auth.go` (`pathRequiresAuth`).
- Config (`atlas-api/internal/config/config.go`):
  - `REDIS_ADDR` (required in staging/prod) and `MAILHOG_SMTP_ADDR` are read but never used.
  - `PRO_ALL_USERS` and `PRO_USER_EMAILS` are read but never used.
  - `.env` is auto-loaded whenever `APP_ENV` is unset or `local` (`loadDotenvIfNeeded`).
  - There's no minimum length on `JWT_SECRET`.
  - These variables also appear in `atlas-api/.env.example`, `infra/staging/k8s/atlas-api-staging-secrets.example.yaml`, and `infra/staging/docker-compose.yml`.
- `atlas-api/README.md` wrongly says `APP_ENV=development` loads `.env` (only local, staging, and prod are valid) and lists only 30 of the 65 endpoints.
- `POST /api/v1/billing/verify` (`atlas-api/internal/httpapi/billing_handlers.go`) trusts client receipts and the client-supplied `expiresAt`. Its spec responses are 200, 400, 401, and 500.
- Codegen: after editing `openapi.yaml`, run `make generate` (needs Docker for sqlc and `cd atlas-mobile && npm ci` under Node ≥22.11 for openapi-typescript), `git add` the generated files (`atlas-api/internal/httpapi/generated/openapi.gen.go`, `atlas-mobile/src/api/generated/openapi.ts`), then run `make verify-generated`, which must print nothing.

**Steps:**
1. Replace wildcard matching with explicit path templates, for example `/api/v1/exercises/{id}` and `/api/v1/exercises/{id}/substitutes`, using a simple segment matcher in `middleware/auth.go`.
2. Add `internal/httpapi/auth_policy_test.go`. It parses `openapi/openapi.yaml` (`gopkg.in/yaml.v3` is already in `go.sum`) and asserts:
   - (a) every operation without `security` is allowlisted;
   - (b) no operation with `security` is reachable without a token;
   - (c) every operationId in the entitlement switch exists in the spec and declares 401 and 403 responses.
3. Remove `RedisAddr`, `MailhogSMTP`, `ProAllUsers`, and `ProUserEmails` from config, tests, `.env.example`, the staging secret example, and the staging compose file.
4. Only auto-load `.env` when `LOAD_DOTENV=true` or when `APP_ENV` is explicitly `local`. If `APP_ENV` is unset, log a startup warning that local defaults are in use.
5. Reject a `JWT_SECRET` shorter than 32 bytes outside local.
6. Add `BILLING_VERIFY_ENABLED` (default `true` for local, `false` otherwise). When it's false, `PostBillingVerify` returns a new `403` response ("billing verification is not enabled"). Add `403` to the operation in `openapi.yaml`, regenerate, and stage the generated files.
7. Fix `atlas-api/README.md`: correct the env loading description, Go version, and test DB guidance, and replace the endpoint list with a pointer to `openapi/openapi.yaml`.

**Acceptance criteria:**
- `cd atlas-api && go build ./... && go vet ./... && ATLAS_TEST_POSTGRES_URL=<throwaway db> go test ./...` pass, including `auth_policy_test.go`.
- `make generate && git add -A atlas-api/internal/httpapi/generated atlas-mobile/src/api/generated && make verify-generated` exits 0.
- `cd atlas-mobile && npm run typecheck && npm test -- --watchAll=false` still pass.
- With `APP_ENV=staging` and no `REDIS_ADDR`, `go run ./cmd/config validate` succeeds when every other required variable is set.
- `curl` without a token to `/api/v1/exercises/<uuid>/biomechanics` returns 401 from the auth middleware, and the response body says "missing or invalid authorization header".

**Out of scope:** Real receipt verification (T15), rate limiting (T05), mobile changes beyond regenerated types, moving entitlements into OpenAPI vendor extensions.

---

### T05: API hardening: rate limits, body limits, timeouts, bounded caches

**Goal:** The API resists basic abuse and memory growth: per-IP rate limits on auth and public endpoints, request body size limits, server timeouts, DB pool limits, and size-bounded in-memory caches.

**Context:**
- Repo: Atlas monorepo; API in `atlas-api/`.
- Router: `atlas-api/internal/httpapi/router.go` (chi middleware: RequestID, RealIP, Recoverer, OTel, logger, Authenticator).
- Server setup: `atlas-api/cmd/atlas-api/main.go` sets only `ReadHeaderTimeout: 5s` and no DB pool settings.
- Public endpoints include `POST /api/v1/auth/register`, `/auth/login`, `/auth/refresh`, and anonymous `POST /api/v1/events`.
- Caches are plain maps, with entries removed only when an expired key is read again:
  - `atlas-api/internal/food/service.go` (`searchCache`, `upcCache`)
  - `atlas-api/internal/food/usda_provider.go` (`detailsByID`)
- Config lives in `atlas-api/internal/config/config.go`, with defaults documented in `atlas-api/.env.example`. Redis is not used and must not be introduced.

**Steps:**
1. Add per-IP rate limiting with an in-memory limiter (for example `github.com/go-chi/httprate`). Make the limits configurable via env, with defaults of 10/min for `/api/v1/auth/*`, 60/min for `/api/v1/events`, and 600/min globally. Return 429 with the same JSON error shape as other errors (`{"message": …}`).
2. Wrap request bodies with `http.MaxBytesReader`. Default to 1 MiB; make it configurable; check `form-check/uploads` and `nutrition/recipes/import` payload sizes in `openapi.yaml` and raise their limits only if needed.
3. Set `ReadTimeout`, `WriteTimeout`, and `IdleTimeout` on `http.Server` (defaults 15 s / 30 s / 60 s), plus `SetMaxOpenConns`, `SetMaxIdleConns`, and `SetConnMaxLifetime` (env-configurable).
4. Replace the three cache maps with size-bounded TTL caches (for example `github.com/hashicorp/golang-lru/v2/expirable`) with configurable caps: search 1000, UPC 5000, USDA details 5000. Preserve the existing TTL semantics and tests.
5. Document every new variable in `atlas-api/.env.example` and the staging secret example.

**Acceptance criteria:**
- `cd atlas-api && go build ./... && go vet ./... && ATLAS_TEST_POSTGRES_URL=<throwaway db> go test ./...` pass.
- New unit tests show that the 11th login attempt within a minute from one IP gets 429; that a 2 MiB body to a JSON endpoint gets 413 (or 400 with a clear message); and that the caches never exceed their caps (insert cap+100 keys and assert the length).
- `make verify-generated` passes.

**Out of scope:** Distributed rate limiting, Redis, auth policy (T04), receipt verification, mobile code.

---

### T06: Account deletion and data export API

**Goal:** Authenticated users can permanently delete their account and download all their data through new API endpoints, as the App Store (guideline 5.1.1(v)) and Google Play account-deletion policy require.

**Context:**
- Repo: Atlas monorepo; API in `atlas-api/` (OpenAPI-first: spec `atlas-api/openapi/openapi.yaml`, strict server, sqlc queries in `atlas-api/internal/db/queries/*.sql` that generate `atlas-api/internal/db/sqlc/`).
- `docs/compliance.md` §3 lists this as unchecked.
- In `atlas-api/migrations/`, all 23 foreign keys to `users(id)` are `ON DELETE CASCADE` except one, `ON DELETE SET NULL` (analytics `app_events`). That includes `crews.created_by_user_id`, so deleting a crew owner deletes the crew and every member's membership. Decide and document the behaviour; the simplest acceptable option is to keep the cascade and say so in the response docs.
- Passwords are bcrypt (`golang.org/x/crypto/bcrypt`, see `PostAuthLogin` in `atlas-api/internal/httpapi/server.go`). Sessions are in `sessions.sql`.
- Form-check uploads may reference stored objects through `atlas-api/internal/storage`.
- If the T03 transaction helper (`inTx` or similar in `server.go`) exists, use it. Otherwise perform the deletion as a single `DELETE FROM users WHERE id=$1` statement, which is atomic with the cascades.
- Codegen: edit the spec, run `make generate` (needs Docker and `cd atlas-mobile && npm ci` under Node ≥22.11), `git add` the generated files, then `make verify-generated`, which must exit 0.

**Steps:**
1. Spec:
   - `DELETE /api/v1/me`, operationId `DeleteMe`, `bearerAuth`. Body `{ "password": string }`. Responses: 204, 400, 401 (wrong password or token), 500.
   - `GET /api/v1/me/export`, operationId `GetMeExport`, `bearerAuth`. Response 200 with a JSON object holding one array or object per domain: profile, goals, consents, sessions metadata without token hashes, workouts with exercises and sets, habits with logs, nutrition targets, check-ins, weights, food logs, meal plans, momentum sprint, crew memberships, subscriptions, form-check upload metadata, and the user's analytics events. Also 401 and 500.
2. Regenerate the Go and TS code.
3. Implement `DeleteMe`: verify the password, delete storage objects for the user's form-check uploads (best effort; log failures without PII), delete the user row (cascades revoke sessions), and return 204.
4. Implement `GetMeExport` with read-only sqlc queries per table (add them to `internal/db/queries/`, then `make generate`). Never include password hashes or refresh-token hashes.
5. Integration tests:
   - Register, create data in several domains, export, and assert the keys are present.
   - Delete with the wrong password returns 401.
   - Delete with the right password returns 204, after which login returns 401, the old refresh token returns 401, and a sampling of tables has no rows for that user id.
6. Update `docs/compliance.md` §3 checkboxes.

**Acceptance criteria:**
- `cd atlas-api && go build ./... && go vet ./... && ATLAS_TEST_POSTGRES_URL=<throwaway db> go test ./...` pass.
- `make generate && git add -A atlas-api/internal atlas-mobile/src/api/generated && make verify-generated` exits 0.
- `cd atlas-mobile && npm run typecheck` passes with the regenerated types.

**Out of scope:** Mobile UI (T14), async export jobs or email delivery, soft-delete or legal hold, admin tooling.

---

### T07: Production reference data and recipe scoping

**Goal:** A fresh production database can be populated with the program, exercise, and recipe catalog by an idempotent command that never creates demo users, and user-imported recipes become private to their importer.

**Context:**
- Repo: Atlas monorepo; API in `atlas-api/`.
- Today all catalog content lives in `atlas-api/seeds/001_local_seed.sql`: 2 programs, their weeks, 6 sessions with exercises, 6 recipes, biomech assets. The same file also inserts demo user `demo@atlas.local` with a known password hash, plus that user's profile, goals, and consents. It's applied with `make api-seed` (plain `psql -f`, with no `ON_ERROR_STOP`).
- Exercises are imported by `go run ./cmd/seed exercises --file ./seed/exercises.csv` (`atlas-api/cmd/seed/main.go`, `atlas-api/internal/exercise/seed.go`). `atlas-api/seed/exercises.csv` and `atlas-api/seeds/exercises.csv` both exist and **differ**.
- Migrations seed `muscle_groups` (`20260227120000_exercise_biomechanics_assets.sql`) and the subscription product mapping. They contain no programs or recipes.
- Effect: on a fresh DB, `GET /api/v1/programs` returns `[]`, and `POST /api/v1/nutrition/meal-plan/generate` returns 400 "no recipes are available" (`atlas-api/internal/httpapi/nutrition_planning_handlers.go`, around line 228).
- Recipes:
  - The `recipes` table (`migrations/20260227110000_weekly_checkins_and_meal_planning.sql`) has no owner.
  - `PostNutritionRecipeImport` (`nutrition_planning_handlers.go`, around lines 591-660) inserts confirmed imports with raw SQL through `s.queryDB`, so a user's imported recipe joins the global catalog and appears in **every** user's generated meal plans.
- Codegen for new SQL: add the query to `atlas-api/internal/db/queries/*.sql`, run `make generate` (needs Docker for sqlc), `git add` the generated files, and run `make verify-generated`.

**Steps:**
1. Create `atlas-api/seed/reference/` with data files (CSV or JSON) for programs → weeks → sessions → session exercises (keyed by slugs) and recipes (keyed by slug), extracted from `001_local_seed.sql`.
2. Merge the two exercise CSVs into one canonical `atlas-api/seed/reference/exercises.csv`: take the union, list the differences in the PR, and delete the duplicate.
3. Add `go run ./cmd/seed reference --dir ./seed/reference`. It upserts everything by slug in one transaction, is idempotent (running it twice leaves no duplicates), and works when `APP_ENV` is staging or prod.
4. Add a Makefile target `api-seed-reference`.
5. Reduce `seeds/001_local_seed.sql` to demo-only rows: demo user, profile, goals, consents. Make `make api-seed` run `psql -v ON_ERROR_STOP=1`, document that it requires `make api-seed-reference` first, and have the target refuse to run unless `APP_ENV` is unset or `local`.
6. Add migration `…_recipe_owner.sql` with `owner_user_id UUID NULL REFERENCES users(id) ON DELETE CASCADE` and an index. NULL means catalog.
7. Move the recipe-import insert into a sqlc query that sets `owner_user_id`.
8. Meal-plan generation uses the catalog plus the caller's own recipes only; recipe listing follows the same rule.
9. Add an integration test showing user A's imported recipe never appears in user B's generated plan.
10. Update `atlas-api/README.md` and the root `README.md` with the new seed flow.

**Acceptance criteria:**
- On an empty DB: `make api-migrate-up && make api-seed-reference` (run twice) leaves exactly 2 programs and no users, and `GET /api/v1/programs` returns 2 programs.
- `make api-seed` on top of that succeeds locally with `ON_ERROR_STOP`.
- `cd atlas-api && go build ./... && go vet ./... && ATLAS_TEST_POSTGRES_URL=<throwaway db> go test ./...` pass.
- `make generate && git add -A atlas-api/internal/db/sqlc && make verify-generated` exits 0.

**Out of scope:** Writing new program or recipe content (the owner supplies it later in the same file format), coach-session content, biomechanics asset storage, mobile code.

---

### T08: Infra: base/overlays, migration job, Dockerfile

**Goal:** Restructure the Kubernetes manifests into a reusable base with staging and prod overlays, run database migrations automatically before each rollout, and leave every environment-specific value as one clearly marked input.

**Context:**
- Repo: Atlas monorepo. Current manifests: `infra/staging/k8s/` (namespace, Deployment, Service, Ingress without TLS, `kustomization.yaml` with image `ghcr.io/your-org/atlas-api:staging-latest`, secret example `atlas-api-staging-secrets.example.yaml`), `infra/staging/otel-collector/` (traces exported only to `debug`), `infra/staging/argocd/atlas-api-staging-application.yaml` (repoURL `https://github.com/your-org/atlas.git`), and `infra/staging/smoke/smoke-staging.sh`.
- The real GitHub repo is `github.com/danielwellz/Atlas`.
- CI (`.github/workflows/ci.yml`, job `staging-deploy`) builds and pushes `ghcr.io/<owner>/atlas-api:<sha12>`, rewrites `newName`/`newTag` in `infra/staging/k8s/kustomization.yaml` with `sed`, commits, then syncs Argo app `atlas-api-staging`.
- There is **no migration step** (`infra/staging/k8s/managed-dependencies.md:11` says migrations must run first).
- `atlas-api/Dockerfile` builds only `./cmd/atlas-api`. Goose migrations are in `atlas-api/migrations/` and are run locally with `go run github.com/pressly/goose/v3/cmd/goose@v3.24.1 -dir migrations postgres "$POSTGRES_URL" up`, using build tags `no_clickhouse no_libsql no_mssql no_mysql no_sqlite3 no_vertica no_ydb`.
- The reference-data command (from T07, if merged) is `atlas-api/cmd/seed reference --dir ./seed/reference`.
- `kubectl kustomize infra/staging/k8s` renders today; keep everything renderable with `kubectl kustomize`.

**Steps:**
1. Create `infra/k8s/base/` (Deployment, Service, Ingress, OTel collector) and `infra/k8s/overlays/{staging,prod}/`, each with its own namespace, replicas, resources, ingress host, TLS secret name, image, and config. Move or replace `infra/staging/k8s`, and update every reference: CI `sed` paths, the Argo Application path, READMEs.
2. Add a migration image. Either a second Dockerfile target `migrate`, containing the goose binary built with the tags above plus `atlas-api/migrations/` (and the seed binary and reference data if T07 is merged), or include `goose` in the main image.
3. Add a Kubernetes `Job` annotated `argocd.argoproj.io/hook: PreSync` with `hook-delete-policy: BeforeHookCreation` that runs `goose up` using `POSTGRES_URL` from the same Secret. Optionally run `seed reference` after it.
4. Harden the Deployment: `securityContext` (runAsNonRoot, readOnlyRootFilesystem, drop ALL capabilities), a PodDisruptionBudget, `startupProbe`, and resource limits.
5. Ingress: add the `tls:` block and a `cert-manager.io/cluster-issuer` annotation, with the hostname and issuer as overlay patches.
6. Put every owner-specific value in one file per overlay (for example `overlays/prod/values.env` consumed by a kustomize `replacements`, or clearly commented patches) with `TODO(owner)` markers: domain, image registry or org, cluster issuer, Argo repoURL. Set the Argo repoURL to `https://github.com/danielwellz/Atlas.git`.
7. Add an Argo Application for prod with manual sync, no `automated`.
8. Make the Dockerfile multi-arch (`ARG TARGETARCH`) if T02 hasn't already.
9. Update CI so `staging-deploy` builds and pushes both images and updates the staging overlay.

**Acceptance criteria:**
- `kubectl kustomize infra/k8s/overlays/staging` and `kubectl kustomize infra/k8s/overlays/prod` both render without errors, and the output contains the PreSync migration Job, TLS on the Ingress, and no `your-org` or `example.com` strings outside lines marked `TODO(owner)`.
- `docker build` succeeds for both images.
- Running the migrate image against an empty local Postgres (`docker run --rm -e POSTGRES_URL=... <migrate-image>`) migrates it to the latest version.
- CI YAML is valid (actionlint, or the workflow runs green on the PR for its non-deploy jobs).

**Out of scope:** Real domains, cluster credentials, or secrets (owner, then T19); choosing a trace backend (T19); Terraform or cluster provisioning; API code changes.

---

### T09: Mobile environment config (API URL per build)

**Goal:** The React Native app takes its API base URL (and other environment values) from build-time configuration, so debug builds hit local dev, staging builds hit staging, and release builds hit production over HTTPS.

**Context:**
- Repo: Atlas monorepo; mobile in `atlas-mobile/`: React Native 0.84 CLI (not Expo), TypeScript, Node ≥22.11, Android `atlas-mobile/android`, iOS `atlas-mobile/ios`.
- Checks: `npm ci && npm run lint && npm run typecheck && npm test -- --watchAll=false`.
- Today `atlas-mobile/src/api/client.ts` hardcodes `http://10.0.2.2:8080` (Android) and `http://localhost:8080` (iOS) into `API_BASE_URL` and the openapi-fetch client `atlasApiClient`.
- `src/api/services/biomechanicsService.ts` builds URLs from `API_BASE_URL` with raw `fetch`.
- Android release builds disallow cleartext HTTP (the manifest uses the `${usesCleartextTraffic}` placeholder set by the RN Gradle plugin; not verified by a build). iOS `Info.plist` has `NSAllowsLocalNetworking=true`.
- There is no env library in `package.json`.

**Steps:**
1. Add build-time config with the smallest reliable approach for RN 0.84 bare apps. Either use `react-native-config` (verify it supports the New Architecture, which is enabled: `newArchEnabled=true` in `android/gradle.properties`), or write a tiny native-free alternative: a generated `src/config/env.ts` produced by a Node script from `ATLAS_ENV` before Metro bundles, wired into Gradle and Xcode build phases.
2. Define `local`, `staging`, and `prod` with `API_BASE_URL`; leave the staging and prod values as `TODO(owner)` placeholders, read from env or CI secrets.
3. `src/api/client.ts` reads `API_BASE_URL` from config, keeping the current local defaults for debug.
4. Fail fast at startup if a release build has a non-HTTPS URL or a placeholder.
5. Map Android build types and iOS schemes or configurations to environments: debug → local; a new Android `staging` build type plus iOS "Staging" configuration → staging; release → prod. Keep this minimal.
6. Make `biomechanicsService.ts` use `atlasApiClient` (the path `/api/v1/exercises/{id}/biomechanics` exists in the generated types) instead of raw `fetch`.
7. Document the setup in `atlas-mobile/README.md`.

**Acceptance criteria:**
- `cd atlas-mobile && npm ci && npm run lint && npm run typecheck && npm test -- --watchAll=false` pass.
- A unit test proves config resolution (prod rejects `http://`).
- `grep -rn "10.0.2.2\|localhost:8080" atlas-mobile/src` only matches the local-env definition.
- If an Android SDK is available: `cd atlas-mobile/android && ./gradlew assembleDebug` succeeds. Otherwise rely on the CI `android-debug-build` job if it exists, and say so in the PR.

**Out of scope:** Token refresh (T10), release signing (T17/T18), feature flags (T12), changing API endpoints.

---

### T10: Mobile token refresh

**Goal:** The mobile app keeps users signed in by transparently refreshing expired access tokens, and logs out only when the refresh token itself is invalid.

**Context:**
- Repo: Atlas monorepo; mobile in `atlas-mobile/` (RN 0.84, TypeScript, React Query, openapi-fetch). Checks: `npm ci && npm run lint && npm run typecheck && npm test -- --watchAll=false`.
- The API issues 15-minute access tokens and 30-day refresh tokens (`atlas-api/.env.example`: `ACCESS_TOKEN_TTL_MINUTES=15`, `REFRESH_TOKEN_TTL_HOURS=720`).
- `POST /api/v1/auth/refresh` (body `{ refreshToken }`) returns new tokens and **revokes the old refresh token** (rotation). The operation is in the generated types `atlas-mobile/src/api/generated/openapi.ts`.
- **The app never calls it.** `src/state/AuthContext.tsx` hydrates by calling `GET /api/v1/me` with the stored access token and on *any* failure runs `clearTokens()`, so every cold start more than 15 minutes after login logs the user out, and every API call fails with 401 after 15 minutes.
- Tokens are stored in Keychain via `src/storage/tokenStorage.ts`.
- Every service in `src/api/services/*.ts` passes `Authorization: Bearer ${accessToken}` explicitly, taking `accessToken` from `session.tokens.accessToken` in hooks and screens. The offline outbox (`src/sync/outbox.ts`, `src/sync/OutboxSyncController.tsx`) receives the access token when flushing.
- Mock mode (`src/state/MockModeContext.tsx`) is `__DEV__`-only; keep it working in tests.

**Steps:**
1. Create a token manager module (for example `src/auth/tokenManager.ts`) that holds the current tokens, exposes `getValidAccessToken()`, and does a **single-flight** refresh: concurrent 401s share one refresh call. Rotated tokens persist via `saveTokens`.
2. Add an openapi-fetch middleware on `atlasApiClient` (`client.use({ onRequest, onResponse })`) that injects the current access token when the request has none, and on a 401 refreshes once and retries the request once. Exclude the `/auth/*` endpoints.
3. Remove the need for callers to pass stale tokens where it's easy; keep the existing service signatures if changing them is too broad, but make sure the middleware overrides a stale header with the fresh token.
4. AuthContext hydrate: on a 401 from `/me`, try a refresh. Clear tokens and log out only if the refresh returns 401. On a network error, keep the session (offline start).
5. The outbox flush must use `getValidAccessToken()` at flush time.
6. Tests (jest, mocking `fetch`):
   - Access token expired → refresh → original request retried and succeeds.
   - Two parallel requests trigger exactly one refresh call.
   - Refresh 401 → session cleared.
   - Offline hydrate keeps the session.

**Acceptance criteria:**
- `cd atlas-mobile && npm run lint && npm run typecheck && npm test -- --watchAll=false` pass, including the new tests.
- `grep -rn "/api/v1/auth/refresh" atlas-mobile/src --include=*.ts` shows the new call site, outside `generated/`.
- Manual check, described in the PR: with the API's `ACCESS_TOKEN_TTL_MINUTES=1`, a logged-in app still loads the dashboard after 2 minutes and after an app restart.

**Out of scope:** Server auth changes, biometric lock, password reset, outbox retry policy (T11).

---

### T11: Offline outbox robustness

**Goal:** The mobile offline outbox never retries permanently failing items forever, never sends one user's queued writes under another user's session, and supports finishing a workout offline.

**Context:**
- Repo: Atlas monorepo; mobile in `atlas-mobile/`. Checks: `npm ci && npm run lint && npm run typecheck && npm test -- --watchAll=false`.
- The outbox is `src/sync/outbox.ts`: an AsyncStorage key `atlas.mobile.sync.outbox.v1`, max 500 items, kinds `workout_set`, `food_log`, `nutrition_checkin`, `analytics_event`, exponential backoff capped at 60 s.
- In `flushOutboxInternal`, any error other than `NonRetryableEventError` on analytics events is rescheduled forever, so a 400 or 409 (for example adding a set to a workout already completed) becomes a poison item.
- Items carry no user id, and `clearOutbox()` (around line 213) is never called. Logout is in `src/state/AuthContext.tsx`.
- `OutboxSyncController.tsx` flushes when online. Existing tests: `__tests__/utilities/outbox.test.ts`, `__tests__/integration/offline-outbox-sync.test.tsx`.
- Workout flows (`src/features/workout/hooks.ts`): only add-set is enqueued when offline (around line 137). `startWorkout` and `completeWorkout` require a connection.
- The API's `POST /api/v1/workouts/{workout_id}/add_set` is idempotent on `idempotency_key`. `POST /api/v1/workouts/{workout_id}/complete` returns 409 when already complete.
- Use `getValidAccessToken()` from the token manager if T10 is merged; otherwise use the existing token flow.

**Steps:**
1. Add a `userId` to every outbox item at enqueue. Flush only the current user's items. On logout or user switch, drop items that belong to other users (log a count).
2. Classify errors. Network errors, 5xx, 408, and 429 → retry with backoff. 401 → pause the flush (refresh or re-login resumes it). Other 4xx → move to a dead-letter list (new key) with the status and message, and expose the count through a hook so the UI can show "N items couldn't sync".
3. Cap retries (for example 20) before dead-lettering.
4. Add a `workout_complete` outbox kind. Completing while offline enqueues it after all pending sets for that workout. Treat a 409 "already completed" as success.
5. Update `src/ui/SyncPendingIndicator.tsx` to show the dead-letter count.
6. Tests for each rule: a poison item is dead-lettered, other users' items aren't sent, ordering of sets before completion holds, and a 409 on complete counts as success.

**Acceptance criteria:**
- `cd atlas-mobile && npm run lint && npm run typecheck && npm test -- --watchAll=false` pass, including the new tests.
- Existing outbox tests still pass, or are updated with a stated reason.
- No API changes; `make verify-generated` is unaffected.

**Out of scope:** Offline workout *start* (it needs server-generated ids; leave it online-only), server changes, conflict resolution UI.

---

### T12: MSV navigation, feature flags, Settings screen, mock cleanup

**Goal:** The app shows only MSV features in a clean tab structure, with deferred features hidden behind build-time flags, a Settings screen for account actions, and no dead mock code.

**Context:**
- Repo: Atlas monorepo; mobile in `atlas-mobile/` (RN 0.84, React Navigation 7). Checks: `npm ci && npm run lint && npm run typecheck && npm test -- --watchAll=false`.
- `src/navigation/MainTabsNavigator.tsx` registers 13 routes in one bottom-tab navigator, 9 of them visible tabs: Dashboard, Crew, Anatomy, Food, MealPlan, WeeklyCheckIn, Programs, WorkoutRunner, PrivacySettings. Paywall, BarcodeScan, FormCheck, and CoachSessionPlayer are hidden via `tabBarButton: () => null`. Param types are in `src/navigation/types.ts`.
- Logout is a button on `src/screens/dashboard/DashboardScreen.tsx` (around line 715).
- Deferred for the MSV: Anatomy (Unity; `src/screens/anatomy/AnatomyScreen.tsx`), Form Check (synthetic native pose frames; `src/screens/workout/FormCheckScreen.tsx`), Crew and Coach sessions (`src/screens/community/*`; no content exists), and biomechanics overlays opened from `WorkoutRunnerScreen.tsx` (around line 621, `getExerciseBiomechanics` / `openUnity`).
- The Paywall (`src/screens/billing/PaywallScreen.tsx`) is shown via `navigation.navigate('Paywall', { feature })` from entitlement-gated screens (`src/features/entitlements.ts`). Whether the paywall ships in 1.0 is an owner decision, so make it a flag.
- Dead code: `src/api/services/mvpService.ts` is imported nowhere. `src/api/mockData.ts` is used by dev-only mock branches in `programsService.ts` and `workoutService.ts`. Mock mode is gated by `__DEV__` (`src/state/MockModeContext.tsx`).
- If T09 has landed, there's a build-config module (for example `src/config/env.ts`). Read flags from it; otherwise create `src/config/features.ts`.

**Steps:**
1. Add a typed feature-flag module with flags `anatomy`, `formCheck`, `community`, `paywall`, `barcode`, and `mealPlan`. MSV defaults: anatomy, formCheck, and community off; paywall, barcode, and mealPlan configurable per environment, with defaults paywall off and barcode and mealPlan on.
2. Restructure navigation into at most 5 tabs: **Today** (Dashboard), **Train** (a stack: Programs → WorkoutRunner), **Nutrition** (a stack: Food → BarcodeScan / MealPlan / WeeklyCheckIn), **Settings** (a stack: Settings → Privacy), plus Community only when its flag is on. Put Paywall and hidden flows in a root modal stack. Register routes for disabled features only when their flag is on.
3. When the paywall is off, entitlement-gated features either become free (if their flag is on) or are hidden. Never navigate to a disabled Paywall. Implement this in `src/features/entitlements.ts`, not per screen.
4. Create `src/screens/settings/SettingsScreen.tsx` with account email, Logout (moved from Dashboard), a link to Privacy, the app version, and an entry point placeholder "Delete account" (T14 implements it).
5. Hide biomechanics and Unity entry points in WorkoutRunner when `anatomy` is off.
6. Delete `mvpService.ts`. Confirm with `grep` that mock data is only reachable under `__DEV__`, and add a unit test asserting `canUseMockMode === false` when `__DEV__` is false (mock the global).
7. Update existing navigation and integration tests.

**Acceptance criteria:**
- `cd atlas-mobile && npm run lint && npm run typecheck && npm test -- --watchAll=false` pass.
- A test renders `MainTabsNavigator` with MSV flags and asserts the visible tab labels are exactly Today, Train, Nutrition, Settings.
- `grep -rn mvpService atlas-mobile/src` returns nothing.

**Out of scope:** Deleting the deferred features' code (keep it behind flags), the account-deletion implementation (T14), crash reporting (T16), splitting the large screens.

---

### T13: Habit creation and nutrition-targets UI

**Goal:** Users can create and archive daily habits and set their calorie and protein targets in the app, using API endpoints that already exist.

**Context:**
- Repo: Atlas monorepo; mobile in `atlas-mobile/`. Checks: `npm ci && npm run lint && npm run typecheck && npm test -- --watchAll=false`.
- Generated API types: `src/api/generated/openapi.ts`; client: `atlasApiClient` in `src/api/client.ts`.
- **Habits:**
  - The Dashboard (`src/screens/dashboard/DashboardScreen.tsx`, around lines 609-620) lists habits from `GET /api/v1/habits` and toggles them via `POST /api/v1/habits/{id}/toggle_today` (`src/api/services/habitService.ts`, `src/features/dashboard/hooks.ts`).
  - `POST /api/v1/habits` (create; see the request schema in `atlas-api/openapi/openapi.yaml`) and `GET /api/v1/habits/streaks` are never called, and the server creates no default habits, so the list is always empty.
  - The DB enforces a maximum number of active habits per user (trigger `enforce_max_active_habits_per_user` in `atlas-api/migrations/20260227030000_habit_system_mvp.sql`); surface that error.
- **Nutrition targets:**
  - `PUT /api/v1/nutrition/targets` exists (handler `atlas-api/internal/httpapi/nutrition_handlers.go`), but mobile never calls it.
  - `GET /api/v1/nutrition/today` returns `targets_configured: false` when unset.
  - Targets are otherwise only written by the Pro weekly check-in.
  - Nutrition service: `src/api/services/nutritionService.ts`; screens: `src/screens/nutrition/*`.
- If T12 has landed, the Nutrition tab is a stack and Settings exists. Otherwise add the entry points to the existing Food screen and Dashboard.

**Steps:**
1. Add `createHabit` (and `getHabitStreaks` if trivial) to `habitService.ts`, plus a React Query mutation that invalidates the dashboard query. Add an "Add habit" button and a simple form on the Dashboard habits card, and show streaks if cheap. Show the server's max-habits error message.
2. Add `putNutritionTargets` to `nutritionService.ts` and a "Set targets" screen or sheet (calories and protein required; carbs and fat optional, if the schema allows) reachable from the Nutrition area. When `targets_configured` is false, show a prompt card linking to it.
3. Tests: create-habit flow (form → service called → list refetched); targets flow (prompt card appears when unconfigured → save → today query refetched).

**Acceptance criteria:**
- `cd atlas-mobile && npm run lint && npm run typecheck && npm test -- --watchAll=false` pass, with the new tests.
- `grep -rn "'/api/v1/habits'" atlas-mobile/src/api/services` shows a POST call, and `grep -rn "nutrition/targets" atlas-mobile/src/api/services` shows a PUT call.
- No OpenAPI changes; `make verify-generated` passes.

**Out of scope:** Server changes, computing recommended targets from the onboarding profile (nice-to-have), habit reminders or push.

---

### T14: Privacy UI: delete account and export data

**Goal:** Users can export their data and permanently delete their account from the app's Settings/Privacy screens, as App Store and Google Play require.

**Context:**
- Repo: Atlas monorepo; mobile in `atlas-mobile/`. Checks: `npm ci && npm run lint && npm run typecheck && npm test -- --watchAll=false`.
- The API (added in task T06) exposes `DELETE /api/v1/me` (body `{ password }`, returns 204; 401 on a wrong password) and `GET /api/v1/me/export` (JSON). Confirm both exist in `atlas-mobile/src/api/generated/openapi.ts`. If they don't, stop and report that T06 must be merged first.
- Privacy screen: `src/screens/settings/PrivacySettingsScreen.tsx` (consents). A Settings screen (`src/screens/settings/SettingsScreen.tsx`) may exist from T12 with a "Delete account" placeholder.
- Auth state and logout: `src/state/AuthContext.tsx`. Offline outbox: `src/sync/outbox.ts` (`clearOutbox`). React Query cache persistence is in `src/app/AppProviders.tsx` (AsyncStorage key `atlas.mobile.query-cache.v1`).
- RN's built-in `Share` API can share a text payload. No file-system library is installed; don't add one unless necessary.

**Steps:**
1. Add `deleteAccount(password)` and `exportMyData()` to `src/api/services/authService.ts` (or a new `accountService.ts`) using `atlasApiClient`.
2. Delete flow: Settings → "Delete account" → a screen explaining permanence → password field → a confirm button that is visually destructive and requires a second confirmation. On 204: clear tokens, outbox, persisted query cache, and onboarding storage, then return to the auth stack. On 401: show "Incorrect password".
3. Export flow: "Export my data" → call the API → open the share sheet with the pretty-printed JSON (or a summary plus JSON if it's too large; document the limit).
4. Tests: delete success clears session and outbox; wrong password shows an error and keeps the session; export calls `Share.share`.

**Acceptance criteria:**
- `cd atlas-mobile && npm run lint && npm run typecheck && npm test -- --watchAll=false` pass, with the new tests.
- The delete path is reachable within two taps of the Settings tab, which the store reviewers expect.

**Out of scope:** API changes, web-based deletion page (an owner or store-listing task), password reset.

---

### T15: Real store receipt verification (only if the paywall ships in 1.0)

**Goal:** `POST /api/v1/billing/verify` grants entitlements only after verifying the purchase with Apple (App Store Server API) or Google (Play Developer API), using server-side expiry, never client-supplied values.

**Context:**
- Repo: Atlas monorepo. API in `atlas-api/` (OpenAPI-first; spec `atlas-api/openapi/openapi.yaml`); mobile in `atlas-mobile/`.
- Handler: `atlas-api/internal/httpapi/billing_handlers.go`. It currently validates non-empty fields, computes expiry from the **client-sent `expiresAt`** or a 30/365-day guess, and upserts `subscriptions` with `status=active`. The audit verified that a fake receipt yields elite until 2099.
- Entitlements come from the DB view `user_entitlements` joining `subscriptions` and `subscription_product_entitlements` (`atlas-api/migrations/20260228080000_monetization_subscriptions_v1.sql`). Product ids: `atlas.pro.monthly`, `atlas.pro.yearly`, `atlas.elite.monthly`, `atlas.elite.yearly`.
- Mobile:
  - `atlas-mobile/src/features/billing/iap.ts` (react-native-iap v14) extracts `purchaseToken`, falling back to `purchase.id`, plus transaction ids and `expirationDateIOS`.
  - `src/screens/billing/PaywallScreen.tsx` (around lines 150-190) sends `expiresAt` to the server.
  - `src/api/services/billingService.ts` calls the API.
- Task T04 may have added a `BILLING_VERIFY_ENABLED` flag (returns 403 when false). Keep it.
- Credentials are owner-provided and must come from env only:
  - Apple: `APPLE_IAP_KEY_ID`, `APPLE_IAP_ISSUER_ID`, `APPLE_IAP_PRIVATE_KEY` (.p8 PEM), `APPLE_BUNDLE_ID`, `APPLE_IAP_ENVIRONMENT` (Sandbox or Production).
  - Google: `GOOGLE_PLAY_PACKAGE_NAME`, `GOOGLE_PLAY_SERVICE_ACCOUNT_JSON`.
  - If they're absent, verification must fail closed (no grant).
- Codegen: after spec edits, run `make generate` (needs Docker and `cd atlas-mobile && npm ci` under Node ≥22.11), `git add` the generated files, then `make verify-generated`, which must exit 0.

**Steps:**
1. Create a package `atlas-api/internal/billing/` with a `Verifier` interface: `Verify(ctx, platform, productID, token, transactionID) (VerifiedPurchase{ProductID, TransactionID, OriginalTransactionID, ExpiresAt, Status, Environment}, error)`.
2. Apple implementation: call the App Store Server API "Get Transaction Info" for the transaction id with an ES256 JWT signed by the .p8 key, then verify the returned JWS (`signedTransactionInfo`) certificate chain against Apple's root CA (embed the Apple Root CA G3 cert) and check `bundleId`, `productId`, and `expiresDate`.
3. Google implementation: OAuth2 service-account token (scope `https://www.googleapis.com/auth/androidpublisher`), then `purchases.subscriptionsv2.get` for the purchase token. Map `subscriptionState`, `lineItems[].expiryTime`, and `productId`. Acknowledge the purchase if it isn't acknowledged.
4. Use `golang.org/x/oauth2/google` and `github.com/golang-jwt/jwt/v5` (already a dependency); avoid heavy SDKs.
5. Handler: ignore client `expiresAt`. Mark `expiresAt` deprecated in the spec, but keep the field so old clients don't break. Reject a product mismatch. Make the transaction id unique per platform (it's already the upsert key; verify that). Store the verified payload, not the client payload, in `raw_receipt`.
6. Tests: unit tests with fake HTTP servers for both stores (valid, expired, wrong bundle, wrong product, revoked) and an integration test that a mismatched or failed verification grants nothing.
7. Mobile:
   - Stop sending `expiresAt`.
   - On iOS send the transaction id (and the JWS if react-native-iap exposes it); on Android send `purchaseToken`. Never fall back to `purchase.id` as a receipt.
   - Call `finishTransaction` only after the server returns 200.
   - Keep the restore flow.
   - Update `atlas-mobile/__tests__` accordingly.
8. Document the env vars in `atlas-api/.env.example` and the staging secret example.

**Acceptance criteria:**
- `cd atlas-api && go build ./... && go vet ./... && ATLAS_TEST_POSTGRES_URL=<throwaway db> go test ./...` pass.
- A test proves the 2099 exploit now returns a non-200 response and no `subscriptions` row.
- `make generate && git add -A atlas-api/internal/httpapi/generated atlas-mobile/src/api/generated && make verify-generated` exits 0.
- `cd atlas-mobile && npm run lint && npm run typecheck && npm test -- --watchAll=false` pass.
- The PR lists the exact owner steps to obtain each credential, plus a sandbox test plan.

**Out of scope:** App Store Server Notifications or Play RTDN (T21), price display or localisation, grace-period UX, creating store products (owner).

---

### T16: Crash reporting and error boundary

**Goal:** Release builds report JS and native crashes and unhandled errors to a crash-reporting service, and the UI shows a recoverable error screen instead of a white screen.

**Context:**
- Repo: Atlas monorepo; mobile in `atlas-mobile/` (RN 0.84 bare CLI app, New Architecture enabled, Hermes). Checks: `npm ci && npm run lint && npm run typecheck && npm test -- --watchAll=false`.
- There's no crash reporting today and no React error boundary. App root: `atlas-mobile/App.tsx` → `src/app/AppProviders.tsx` → `src/navigation/RootNavigator.tsx`.
- Environment config may exist from T09 (for example `src/config/env.ts`). The DSN must come from build config or env, never hardcoded. If it's missing, initialization is a no-op.
- The vendor is the owner's choice. Default to Sentry (`@sentry/react-native`) unless the repo already contains Firebase config.

**Steps:**
1. Add `@sentry/react-native` and follow its bare-RN setup for Android (Gradle plugin for source maps) and iOS (the Podfile/Xcode build phase changes; you can't run `pod install` without macOS, so make the file changes and note that in the PR).
2. Initialize it in `index.js` or `App.tsx` with `dsn` from config, `environment`, and release and dist from the app version. Disable it in `__DEV__` and in jest.
3. Scrub PII: no emails or auth headers, and set `sendDefaultPii: false`.
4. Add an `ErrorBoundary` component wrapping `RootNavigator`, with a "Something went wrong — Retry" screen that remounts the tree.
5. Add a jest mock for the SDK in `jest.setup.js`.
6. Add a test that a thrown render error shows the fallback.

**Acceptance criteria:**
- `cd atlas-mobile && npm run lint && npm run typecheck && npm test -- --watchAll=false` pass.
- With no DSN configured, the app starts and no network calls are made to the vendor (unit test on the init helper).
- The PR lists the owner steps: create a project, add the DSN to CI secrets, and set up the auth token for source-map upload.

**Out of scope:** Performance tracing, analytics, push, native crash tests on devices.

---

### T17: Android release build: IDs, signing, icons, CI artifact

**Goal:** CI produces a signed Android App Bundle for production with the final application id, version numbers, app name, and launcher icon, using a keystore supplied only through CI secrets.

**Context:**
- Repo: Atlas monorepo; mobile in `atlas-mobile/` (RN 0.84, Gradle 9 wrapper, compile/target SDK 36, JDK 17 required).
- `atlas-mobile/android/app/build.gradle`:
  - `applicationId "com.atlasmobile"`, `namespace "com.atlasmobile"`, `versionCode 1`, `versionName "1.0"`.
  - The `release` build type uses `signingConfigs.debug` with the committed `android/app/debug.keystore`.
  - `minifyEnabled enableProguardInReleaseBuilds`.
- App name `AtlasMobile` (`android/app/src/main/res/values/strings.xml`, `atlas-mobile/app.json`). Launcher icons are the default Android robot (`android/app/src/main/res/mipmap-*`). Kotlin sources live under package `com.atlasmobile` (`MainActivity.kt`, `MainApplication.kt`, `formcheck/*`, `unity/*`).
- An Android Gradle subproject `:unityLibrary` is included only if `atlas-unity/Builds/android/unityLibrary` exists (it doesn't).
- Environment config may exist from T09. CI is `.github/workflows/ci.yml`; T02 may have added an `android-debug-build` job.
- **Owner inputs** (use `TODO(owner)` placeholders if they're not provided in this prompt):
  - final applicationId, for example `app.<yourdomain>.atlas`;
  - display name;
  - a 512×512 icon source PNG;
  - CI secrets `ANDROID_UPLOAD_KEYSTORE_BASE64`, `ANDROID_UPLOAD_KEYSTORE_PASSWORD`, `ANDROID_UPLOAD_KEY_ALIAS`, `ANDROID_UPLOAD_KEY_PASSWORD`.

**Steps:**
1. Make `applicationId` configurable via a Gradle property with the owner value or a placeholder. Keep the `namespace` and Kotlin package unchanged unless the owner asks; the applicationId can differ from the namespace.
2. `versionCode` comes from a `-PversionCode` property (CI uses `github.run_number`) and `versionName` from `package.json` `version`.
3. Add a `release` signingConfig that reads the keystore path, passwords, and alias from Gradle properties or env. If they're absent, the release build must **fail** with a clear message, never fall back to debug signing. Keep `debug.keystore` for debug only.
4. Enable R8 for release (`enableProguardInReleaseBuilds=true`) and add keep rules for the custom native modules (`com.atlasmobile.formcheck.*`, `com.atlasmobile.unity.*`), react-native-iap, keychain, and camera-kit as their docs require. Verify the release build doesn't crash at startup if you can run an emulator; otherwise note it.
5. Set the display name. Generate adaptive icons (foreground and background, all densities) from the owner PNG if provided; otherwise leave the placeholder and a TODO. Add a simple splash or background color consistent with the app theme.
6. CI: add a job `android-release` that runs on pushes to `main` and manual dispatch. It decodes the keystore secret to a temp file and runs `./gradlew bundleRelease -PversionCode=${{ github.run_number }}` with the prod env config, then uploads the `.aab` as a workflow artifact. Skip it with a notice when the secrets are absent, for example on forks.
7. Document the release process and Play upload in `atlas-mobile/README.md`.

**Acceptance criteria:**
- `cd atlas-mobile && npm run lint && npm run typecheck && npm test -- --watchAll=false` pass.
- `./gradlew assembleDebug` still works (locally or in CI).
- `./gradlew bundleRelease` without signing properties fails with the explicit message.
- With a throwaway keystore you generate locally via `keytool` (don't commit it), `bundleRelease` succeeds and `jarsigner -verify` passes.
- The CI workflow file is valid.
- `grep -n "signingConfigs.debug" atlas-mobile/android/app/build.gradle` appears only inside the debug build type.

**Out of scope:** Uploading to Play Console (owner), iOS (T18), Unity integration, store listing assets.

---

### T18: iOS release config and CI build

**Goal:** The iOS app has its final bundle id, display name, icons, accurate privacy manifest and permission strings, committed Pods lockfile, and a CI job that builds an unsigned release archive on macOS.

**Context:**
- Repo: Atlas monorepo; mobile in `atlas-mobile/` (RN 0.84, iOS deployment target 15.1, CocoaPods via `atlas-mobile/ios/Podfile`, `atlas-mobile/Gemfile`).
- `atlas-mobile/ios/AtlasMobile.xcodeproj/project.pbxproj` has `PRODUCT_BUNDLE_IDENTIFIER = "org.reactjs.native.example.$(PRODUCT_NAME:rfc1034identifier)"` (around lines 274 and 303) and no `DEVELOPMENT_TEAM`.
- `atlas-mobile/ios/AtlasMobile/Info.plist`:
  - `CFBundleDisplayName` is `AtlasMobile`;
  - `NSLocationWhenInUseUsageDescription` is an empty string (the app doesn't use location);
  - `NSCameraUsageDescription` mentions only barcode scanning (form check also uses the camera, but it's deferred);
  - `NSAllowsLocalNetworking=true`.
- `ios/AtlasMobile/PrivacyInfo.xcprivacy` declares no collected data types, but the app collects email (account), fitness and health-adjacent data (workouts, nutrition, weight), purchase history, product-interaction analytics (`POST /api/v1/events`, consent-gated), and crash data (if T16 is merged).
- `ios/AtlasMobile/Images.xcassets/AppIcon.appiconset/Contents.json` has no images.
- Local pods `AtlasUnityBridge` and `AtlasFormCheckPoseBridge` live under `ios/`; the Unity framework is optional and absent.
- There's no `Podfile.lock` or `.xcworkspace` in git.
- **The audit machine had no Xcode.** Use a GitHub `macos-latest` runner (or a Mac if you have one) for verification.
- **Owner inputs** (use `TODO(owner)` placeholders if not provided): bundle id, display name, Apple Team ID, a 1024×1024 icon PNG.

**Steps:**
1. Set `PRODUCT_BUNDLE_IDENTIFIER` (Debug and Release) to the owner value or placeholder, set the display name, and remove the empty location string.
2. Broaden the camera string to cover barcode scanning (and form check, if it un-defers later).
3. Gate `NSAllowsLocalNetworking` to Debug, for example via a separate `Info-Debug.plist` or a build setting.
4. Fill `PrivacyInfo.xcprivacy` `NSPrivacyCollectedDataTypes` for the data listed above (linked to the user, not used for tracking, with purposes app functionality, analytics, and purchase), and keep the required-reason API entries.
5. Generate the AppIcon set from the owner PNG if provided.
6. Add a CI job `ios-build` on `macos-latest`: Node 22, `npm ci`, `bundle install`, `cd ios && bundle exec pod install`, then `xcodebuild -workspace AtlasMobile.xcworkspace -scheme AtlasMobile -configuration Release -sdk iphonesimulator -destination 'generic/platform=iOS Simulator' CODE_SIGNING_ALLOWED=NO build`. Commit the generated `Podfile.lock` (and the workspace) from that run, or from a local Mac.
7. Document in `atlas-mobile/README.md` how the owner sets the Team ID, signing, and archive or upload (Xcode, or fastlane later).

**Acceptance criteria:**
- `cd atlas-mobile && npm run lint && npm run typecheck && npm test -- --watchAll=false` pass.
- The `ios-build` CI job succeeds on the PR.
- `plutil -lint` passes for `Info.plist` and `PrivacyInfo.xcprivacy` (on macOS CI).
- `grep -n "org.reactjs.native.example" atlas-mobile/ios -r` returns nothing.

**Out of scope:** Signing certificates and provisioning profiles, App Store Connect upload (owner), Unity framework, push entitlements.

---

### T19: Infra real values: hosts, TLS, OTel backend, staging deploy

**Goal:** Fill the owner-provided values into the Kubernetes overlays and CI, send traces to a durable backend, and get one green staging deploy with passing smoke tests.

**Context:**
- Repo: Atlas monorepo. Task T08 restructured the manifests into `infra/k8s/base` and `infra/k8s/overlays/{staging,prod}`, with `TODO(owner)` markers and a PreSync migration Job. Check `git log` and the files to confirm.
- OTel collector config (`infra/k8s/base/.../otel-collector` config, originally `infra/staging/otel-collector/config.yaml`) exports traces only to `debug`.
- The CI job `staging-deploy` (`.github/workflows/ci.yml`):
  - needs the secrets `ARGOCD_SERVER`, `ARGOCD_AUTH_TOKEN`, `STAGING_API_BASE_URL`, and `STAGING_OTEL_COLLECTOR_METRICS_URL`;
  - checks collector metrics for `otelcol_receiver_accepted_spans`, which are the collector's *internal* telemetry metrics (normally on port 8888, which isn't exposed). The prometheus exporter is on 8889, so this check likely never passes as written.
- Smoke script: `infra/staging/smoke/smoke-staging.sh` (it may have moved in T08).
- **Owner inputs required in this prompt; stop if any are missing:**
  - staging and prod API hostnames;
  - cert-manager ClusterIssuer name;
  - container registry or org;
  - OTLP endpoint and auth header for the trace backend (for example Grafana Cloud, Honeycomb);
  - confirmation that the Secrets `atlas-api-staging-secrets` and `atlas-api-prod-secrets` exist in the cluster, with every variable from `atlas-api/.env.example`'s staging section, populated by the owner and never committed;
  - confirmation that GitHub secrets are set.

**Steps:**
1. Replace every `TODO(owner)` with the provided values.
2. Add an `otlphttp` exporter to the collector traces (and metrics) pipelines, using the endpoint and auth header from a Kubernetes Secret referenced via env. Keep `debug` only in staging, at `basic` verbosity.
3. Expose the collector's internal telemetry port (8888) through the Service, or switch the CI check to query the trace backend's API. Make the CI check match whatever is actually exposed.
4. Update `smoke-staging.sh` to also check register → login → `/me` → refresh with a random email, then delete that user via `DELETE /api/v1/me` if T06 is merged.
5. Trigger a staging deploy from `main` and iterate until it's green.

**Acceptance criteria:**
- `kubectl kustomize` renders both overlays with no `TODO(owner)`, `your-org`, or `example.com`.
- CI `staging-deploy` is green.
- `curl -fsS https://<staging-host>/api/v1/health` returns `"status":"ok"` over valid TLS.
- Traces from the staging API are visible in the chosen backend (screenshot or link in the PR).

**Out of scope:** Cluster provisioning, DNS registration, managed DB creation, prod deploy (a manual Argo sync by the owner after staging is verified).

---

### T20: Password reset by emailed code (post-MSV)

**Goal:** Users who forget their password can reset it with a 6-digit code emailed to them, without deep links.

**Context:**
- Repo: Atlas monorepo; API in `atlas-api/` (OpenAPI-first), mobile in `atlas-mobile/`. No email sending exists today, and the MailHog config was unused (T04 may have removed it).
- Auth handlers are in `atlas-api/internal/httpapi/server.go`; passwords are bcrypt; sessions are in `atlas-api/internal/db/queries/sessions.sql`.
- Rate limiting may exist from T05.
- **Owner input:** an email provider (for example Postmark, Resend, or SES) with its API key and sender address.
- Codegen: `make generate`, `git add` the generated files, then `make verify-generated`.

**Steps:**
1. Add `POST /api/v1/auth/password-reset/request` with body `{email}`. It always returns 202 (no account enumeration) and stores a hashed 6-digit code with a 15-minute expiry and a 5-attempt limit (new migration).
2. Add `POST /api/v1/auth/password-reset/confirm` with body `{email, code, newPassword}`. It returns 204, revokes all sessions, and enforces the existing password policy.
3. Create a pluggable `Mailer` interface with an SMTP implementation for local dev (mailpit or MailHog via compose profile) and an HTTP API implementation for the chosen provider. Both are configured by env.
4. Rate-limit both endpoints.
5. Mobile: a "Forgot password?" link on `src/screens/auth/LoginScreen.tsx` leading to request and confirm screens.
6. Add tests on both sides.

**Acceptance criteria:**
- `cd atlas-api && go build ./... && ATLAS_TEST_POSTGRES_URL=<throwaway db> go test ./...` pass.
- `make generate && git add -A … && make verify-generated` exits 0.
- `cd atlas-mobile && npm run lint && npm run typecheck && npm test -- --watchAll=false` pass.

**Out of scope:** Email verification at signup, magic links or deep links, social login.

---

### T21: Store server notifications (post-MSV)

**Goal:** Subscription renewals, cancellations, refunds, and expirations update entitlements automatically through App Store Server Notifications V2 and Google Play Real-Time Developer Notifications.

**Context:**
- Repo: Atlas monorepo; API in `atlas-api/`.
- Requires T15 (the `atlas-api/internal/billing/` verifiers, the `subscriptions` table keyed by transaction id).
- Entitlements derive from the `user_entitlements` view (status `active` or `grace_period`, not expired).
- **Owner inputs:** the Apple notification URL configured in App Store Connect, and a Google Pub/Sub push subscription to the API URL with an auth token or OIDC audience.

**Steps:**
1. Add `POST /api/v1/billing/notifications/apple`: verify the `signedPayload` JWS chain with T15's verifier, then update the subscription's status and expiry by `originalTransactionId`.
2. Add `POST /api/v1/billing/notifications/google`: verify the Pub/Sub push auth, decode the message, re-fetch the subscription via `subscriptionsv2.get`, and update it.
3. Both endpoints are public in the spec (no bearerAuth) and authenticated by signature or token; update the T04 allowlist test.
4. Make processing idempotent by notification id.
5. Add a daily re-verification job (`cmd/billing-reconcile`) for subscriptions expiring within 3 days.
6. Add tests with fixture payloads.

**Acceptance criteria:**
- `go test ./...` passes with a throwaway DB.
- `make generate && git add -A … && make verify-generated` exits 0.
- A fixture refund notification flips entitlements off in an integration test.

**Out of scope:** Mobile UI changes, analytics dashboards.

---

### T22: React Native upgrade (post-MSV, or earlier if store rules require it)

**Goal:** Upgrade `atlas-mobile` from React Native 0.84 to the latest stable release, with all checks and both platform builds passing.

**Context:**
- Repo: Atlas monorepo; mobile in `atlas-mobile/` (RN 0.84.0, React 19.2.3; `npm outdated` on 2026-10-06 showed RN 0.87.1 as latest).
- `npm audit --omit=dev` reported 48 vulnerabilities (1 critical, `shell-quote`) in build tooling (metro, `@react-native-community/cli`, jest).
- Custom native code:
  - Android Kotlin modules in `android/app/src/main/java/com/atlasmobile/{formcheck,unity}`;
  - iOS local pods `ios/AtlasUnityBridge`, `ios/AtlasFormCheckPoseBridge`.
- Native libraries: react-native-iap, keychain, camera-kit, permissions, screens, gesture-handler, safe-area-context, async-storage, netinfo.
- CI has Android (and maybe iOS) build jobs from T02, T17, and T18.

**Steps:**
1. Use the React Native Upgrade Helper diff (0.84.0 → target) to update native template files, `package.json`, Gradle, and the Podfile.
2. Bump the RN-adjacent libraries to versions compatible with the target. Move ESLint off v8 if `@react-native/eslint-config` requires it.
3. Fix breakages.

**Acceptance criteria:**
- `cd atlas-mobile && npm ci && npm run lint && npm run typecheck && npm test -- --watchAll=false` pass.
- The CI Android and iOS build jobs are green.
- `npm audit --omit=dev` has no critical findings.

**Out of scope:** Feature changes, refactoring large screens.
