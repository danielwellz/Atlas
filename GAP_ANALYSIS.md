# Atlas Gap Analysis

Audit date: 2026-10-06. Repo HEAD: `2f8206d` ("Add root README"), branch `main`, remote `git@github.com:danielwellz/Atlas.git`.

**Method.** I modified no existing files. Every build, codegen, migration, and test command ran in a throwaway `git clone` in a scratch directory outside the repo. In that clone I made three temporary patches, never applied to this repo, so I could keep going past failures: goose annotations in 3 migrations, the OTel semconv import, and the semconv function rename. Each is listed where it matters. File paths below are relative to the repo root.

---

## 0. Headline

The code base is further along than the old findings suggested. All 65 OpenAPI operations have Go implementations, mobile screens call the real API (mock mode only exists in `__DEV__`), and lint, typecheck, and 69/69 mobile tests pass. However, **as committed, the API cannot be deployed**:

1. **The API binary exits at startup in every environment.** OTel resource merge fails with `conflicting Schema URL: .../1.39.0 and .../1.26.0` (`atlas-api/internal/observability/otel.go:17,25-31`, which feeds `logger.Fatal` in `atlas-api/cmd/atlas-api/main.go:41-44`). No test exercises `InitOTel`.
2. **A fresh database can't be migrated.** Three migrations have `DO $$` blocks without `-- +goose StatementBegin/End`. GitHub CI run `28324207541` (2026-06-28) failed at exactly this step.
3. **Mobile sessions die after 15 minutes.** The app never calls `/auth/refresh` (`ACCESS_TOKEN_TTL_MINUTES=15`).
4. **`/billing/verify` grants any entitlement to anyone.** I confirmed it live: a fake receipt returned `coachTier=elite`, `expiresAt=2099`.
5. **A production DB would have an empty catalog.** Programs, exercises, and recipes exist only in the local demo seed, and there are 0 coach sessions anywhere.

Estimated completion toward the MSV (section 9): **about 60%.** Most of the product code exists. What's left is a handful of P0 fixes, release plumbing, store/account setup, infra, and content.

---

## 1. Phase 1: does it still build after a year?

### 1.1 Toolchain found on this machine

| Tool | Found | Notes |
|---|---|---|
| Go | go1.26.0 darwin/arm64 | `go.mod` says `go 1.24.0`; CI and Dockerfile use 1.24 |
| Node | v20.20.2 default; v22.21.0 under `~/.nvm` | `atlas-mobile/package.json` requires `>= 22.11.0`, so I ran mobile checks with 22.21.0 |
| Docker | 28.5.1 (Docker Desktop) | Image pulls go through a configured mirror, `docker.arvancloud.ir` |
| psql | Homebrew postgresql@17 | A **Homebrew `postgresql@17` service is listening on 127.0.0.1:5432** and shadows the compose Postgres (see 1.2) |
| Java | 1.8.0_503 only | Gradle 9.0.0 wrapper (`atlas-mobile/android/gradle/wrapper/gradle-wrapper.properties`) needs JDK 17+ |
| Android SDK / adb | **not installed** | `ANDROID_HOME` unset, `~/Library/Android/sdk` missing |
| Xcode | **not installed** (Command Line Tools only) | `xcodebuild` unavailable |
| CocoaPods | 1.17.0 | Not run (needs Xcode) |
| Unity | **not installed** | Project targets Unity 6000.3.10f1 (`atlas-unity/ProjectSettings/ProjectVersion.txt`) |
| kubectl | 1.36.1 | `kubectl kustomize infra/staging/k8s` renders OK |
| sqlc / goose / oapi-codegen | not installed globally | The Makefile runs them via `go run` or Docker, which works |

### 1.2 Commands and results

| # | Command (run in the scratch clone) | Result |
|---|---|---|
| 1 | `make dev-up` | **FAIL (environment).** `minio/minio:RELEASE.2024-07-16T23-46-41Z`: "error from registry: unknown", and the mirror `docker.arvancloud.ir` returns manifest not found. I didn't verify whether Docker Hub itself still serves this tag. Postgres, Redis, and MailHog started fine individually: `docker compose -f infra/local/docker-compose.yml up -d postgres mailhog redis`. MailHog's image is amd64-only and runs under emulation on arm64. |
| 2 | `make api-migrate-up` | **FAIL (environment), then FAIL (code).** First, Homebrew Postgres answers on localhost:5432 (`role "atlas" does not exist`). I worked around it with `POSTGRES_URL=postgres://atlas:atlas@<LAN-IP>:5432/...`. Then: `ERROR 20260227070000_exercise_substitution_metadata.sql: ... unterminated dollar-quoted string`. **The same failure happened in GitHub CI** (run 28324207541, job "API Tests (Go)", step "Run database migrations"). Affected: `atlas-api/migrations/20260227070000_exercise_substitution_metadata.sql`, `20260228040000_exercise_substitution_taxonomy.sql`, `20260228080000_monetization_subscriptions_v1.sql` (its `END\n$$;` form also needs the marker). |
| 2b | Same command, after adding `StatementBegin/End` in scratch | **PASS.** Migrated to version `20260228090000`. `goose reset` then `up` also passes, so all Down migrations are reversible. |
| 3 | `make api-seed` (psql with `ON_ERROR_STOP=1` on a fresh DB) | **PASS** after migrations: 2 programs, 6 program sessions, 6 recipes, 1 demo user (`demo@atlas.local`), 0 coach sessions. Without `ON_ERROR_STOP` the Makefile target hides partial failures, and a DB that `go test` has truncated fails on the `muscle_groups` FK. |
| 4 | `make api-seed-exercises` | **PASS.** `rows_processed: 30`. Note that `atlas-api/seed/exercises.csv` and `atlas-api/seeds/exercises.csv` exist and differ. |
| 5 | `make generate` (with Node 22 on PATH) | **PASS.** oapi-codegen v2.4.1, sqlc 1.27.0 via Docker, openapi-typescript 7.10.1. |
| 6 | `make verify-generated` | **PASS.** No drift between the spec and the committed Go/TS/sqlc code. |
| 7 | `cd atlas-api && go build ./...` | **PASS** |
| 8 | `cd atlas-api && go vet ./...` | **PASS** |
| 9 | `cd atlas-api && ATLAS_TEST_POSTGRES_URL=... go test ./...` | **FAIL, 1 test.** `TestDashboardSummaryIntegration` (`atlas-api/internal/httpapi/router_integration_test.go:1571`): expected 1260 kg in the current-week bucket, got 360. Cause: fixtures at `now-2d` and `now-1d` (`router_integration_test.go` in `seedAnalyticsWorkoutData`) straddle `DATE_TRUNC('week')` (`atlas-api/internal/db/queries/analytics.sql:149`) on Mondays and Tuesdays (today is a Tuesday). It's a flaky test, not a product bug. All other packages pass. The S3/MinIO integration test skipped because MinIO wasn't running. |
| 10 | `gofmt -l atlas-api` | 3 unformatted files: `internal/entitlement/service.go`, `internal/httpapi/billing_handlers.go`, `internal/httpapi/router_integration_test.go` |
| 11 | Boot the API binary (`APP_ENV=local`, migrated DB) | **FAIL.** `FATAL failed initializing OpenTelemetry {"error": "conflicting Schema URL: https://opentelemetry.io/schemas/1.39.0 and https://opentelemetry.io/schemas/1.26.0"}`. It boots after changing `semconv/v1.26.0` to `semconv/v1.39.0` and `DeploymentEnvironment` to `DeploymentEnvironmentName` (scratch only). |
| 12 | Smoke test on the patched binary | `/health` 200 · register 200 · `/me` 200 · `/auth/refresh` 200 · `/programs` → `{"programs":[]}` on a DB that `go test` had truncated · `/exercises/{id}/biomechanics` without a token → 401 · `/billing/verify` with `receiptToken:"anything"` and `expiresAt:2099-01-01` → `isPro=true`, `coachTier=elite`, `expiresAt=2098-12-31` |
| 13 | `cd atlas-mobile && npm ci` (Node 22) | **PASS** in 25 s. Warns that 73 vulnerabilities exist (see 2.3). Under Node 20 it shows an EBADENGINE warning. |
| 14 | `npm run lint` | **PASS.** 0 errors, 3 warnings (`no-shadow` of `React` in 3 integration tests) |
| 15 | `npm run typecheck` | **PASS** |
| 16 | `npm test -- --watchAll=false` | **PASS.** 23 suites, 69 tests. One warning: "worker process has failed to exit gracefully", so a test leaks timers. |
| 17 | Android `./gradlew assembleDebug` | **NOT RUN.** No Android SDK and only JDK 8 on this machine. **Unverified.** |
| 18 | iOS `pod install` / `xcodebuild` | **NOT RUN.** No Xcode. There's no `Podfile.lock` or `.xcworkspace` in the repo, so pods have never been committed. **Unverified.** |
| 19 | Unity export | **NOT RUN.** Unity isn't installed. The project has no scenes or assets (see N16). |
| 20 | `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` | "Your code is affected by **34 vulnerabilities from 7 modules and the Go standard library**." See 2.2. |
| 21 | Last GitHub CI runs (`gh run list`) | Both runs (2026-06-28) **failed**. Generated Artifacts ✓, Mobile Checks ✓, API Tests ✗ (migrations), Staging Deploy skipped. |

---

## 2. Outdated or deprecated toolchains and dependencies

### 2.1 Toolchains

| Item | Repo pin | Current (2026-10) | Note |
|---|---|---|---|
| Go | `go 1.24.0` (`atlas-api/go.mod`), `1.24.x` (`.github/workflows/ci.yml`), `golang:1.24-alpine` (`atlas-api/Dockerfile`) | 1.26.x | With 1.26 released, 1.24 is outside Go's two-release support window, so it gets no more security fixes. govulncheck lists stdlib vulns fixed only in 1.26.6. |
| Node | engines `>=22.11`, CI Node 22 | — | GitHub annotates `actions/checkout@v4`, `setup-go@v5`, `setup-node@v4` as "Node.js 20 is deprecated" on runners. |
| React Native | 0.84.0 | 0.87.1 per `npm outdated` | 3 minors behind. Whether 0.84 still meets current App Store and Play build-SDK requirements is unverified; check before submission. |
| Gradle | 9.0.0 wrapper; AGP via RN plugin | — | compileSdk/targetSdk 36, minSdk 24, NDK 27.1.12297006 (`atlas-mobile/android/build.gradle:3-8`). targetSdk 36 should satisfy Play's 2026 target-API rule (verify at submission). |
| Xcode / CocoaPods | iOS deployment target 15.1; Gemfile pins `xcodeproj < 1.26.0`, `concurrent-ruby < 1.3.4` | — | Unverified whether these pins work with the current Xcode SDK required by App Store Connect. |
| Unity | 6000.3.10f1 | — | Unverified (not installed). |
| ESLint | 8.57.1 | 10.x | ESLint 8 is end-of-life ("no longer supported" warning in `npm ci`). |
| OTel collector | `otel/opentelemetry-collector-contrib:0.102.1` (`infra/staging/otel-collector/deployment.yaml`) | — | Old, but not a blocker. |
| Argo CD CLI in CI | v2.12.6 (`.github/workflows/ci.yml`) | — | Old. |

### 2.2 Go modules: security-relevant (govulncheck, reachable)

`github.com/golang-jwt/jwt/v5 v5.2.1` (update to 5.3.1), `github.com/jackc/pgx/v5 v5.7.1` (to 5.11.0), `github.com/go-chi/chi/v5 v5.2.3` (to 5.3.2), `go.opentelemetry.io/otel* v1.40.0` (GO-2026-6505, fixed in 1.45.0), `google.golang.org/grpc v1.78.0` (GO-2026-6348 fixed 1.83.1; GO-2026-6061 fixed 1.82.1), `golang.org/x/net v0.49.0`, `golang.org/x/text v0.33.0`, plus stdlib (`net/http`, `crypto/tls`, `crypto/x509`, `net/url`, `html/template`, `encoding/asn1`, `encoding/xml`) fixed in go1.26.6. Other outdated modules: oapi-codegen 2.4.1 (2.8.0 available), goose 3.24.1 (3.28.0), minio-go 7.0.81 (7.3.0), x/crypto 0.47.0 (0.57.0).

### 2.3 npm: security-relevant

- `npm audit` for the full tree: 73 (1 critical, 59 high, 11 moderate, 2 low).
- `npm audit --omit=dev`: 48 (1 critical `shell-quote`, 34 high). Nearly all of these are in build/dev-server tooling (metro, `@react-native-community/cli`, jest, `ws`, `compression`, `fast-xml-parser`, `image-size`), not code that ships in the JS bundle. My judgment is low runtime risk; they're fixed by the RN upgrade, which I've deferred.
- Deprecated packages: `eslint@8`, `glob@7`, `rimraf@3`, `inflight`.
- Behind on minors or majors: `react-native-iap` 14.7.12 (16.7.2 available), `react-native-gesture-handler` 2.30 (3.3), `react-native-camera-kit` 17 (18), TypeScript 5.9 (7.0), jest 29 (30).

---

## 3. Verdicts on last year's findings

| # | Known finding | Verdict | Evidence |
|---|---|---|---|
| M1 | API base URL hardcoded to 10.0.2.2/localhost, no env config | **CONFIRMED** | `atlas-mobile/src/api/client.ts:5-11` (`http://10.0.2.2:8080` / `http://localhost:8080`, no env/config lib in `package.json`). `biomechanicsService.ts:91` also builds URLs from `API_BASE_URL` with raw `fetch`. |
| M2 | Android release uses debug keystore; iOS bundle id `org.reactjs.native.example…` | **CONFIRMED** | `atlas-mobile/android/app/build.gradle:89-105` (release → `signingConfigs.debug`, `debug.keystore` is committed); `applicationId "com.atlasmobile"` at line 82, `versionCode 1`. `atlas-mobile/ios/AtlasMobile.xcodeproj/project.pbxproj:274,303` `PRODUCT_BUNDLE_IDENTIFIER = "org.reactjs.native.example.$(PRODUCT_NAME:rfc1034identifier)"`, no `DEVELOPMENT_TEAM`. |
| M3 | FormCheck native modules generate synthetic frames | **CONFIRMED** | Android `atlas-mobile/android/app/src/main/java/com/atlasmobile/formcheck/FormCheckPoseModule.kt:48,99-101` (`buildSyntheticFrame`, `sin(phase)`); iOS `atlas-mobile/ios/AtlasFormCheckPoseBridge/ios/FormCheckPoseModule.m:86,121-128` (`emitSyntheticFrame`). No ML Kit or Vision usage. |
| M4 | Mock mode, `mockData.ts`, `mvpService.ts`: which screens depend on mocks? | **CHANGED.** No release screen depends on mocks. | Mock mode is hard-gated: `CAN_USE_MOCK_MODE = __DEV__` (`src/state/MockModeContext.tsx:19`), and nothing in `src/` calls `setMockMode`/`toggleMockMode`, so it can only be switched on through AsyncStorage key `atlas.mobile.mock_mode` in dev builds. Services with mock branches (all bypassed in release): `authService`, `onboardingService`, `programsService`, `workoutService`, `consentService`, `biomechanicsService`, `analytics/eventClient`. Screens that pass `isMockMode`: onboarding Goals/Equipment/Schedule/MomentumSprint, PrivacySettings, FormCheck, WorkoutRunner. **`src/api/services/mvpService.ts` is dead code** (imported nowhere). Mock data still ships in the release JS bundle. |
| M5 | No push notifications, no deep-link handling | **CONFIRMED** | No `Linking`, no `linking=` on `NavigationContainer` (`src/navigation/RootNavigator.tsx:29`), no messaging/notification libraries in `atlas-mobile/package.json`, no intent filters other than LAUNCHER in `AndroidManifest.xml`. |
| M6 | CI runs mobile lint/typecheck/tests but never builds a release binary | **CONFIRMED** | `.github/workflows/ci.yml` job `mobile-checks`: lint, typecheck, test only. No Gradle or Xcode step anywhere. |
| M7 | Very large screens | **CONFIRMED** | `WorkoutRunnerScreen.tsx` 1092 lines, `DashboardScreen.tsx` 881, `FormCheckScreen.tsx` 614; also `workoutService.ts` 1104. |
| A1 | `/billing/verify` accepts client receipt data without store verification | **CONFIRMED, and worse than noted** | `atlas-api/internal/httpapi/billing_handlers.go:23-133`. The request is checked only for non-empty fields, then `UpsertSubscription` runs with `status=active`. **Expiry comes from the client** (`resolveSubscriptionExpiry`, lines 54 and 170-178), and the client sends it (`PaywallScreen.tsx:169,187`). Verified live: fake receipt produced elite until 2099. |
| A2 | Multi-step writes without transactions | **CONFIRMED, broader** | There's no `BeginTx`/`WithTx` call anywhere in `atlas-api/internal` (only the sqlc-generated `WithTx`). Multi-write handlers: `PostCrews` (`community_handlers.go:51,76,89`), `PostCrewsJoin` (`:223`, AddCrewMember happens **before** the guarded `IncrementCrewInviteUse`, so an exhausted invite still adds the member), `PostWorkoutsStart` (`workout_handlers.go:32`), `PostMomentumSprintEnroll` (`momentum_sprint_handlers.go:61`), `PostNutritionWeeklyCheckin`, `PostNutritionMealPlanGenerate` (`nutrition_planning_handlers.go:200`), `PutNutritionMealPlan` (`:392`, delete-then-insert items), `PostHabitsIdToggleToday`. Billing is a single upsert, so it's fine. |
| A3 | Auth policy split across router allowlist, OpenAPI security, entitlement middleware | **CONFIRMED** | Allowlist `atlas-api/internal/httpapi/router.go:16-26` includes the wildcard `/api/v1/exercises/*`, which also covers `GetExerciseBiomechanicsById`. The spec marks that op `bearerAuth`, so only the entitlement middleware (`entitlement_middleware.go:52-79`) returns the 401. Entitlement mapping is a hand-written `switch` on operationId, plus three more switches for response types. There's no test asserting the allowlist matches spec `security`. |
| A4 | REDIS_ADDR / MAILHOG_SMTP_ADDR configured but unused; README env mismatch | **CONFIRMED, plus more** | `atlas-api/internal/config/config.go:74,76` reads them, nothing else references `cfg.RedisAddr`/`cfg.MailhogSMTP`, and **`REDIS_ADDR` is required in staging/prod** (`required=true`), so deploys must provision an unused Redis. `PRO_ALL_USERS`/`PRO_USER_EMAILS` (`config.go:86-87`) are also unused. `atlas-api/README.md` says `APP_ENV=development` loads `.env`, but `development` is rejected by `config.go:62-64` (valid values: local, staging, prod). The README endpoint list covers 30 of 65 ops and says "Go 1.22+". |
| A5 | Unbounded in-memory food caches; USDA/Edamam config | **CONFIRMED** | `atlas-api/internal/food/service.go:38-40` (`searchCache`, `upcCache` maps) and `usda_provider.go:32` (`detailsByID`). Entries are evicted only when an expired key is read again (`service.go:534,574`, `usda_provider.go:221`), with no size cap. **Config:** USDA needs `USDA_API_KEY` (an api.data.gov key; `DEMO_KEY` is rejected outside local, `config.go:132`) and `USDA_API_BASE_URL`. Edamam needs `EDAMAM_APP_ID` and `EDAMAM_APP_KEY` (**required** when `APP_ENV!=local`, `config.go:80-81`), used for UPC lookup at `/api/food-database/v2/parser` (`edamam_provider.go:75-82`). Also `FOOD_DETAILS_CACHE_TTL_MINUTES`. |
| I1 | Staging manifests contain placeholders | **CONFIRMED** | `infra/staging/k8s/kustomization.yaml:15` (`ghcr.io/your-org/atlas-api`), `atlas-api-ingress.yaml:11` (`api.staging.atlas.example.com`), `argocd/atlas-api-staging-application.yaml:10` (`github.com/your-org/atlas.git`), `infra/staging/docker-compose.yml:3`. CI rewrites `newName` at deploy time, but the Argo repoURL and ingress host are never rewritten. |
| I2 | OTel collector exports traces only to debug; no prod overlay | **CONFIRMED** | `infra/staging/otel-collector/config.yaml` traces → `[debug]`, metrics → `[debug, prometheus]`. Only `infra/staging/` exists, with no base/overlay structure. Also, the CI check greps `otelcol_receiver_accepted_spans` from `STAGING_OTEL_COLLECTOR_METRICS_URL`, but the collector only exposes the prometheus *exporter* on 8889. Internal `otelcol_*` metrics are normally on 8888, which isn't exposed, so this step likely always fails (unverified). |
| I3 | No `.git` at root; noise files | **FIXED / CHANGED** | Root is a git repo with GitHub remote `danielwellz/Atlas`, and `make verify-generated` works. Noise: `.DS_Store` files and `.gradle/wrapper/dists/gradle-9.0.0-bin/.../gradle-9.0.0-bin.zip.part` still exist on disk but are gitignored and **not tracked** (`git ls-files` shows none). |
| I4 | Anatomy schema duplicated by hand across docs, TS, C# | **CONFIRMED** | `docs/anatomy-engine-schema-v1.md`, `atlas-mobile/src/native/anatomyEngineBridge.ts`, `atlas-unity/Assets/Scripts/AnatomyEngineMessageSchema.cs`: three hand-written copies of topic `anatomy.engine.v1`, no generator. |

### 3.1 New findings not in last year's list

| # | Finding | Evidence |
|---|---|---|
| N1 | **The API exits at startup** because of an OTel semconv schema conflict | Section 1.2 rows 11-12; `atlas-api/internal/observability/otel.go:17` |
| N2 | **Migrations fail on a fresh DB** (goose annotations missing) | Section 1.2 row 2; CI run 28324207541 |
| N3 | **Mobile never refreshes tokens.** On cold start, `/me` fails after 15 minutes, then `clearTokens()` runs and the user is logged out | No `/auth/refresh` call in `atlas-mobile/src`; `src/state/AuthContext.tsx:76-86`; `atlas-api/.env.example` `ACCESS_TOKEN_TTL_MINUTES=15` |
| N4 | **Prod would have no catalog.** Programs, exercises, recipes, and biomech assets exist only in `seeds/001_local_seed.sql`, which also creates demo user `demo@atlas.local`. There are no coach sessions anywhere. Meal plan returns 400 "no recipes are available" | `atlas-api/seeds/001_local_seed.sql`; `nutrition_planning_handlers.go:223-229`; no `INSERT INTO programs` in `atlas-api/migrations/` |
| N5 | **No account deletion or data export** (App Store guideline 5.1.1(v); Play account-deletion policy) | No such ops in `atlas-api/openapi/openapi.yaml`; `docs/compliance.md:40-41` unchecked. All user FKs are `ON DELETE CASCADE` except one `SET NULL`, so deletion is cheap to build. |
| N6 | Free users have no way to set nutrition targets | `PUT /api/v1/nutrition/targets` is never called by mobile. Targets are only written by the Pro weekly check-in (`nutrition_planning_handlers.go:128`). |
| N7 | Habits can't be created in the app | `POST /api/v1/habits` is never called, and nothing on the server auto-creates habits, so the Dashboard "Habits" list (`DashboardScreen.tsx:609-620`) stays empty for real users |
| N8 | User-imported recipes go into the **global** `recipes` table and appear in every user's meal plans; the insert is raw SQL that bypasses sqlc | `nutrition_planning_handlers.go:591-640`; the `recipes` table has no owner column (`migrations/20260227110000_weekly_checkins_and_meal_planning.sql:18`) |
| N9 | Offline outbox retries permanent 4xx failures forever (poison items), isn't scoped per user, and isn't cleared on logout. Workout start and complete need a connection. | `atlas-mobile/src/sync/outbox.ts:21,436-447`; `clearOutbox` (`:213`) is never called; `src/features/workout/hooks.ts:137` (only add_set is enqueued) |
| N10 | No rate limiting, no request body limit, no write/idle timeouts; `/api/v1/events` accepts anonymous writes | `router.go:28-45`; `cmd/atlas-api/main.go:68-72` (only `ReadHeaderTimeout`) |
| N11 | Integration tests `TRUNCATE` every table in whatever DB `ATLAS_TEST_POSTGRES_URL` points to, including migration-seeded `muscle_groups`. Running tests against the dev DB wipes it and breaks `make api-seed`. | `router_integration_test.go:2729` |
| N12 | No crash reporting, no React error boundary | No Sentry, Crashlytics, or Bugsnag in `package.json`; no `ErrorBoundary` in `src/` |
| N13 | 13 tab routes in one bottom-tab navigator; 9 are visible tabs, including deferred features (Anatomy, Crew) | `src/navigation/MainTabsNavigator.tsx` |
| N14 | Store metadata isn't ready: app name `AtlasMobile`; Android uses the default robot launcher icon; the iOS AppIcon set has no images; `NSLocationWhenInUseUsageDescription` is an empty string; the privacy manifest declares **no collected data types** even though the app collects email, fitness, and health-adjacent data; the camera string mentions only barcodes | `android/app/src/main/res/values/strings.xml`, `mipmap-*/ic_launcher.png`, `ios/AtlasMobile/Images.xcassets/AppIcon.appiconset/Contents.json` (0 filenames), `ios/AtlasMobile/Info.plist`, `ios/AtlasMobile/PrivacyInfo.xcprivacy` |
| N15 | No migration step in deployment (no Job/hook, no goose in the image); Dockerfile hardcodes `GOARCH=amd64`; no TLS on ingress | `infra/staging/k8s/*`, `atlas-api/Dockerfile`, `infra/staging/k8s/managed-dependencies.md:11` |
| N16 | Unity project is a skeleton: no scenes (`Assets/Scenes/.gitkeep`), no models or rig, empty `Builds/android` and `Builds/ios`. Native bridges degrade to a "fallback" state, so the app doesn't crash. | `git ls-files atlas-unity`; `UnityBridgeRuntime.kt:43`; `UnityBridgeModule.m:171` |
| N17 | No password reset or email verification flow, and no email provider integration | `openapi.yaml` has no such ops |
| N18 | CI hygiene: `setup-go` cache can't find `go.sum` at the repo root (annotation "Restore cache failed"); deprecated Node 20 actions | CI run 28324207541 annotations |

---

## 4. Contract diff (OpenAPI vs Go vs mobile)

The spec (`atlas-api/openapi/openapi.yaml`) has **65 operations**.

**Defined but unimplemented in Go: none.** `generated.NewStrictHandler(apiServer, …)` (`router.go:42`) requires `*Server` to satisfy all 65 `StrictServerInterface` methods, and `go build` passes. Being implemented doesn't mean being correct; see the stubs in section 5.

**Implemented but unused by mobile (10):**

| Operation | Assessment |
|---|---|
| `GET /health` (GetHealth) | Infra and smoke only. Fine. |
| `POST /auth/refresh` (PostAuthRefresh) | **BLOCKER**, see N3 |
| `PUT /nutrition/targets` (PutNutritionTargets) | **Gap.** Free users can't set targets (N6). |
| `POST /habits` (PostHabits) | **Gap.** No habit creation (N7). |
| `GET /habits/streaks` (GetHabitsStreaks) | Unused. Nice-to-have. |
| `POST /nutrition/weight` (PostNutritionWeightEntry) | Mobile uses the `PUT` upsert instead. Redundant. |
| `GET /exercises`, `GET /exercises/{id}` | No exercise library screen. Fine for the MSV. |
| `PUT /profile`, `PUT /goals` | Superseded by `PUT /onboarding/profile`. Legacy. |

**Called by mobile but missing in the spec or Go: none.** Every call goes through the typed `atlasApiClient` (typecheck passes) except `biomechanicsService.ts:91`, which uses raw `fetch` to an existing path.

**Missing from the contract but needed for shipping:** account deletion, data export, password reset/forgot password, store server-notification webhooks (App Store Server Notifications v2 / Play RTDN), and push-token registration (deferred).

---

## 5. Journey traces

Legend: **REAL** = wired to the live API/DB and exercised by tests or my smoke run. **PARTIAL** = works with a material gap. **MOCKED** = fake data on a production path. **STUBBED** = an endpoint that exists but doesn't do the real work. **MISSING** = absent.

| Journey | Screen | Hook / state | Service | Endpoint | Handler | SQL | Verdict and gap |
|---|---|---|---|---|---|---|---|
| Register | `screens/auth/RegisterScreen.tsx` REAL | `features/auth/hooks.ts` REAL | `api/services/authService.ts` REAL | POST `/auth/register` REAL | `server.go` PostAuthRegister REAL | `users.sql`, `sessions.sql` REAL | **REAL** (smoke 200) |
| Login | `LoginScreen.tsx` REAL | same | same | POST `/auth/login` REAL | PostAuthLogin REAL | same | **REAL** |
| Refresh | — **MISSING** | `state/AuthContext.tsx` has no refresh **MISSING** | — | POST `/auth/refresh` REAL (smoke 200; rotates by revoking the old session) | PostAuthRefresh REAL | `sessions.sql` REAL | **PARTIAL → BLOCKER.** API works; mobile never calls it (N3). |
| Logout | `DashboardScreen.tsx:715` REAL | AuthContext REAL | authService REAL | POST `/auth/logout` REAL | REAL | REAL | **REAL** (outbox not cleared, N9) |
| Onboarding (goals → readiness) | `screens/onboarding/*` REAL | `state/OnboardingContext.tsx` REAL | `onboardingService.ts` REAL | PUT `/onboarding/profile`, GET `/onboarding/status`, `/onboarding/plan` REAL | `server.go` PutOnboardingProfile / GetOnboardingPlan REAL | `onboarding.sql` REAL | **REAL** |
| Momentum sprint enroll | `MomentumSprintEnrollmentScreen.tsx` REAL | — | `habitService.enrollMomentumSprint` REAL | POST `/momentum-sprint/enroll` REAL | `momentum_sprint_handlers.go:61` PARTIAL (3 writes, no tx) | `momentum_sprint.sql` REAL | **REAL** (robustness gap) |
| Programs list / enroll | `ProgramsScreen.tsx` REAL | `features/programs/hooks.ts` REAL | `programsService.ts` REAL | GET `/programs`, POST `/programs/enroll`, GET `/programs/current[/sessions]` REAL | REAL | `programs.sql` REAL | **PARTIAL.** Catalog is empty in a fresh DB (N4); 2 programs in the demo seed. |
| Run workout | `WorkoutRunnerScreen.tsx` REAL | `features/workout/hooks.ts` REAL | `workoutService.startWorkout` REAL | POST `/workouts/start` REAL | `workout_handlers.go:32` PARTIAL (2 writes, no tx) | `workouts.sql` REAL | **PARTIAL.** Start needs a connection. |
| Log sets offline then sync | WorkoutRunner REAL | `hooks.ts:137` enqueues when offline REAL | `sync/outbox.ts` + `OutboxSyncController.tsx` PARTIAL | POST `/workouts/{id}/add_set` with idempotency key REAL | REAL | `workout_sets` + idempotency migration REAL | **PARTIAL.** Breaks after token expiry (N3); poison items retried forever; not user-scoped (N9). |
| Complete workout / dashboard | WorkoutRunner / Dashboard REAL | REAL | REAL | POST `/complete`, GET `/dashboard/summary` REAL | REAL | `analytics.sql` REAL | **REAL** (complete needs a connection) |
| Nutrition targets | — **MISSING** for free users | — | `nutritionService` has no targets call | PUT `/nutrition/targets` REAL (unused) | REAL | REAL | **MISSING (UI)** (N6) |
| Food search | `FoodScreen.tsx` REAL | — | `foodService.ts:61` REAL | GET `/foods/search` REAL | `food_handlers.go` REAL | `foods.sql` + USDA provider REAL | **REAL** (needs a real USDA key; unbounded cache) |
| Barcode | `BarcodeScanScreen.tsx` (react-native-camera-kit) REAL | entitlement check REAL | `foodService.ts:106` REAL | GET `/foods/upc/{code}` [barcode_scan] REAL | REAL | Edamam provider REAL | **REAL code; live Edamam unverified** (no keys); Pro-gated |
| Food logging | Food/Barcode screens REAL | `enqueueFoodLogOutboxItem` REAL | outbox → `foodService.ts:139` REAL | POST `/food-logs` REAL | REAL | REAL | **REAL** (outbox caveats) |
| Weekly check-in | `WeeklyCheckInScreen.tsx` REAL | — | `nutritionService.ts:207` REAL | POST `/nutrition/weekly-checkin` [deep_nutrition] REAL | PARTIAL (2 writes, no tx) | REAL | **REAL**, Pro |
| Meal plan | `MealPlanScreen.tsx` REAL | — | `nutritionService.ts:249-338` REAL | generate / GET / PUT / DELETE REAL | PARTIAL (4 writes, no tx) | `meal_plans.sql` REAL | **PARTIAL.** Returns 400 without recipe content (N4); imported recipes leak globally (N8). |
| Habits | `DashboardScreen.tsx:609` REAL | `features/dashboard/hooks.ts:142` REAL | `habitService.ts:55,75` REAL | GET `/habits`, POST `/toggle_today` REAL | REAL | `habits.sql` REAL | **PARTIAL.** No create UI (N7). |
| Crews + invites | `CrewScreen.tsx` REAL | `features/community/hooks.ts` REAL | `communityService.ts` REAL | `/crews*` REAL | `community_handlers.go` PARTIAL (no tx; join over-use) | `community.sql` REAL | **REAL with bugs.** No UGC reporting/moderation. |
| Coach sessions | `CrewScreen` / `CoachSessionPlayerScreen.tsx` REAL | REAL | REAL | `/coach-sessions*` REAL | REAL | REAL | **PARTIAL.** 0 sessions exist anywhere (no content). |
| Paywall → purchase → unlock | `PaywallScreen.tsx` REAL | `features/billing/iap.ts` (react-native-iap 14) REAL; store products unverified | `billingService.ts` REAL | POST `/billing/verify` **STUBBED** (trusts client) | `billing_handlers.go` **STUBBED** | `subscriptions` + `user_entitlements` view REAL | **STUBBED** at verification; entitlement unlock via `/me` is REAL |
| Consent / privacy | `PrivacySettingsScreen.tsx` REAL | — | `consentService.ts` REAL | `/consents*` REAL | REAL | `consents.sql` REAL | **REAL** |
| Data export / delete | — | — | — | — | — | — | **MISSING** (N5) |
| Form-check upload | `FormCheckScreen.tsx` REAL | `native/formCheckPose.ts` → native module **MOCKED** (synthetic frames) | `formCheckService.ts` REAL | POST `/form-check/uploads` [entitlement + consent] REAL | REAL (stores metadata) | `form_check_uploads.sql` REAL | **MOCKED** at pose detection |
| Anatomy / Unity open | `AnatomyScreen.tsx` REAL | `native/unityBridge.ts` REAL | `anatomyEngineBridge.ts` REAL | GET `/exercises/{id}/biomechanics` REAL | REAL | biomech assets: seed only | **MISSING** Unity content (N16); bridge falls back gracefully |

---

## 6. Grep sweep (TODO, placeholders, skips, dev flags)

- `TODO|FIXME|HACK|XXX|not implemented`: **no hits in source.** The only `xxxxx` hits are DSN redaction in `cmd/atlas-api/main.go`.
- Placeholders: `your-org` (kustomization, Argo app, staging compose), `atlas.example.com` (ingress, smoke usage text), `https://atlas.local/mobile` and `mobile@atlas.local` in iOS podspecs, and `<…>` values in `infra/staging/k8s/atlas-api-staging-secrets.example.yaml` (an example file, as expected).
- Skipped tests: `router_integration_test.go:2705` skips when Postgres is unavailable (CI turns that into an error, which is good). `storage/s3_storage_integration_test.go:34` skips without MinIO, and CI has no MinIO service, so it **always skips in CI**. No `.only`, `it.skip`, or `xit` in mobile tests.
- Dev-only flags that could leak into production:
  - `__DEV__` mock mode is safe in release.
  - `JWT_SECRET` default and `USDA DEMO_KEY` are rejected outside local (`config.go:128-134`), which is good.
  - **`.env` is auto-loaded whenever `APP_ENV` is unset or `local`** (`config.go:280-291`). An unset `APP_ENV` in prod silently falls back to local defaults: dev JWT secret, `sslmode=disable`. The k8s manifest sets `APP_ENV=staging`, but this fallback is risky.
  - Android `debug.keystore` used for release.
  - `NSAllowsLocalNetworking=true` in iOS `Info.plist`.
  - The local seed creates a demo user with a known password hash. It must never run in prod.

---

## 7. Phase 3: production-readiness checklist

| Area | State | Evidence |
|---|---|---|
| Mobile env config | **Missing** | `src/api/client.ts:5-11` |
| Release signing | **Missing** (debug keystore; no iOS team) | `android/app/build.gradle:89-105`; `project.pbxproj` |
| Bundle IDs | **Placeholder** (`com.atlasmobile`, `org.reactjs.native.example.*`) | same |
| App icons and splash | **Missing** (default robot icon; no iOS icons; default `LaunchScreen.storyboard`) | N14 |
| iOS privacy manifest | **Inaccurate** (no collected data types) | `PrivacyInfo.xcprivacy` |
| Permission strings | Camera OK; empty location string | `Info.plist` |
| Account deletion | **Missing** | N5 |
| IAP products | Client IDs `atlas.{pro,elite}.{monthly,yearly}` (`features/billing/iap.ts:14-19`) match DB mapping (`migrations/20260228080000_…:41-61`); store-side products unverified | — |
| Crash reporting | **Missing** | N12 |
| Push / deep links | **Missing** | M5 |
| API secrets | Via env and k8s Secret; example provided | `infra/staging/k8s/atlas-api-staging-secrets.example.yaml` |
| JWT | HS256 with a shared secret; 15 m access / 30 d refresh; refresh rotation via session revoke. No minimum secret-length check. | `internal/auth/token_service.go`, `config.go` |
| CORS | None. Not needed for a native-only client; required only if a web client appears. | `router.go` |
| Rate limiting | **Missing** | N10 |
| Transactions | **Missing** | A2 |
| Receipt verification | **Stubbed** | A1 |
| Backups / migrations in prod | No migration step; backups described in a doc only | N15; `infra/staging/k8s/managed-dependencies.md` |
| Prod overlay | **Missing** | I2 |
| Real hostnames / TLS | **Placeholder / missing** | I1 |
| Durable traces and logs | **Missing** (debug exporter; logs to stdout only) | I2 |

---

## 8. Gap register

Categories: BLOCKER, IMPORTANT, NICE-TO-HAVE, DEFERRED. Tags: AGENT-DOABLE or NEEDS-HUMAN; "AGENT+HUMAN" means an agent can do the code and a human must supply accounts, keys, or decisions. Size applies to agent work. The Task column refers to `EXECUTION_PLAN.md`.

| ID | Gap | Cat. | Tag | Size | Evidence | Task |
|---|---|---|---|---|---|---|
| G01 | API exits at startup (OTel semconv conflict) | BLOCKER | AGENT | S | `internal/observability/otel.go:17,25-31` | T01 |
| G02 | 3 migrations fail on a fresh DB | BLOCKER | AGENT | S | §1.2 row 2; CI run 28324207541 | T01 |
| G03 | Day-of-week flaky dashboard test; tests truncate the dev DB | IMPORTANT | AGENT | S | `router_integration_test.go:1522-1571,2729` | T01 |
| G04 | Mobile never refreshes tokens (logout after 15 min) | BLOCKER | AGENT | M | N3 | T10 |
| G05 | Mobile API URL hardcoded to dev http hosts | BLOCKER | AGENT | M | `src/api/client.ts:5-11` | T09 |
| G06 | `/billing/verify` trusts client receipt and expiry | BLOCKER (if paywall is in the MSV) | AGENT+HUMAN (store API credentials) | L | `billing_handlers.go:23-178` | T15 |
| G07 | No account deletion or data export | BLOCKER | AGENT | M (API) + S (UI) | N5 | T06, T14 |
| G08 | Android: debug-keystore release signing, placeholder applicationId/version, default icon/name | BLOCKER | AGENT+HUMAN (keystore, final ID, icon) | M | `android/app/build.gradle:82-105` | T17 |
| G09 | iOS: placeholder bundle id, no team, no icons, inaccurate privacy manifest, empty location string | BLOCKER for iOS | AGENT+HUMAN (Apple account, Mac or CI macOS runner) | M | `project.pbxproj:274,303`; N14 | T18 |
| G10 | No production reference data (programs, exercises, recipes); demo user mixed into seed | BLOCKER | AGENT (tooling) + HUMAN (content) | M | N4 | T07 |
| G11 | Infra placeholders; no TLS, migration job, or prod overlay | BLOCKER | AGENT+HUMAN (domain, cluster, registry) | M+S | I1, I2, N15 | T08, T19 |
| G12 | No release binary build in CI | BLOCKER | AGENT | M | M6 | T17, T18 |
| G13 | No transactions in 8 multi-write handlers; crew-join over-use | IMPORTANT | AGENT | M | A2 | T03 |
| G14 | Outbox: poison retries, not user-scoped, not cleared on logout | IMPORTANT | AGENT | M | N9 | T11 |
| G15 | Auth policy drift (allowlist wildcard, hand-written switches) | IMPORTANT | AGENT | S | A3 | T04 |
| G16 | Unused or required-but-unused config; `.env` auto-load when `APP_ENV` unset; README mismatch | IMPORTANT | AGENT | S | A4, §6 | T04 |
| G17 | No rate limit, body limit, or server timeouts; anonymous `/events` | IMPORTANT | AGENT | S | N10 | T05 |
| G18 | Unbounded food caches | IMPORTANT | AGENT | S | A5 | T05 |
| G19 | Go 1.24 EOL plus 34 reachable vulns (jwt, pgx, chi, otel, grpc, x/net, stdlib) | IMPORTANT | AGENT | S | §2.2 | T02 |
| G20 | No nutrition-targets UI for free users | IMPORTANT | AGENT | S | N6 | T13 |
| G21 | No habit-creation UI | IMPORTANT | AGENT | S | N7 | T13 |
| G22 | No crash reporting or error boundary | IMPORTANT | AGENT+HUMAN (DSN) | S | N12 | T16 |
| G23 | 9 visible tabs including deferred features; no feature flags; no Settings screen | IMPORTANT | AGENT | M | N13 | T12 |
| G24 | Imported recipes are global; raw SQL bypasses sqlc | IMPORTANT | AGENT | S | N8 | T07 |
| G25 | No password reset / email provider | IMPORTANT | AGENT+HUMAN (email provider) | M | N17 | T20 (post-MSV) |
| G26 | No store server notifications (renewal, refund, cancel) | IMPORTANT (if paywall) | AGENT+HUMAN | M | — | T21 (post-MSV) |
| G27 | OTel traces debug-only; CI OTel check probably wrong | IMPORTANT | AGENT+HUMAN (backend choice) | S | I2 | T19 |
| G28 | CI hygiene (go.sum cache path, Node 20 actions); local Node 20 vs required 22 | NICE-TO-HAVE | AGENT | S | N18 | T02 |
| G29 | Client sends `expiresAt`; iOS receipt token falls back to `purchase.id` | IMPORTANT (if paywall) | AGENT | S | `features/billing/iap.ts:87-115`, `PaywallScreen.tsx:169,187` | T15 |
| G30 | Store listing, privacy policy, data-safety form, content rating | BLOCKER | NEEDS-HUMAN | — | — | Human checklist |
| G31 | `mvpService.ts` dead; mock code ships in the release bundle; `biomechanicsService` uses raw fetch | NICE-TO-HAVE | AGENT | S | M4 | T12 |
| G32 | Large screens (1092 / 881 / 614 lines) | NICE-TO-HAVE | AGENT | L | M7 | DEFERRED |
| G33 | gofmt drift (3 files); duplicate, divergent `seed/` vs `seeds/` CSV | NICE-TO-HAVE | AGENT | S | §1.2 row 10, row 4 | T01, T07 |
| G34 | Local dev friction: MinIO image pull, Homebrew PG shadowing 5432, amd64-only MailHog | NICE-TO-HAVE | AGENT | S | §1.2 | T02 |
| G35 | npm audit (48 in prod tree, mostly build tooling); RN 0.84 is 3 minors behind | IMPORTANT (pre-store check) | AGENT | L | §2.3 | DEFERRED (T22) unless store SDK rules force it |
| G36 | FormCheck pose detection is synthetic | DEFERRED | AGENT+HUMAN (real-device testing) | L | M3 | — |
| G37 | Unity anatomy has no content; schema duplicated ×3 | DEFERRED | NEEDS-HUMAN (3D assets, Unity licence) | L | N16, I4 | — |
| G38 | Push notifications | DEFERRED | AGENT+HUMAN (FCM/APNs) | M | M5 | — |
| G39 | Deep links | DEFERRED (needed only if the password reset email uses links) | AGENT | S | M5 | — |
| G40 | Community/crews/coach sessions (no content, no UGC reporting) | DEFERRED | AGENT+HUMAN | M | §5 | hidden in T12 |

---

## 9. Minimum Shippable Version: recommendation

Your default: API on staging/prod; Android (and iOS if feasible) release builds hitting the real API with no mocks; auth, onboarding, programs/workouts with offline sync, nutrition, habits, and paywall with real receipt verification; form-check and Unity deferred behind flags.

**I agree with most of it. I'd challenge four points, each with evidence:**

1. **Paywall in 1.0 is the most expensive line item for the least value.**
   - Once form-check, biomechanics overlays, and coach tiers are deferred, Pro unlocks only barcode scan (needs a paid Edamam plan; requirement at `config.go:80-81`) and weekly check-in plus meal plan (needs recipe content that doesn't exist yet; N4).
   - Real verification is the single L-sized task (T15). It also needs App Store Connect and Play Console subscription products plus server API credentials before it can be tested at all, and without store notifications (G26) renewals and refunds drift anyway.
   - **Recommendation:** ship 1.0 with every feature free and the paywall hidden (T12 flag), and turn on paywall plus T15 in 1.1.
   - If you keep it in 1.0, T15 stays a BLOCKER and adds about one L session plus your store setup time.
   - **Never ship the current `/billing/verify`.** If the paywall is hidden, the endpoint should reject requests (T04 includes this).
2. **Community, crews, and coach sessions should be hidden.** There are 0 coach sessions anywhere, and crews add user-generated content without reporting or moderation. Note that `bible.md` lists coach-led sessions as an MVP item. If that's still the plan, it's a content problem, not a code problem.
3. **iOS: this machine can't build it** (no Xcode). Do Android first. iOS stays in the MSV only if you have an Apple Developer account and either a Mac with current Xcode or a GitHub macOS runner (T18 adds a CI build).
4. **Nutrition in the MSV:** targets (new UI, T13), USDA search, logging, and weekly check-in. Barcode needs Edamam keys, so it's in or out depending on your answer to Q4.

**Proposed MSV:**
- API on a real host with TLS, migrations, and backups.
- Android release build (iOS if Q2 = yes).
- Flows: auth (with refresh and account deletion), onboarding plus momentum sprint, programs plus workout runner with offline set logging, dashboard and habits (with create), nutrition (targets, search, logging, weekly check-in; barcode and meal plan if content and keys exist), privacy (consents, export, delete), crash reporting.
- Hidden: Anatomy, FormCheck, Crew/Coach, and Paywall (if Q1 = defer).

---

## 10. NEEDS-HUMAN checklist (in this order)

1. **Answer the owner questions** (section 11), especially Q1 paywall, Q2 iOS, Q3 hosting, Q5 app identity.
2. **Choose a final app name and permanent IDs.** Play `applicationId` can never change after the first upload. Register a domain and set up DNS.
3. **Google Play Console** ($25 one-time). Create the app.
   - Generate an **upload keystore** locally and keep it in a password manager; never commit it.
   - Add it to GitHub Actions secrets as base64 keystore, store password, key alias, and key password.
   - Enrol in Play App Signing. Set up internal testing and a tester list.
4. **Apple Developer Program** ($99/yr), if iOS is in. Create the App ID for the chosen bundle id, note the Team ID, and create the App Store Connect app record.
5. **If paywall is in 1.0:**
   - Create subscription products `atlas.pro.monthly`, `atlas.pro.yearly`, `atlas.elite.monthly`, `atlas.elite.yearly` (or rename them; they must match `iap.ts:14-19` and the migration mapping) in both stores.
   - Apple: create an App Store Server API key (.p8 plus Key ID plus Issuer ID).
   - Google: create a Google Cloud service account, link it in Play Console with finance permissions, and download its JSON key.
   - Set up sandbox and licence testers.
6. **Food API keys:** a USDA FoodData Central key (free from api.data.gov). An Edamam Food Database app id and key if barcode stays (paid tiers; pricing unverified).
7. **Hosting:**
   - Managed Postgres 16 with automated backups and PITR.
   - A Kubernetes cluster with ingress-nginx, cert-manager, and Argo CD, or a simpler PaaS (Q3).
   - GHCR image visibility or an imagePullSecret.
   - Put secrets in your secret manager: `JWT_SECRET` (≥32 random bytes), `POSTGRES_URL` (sslmode=require), food keys, store credentials.
   - GitHub secrets `ARGOCD_SERVER`, `ARGOCD_AUTH_TOKEN`, `STAGING_API_BASE_URL`.
8. **Observability:** create a Sentry (or Crashlytics) project and supply the DSN. Pick a trace and log backend (Grafana Cloud, Honeycomb, etc.) and an OTLP endpoint and token.
9. **Content:**
   - Decide the program catalog (how many programs and weeks; who writes them).
   - Review the 30-exercise CSV.
   - Supply recipes if meal plans stay.
   - Supply coach-session content if community un-defers.
10. **Store listings and compliance:**
    - A 1024 px icon and splash art, screenshots, description, support URL, and **privacy policy URL**.
    - Play Data safety form and content rating; Apple App Privacy labels.
    - A demo account for app review.
    - Check whether Play's health-app declaration applies (unverified).
11. **Real-device QA** on at least one physical Android phone (and an iPhone): offline workout logging, camera barcode, sandbox purchases.
12. **Release:** upload to internal testing, then closed testing, then production.

---

## 11. Questions for the owner

1. **Paywall in 1.0, or hidden until 1.1?** (Section 9, point 1; this changes whether T15 is a blocker.)
2. **Is iOS in the MSV?** Do you have an Apple Developer account and a Mac with current Xcode, or is a GitHub macOS runner acceptable?
3. **Hosting:** do you already have a Kubernetes cluster with Argo CD, or should staging and prod move to something simpler (Fly.io, Render, Cloud Run plus managed Postgres)? The current manifests assume k8s, Argo, and GHCR.
4. **Barcode scanning:** pay for Edamam, switch the UPC provider to Open Food Facts (free; an agent can add it), or defer barcode?
5. **App identity:** final display name (currently "AtlasMobile"), Android applicationId, iOS bundle id, and domain. Has the "Atlas" name been checked for trademark and store conflicts?
6. **Content:** how many programs at launch, who authors them, and are the 30 seeded exercises correct? Are meal plans in 1.0 (they need recipes)?
7. **Pricing and tiers:** keep both Pro and Elite? Elite only adds `coach_tier_elite`, which has no content.
8. **Community and coach-led sessions:** confirm they're deferred. `bible.md` lists coach-led sessions as MVP.
9. **Password reset:** required for 1.0? It needs an email provider such as Postmark, Resend, or SES. Otherwise users who forget their password are locked out.
10. **Jurisdictions:** EU users at launch (GDPR special-category health data; `docs/compliance.md`)? This affects the privacy policy and the data-export scope.
11. **Crash and analytics vendor:** Sentry or Firebase Crashlytics?
12. **Redis:** drop it from config (it's unused) or keep it for planned rate limiting or caching? I recommend dropping it for the MSV.
