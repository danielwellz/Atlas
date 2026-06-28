# Atlas

Atlas is a fitness platform monorepo with a Go API, a React Native mobile app, and a Unity anatomy/biomechanics module. The app covers onboarding, training programs, workout logging, habits, nutrition, form check, anatomy visualization, and coaching/community workflows.

## Repository Layout

```text
.
├── atlas-api      # Go backend, PostgreSQL schema, OpenAPI contract, sqlc queries
├── atlas-mobile   # React Native iOS/Android app
├── atlas-unity    # Unity as a Library project for anatomy and biomechanics
├── docs           # Architecture, compliance, privacy, and bridge schema docs
├── infra          # Local and staging infrastructure
└── Makefile       # Common development commands
```

## Stack

- Backend: Go 1.22+, chi, PostgreSQL, goose, sqlc, oapi-codegen
- Mobile: React Native 0.84, TypeScript, React Navigation, TanStack Query
- Native modules: Android Kotlin, iOS Swift/Objective-C bridges
- Anatomy engine: Unity as a Library
- Local infra: Docker Compose

## Prerequisites

For the full local development environment, install:

- Go 1.22+
- Node.js 22.x
- Docker Desktop
- Xcode and Xcode Command Line Tools
- CocoaPods
- Watchman
- Android Studio with Android SDK Platform 36, Build Tools 36.0.0, and NDK 27.1.12297006

See [docs/dev-onboarding.md](docs/dev-onboarding.md) for the complete macOS setup.

## Quick Start

Start local infrastructure:

```bash
make dev-up
```

Install mobile dependencies:

```bash
make mobile-install
```

Install iOS pods:

```bash
cd atlas-mobile/ios
pod install
cd ../..
```

Run database migrations and seed data:

```bash
make api-migrate-up
make api-seed
make api-seed-exercises
```

Run the API:

```bash
LOAD_DOTENV=true make api-run
```

Start Metro in a separate terminal:

```bash
cd atlas-mobile
npm start
```

Run the mobile app:

```bash
make mobile-run-ios
# or
make mobile-run-android
```

## Environment

Copy the API example environment file before running locally:

```bash
cd atlas-api
cp .env.example .env
```

The API can load `.env` when either `APP_ENV=development` or `LOAD_DOTENV=true` is set. Real `.env` files and deployment secrets must not be committed.

Useful config checks:

```bash
make config:validate
make config:print
```

## Common Commands

```bash
make dev-up              # Start local Docker dependencies
make dev-down            # Stop local Docker dependencies
make api-run             # Run the Go API
make api-test            # Run backend tests
make api-migrate-up      # Apply database migrations
make api-migrate-down    # Roll back one migration
make api-seed            # Load local seed data
make mobile-install      # npm ci in atlas-mobile
make mobile-run-ios      # Run React Native on iOS
make mobile-run-android  # Run React Native on Android
make generate            # Regenerate API, sqlc, and mobile OpenAPI clients
make verify-generated    # Verify generated files are committed
```

Mobile-only validation:

```bash
cd atlas-mobile
npm test -- --watchAll=false
npm run lint
npm run typecheck
```

## API Contract

The OpenAPI contract lives at:

```text
atlas-api/openapi/openapi.yaml
```

Generated artifacts include:

- `atlas-api/internal/httpapi/generated/openapi.gen.go`
- `atlas-api/internal/db/sqlc/`
- `atlas-mobile/src/api/generated/openapi.ts`

Regenerate them from the repo root:

```bash
make generate
```

## Unity Anatomy Module

The Unity project in `atlas-unity` exports Unity as a Library for the React Native Anatomy tab.

Bridge contract:

- GameObject: `AtlasBridge`
- Method: `OnReactNativeMessage(string json)`
- Topic: `anatomy.engine.v1`

References:

- [atlas-unity/README.md](atlas-unity/README.md)
- [docs/anatomy-engine-schema-v1.md](docs/anatomy-engine-schema-v1.md)

## Privacy And Compliance

Atlas includes consent-gated local form check and optional upload flows. Upload behavior requires explicit user action, entitlement, and consent.

References:

- [docs/form-check-privacy-v1.md](docs/form-check-privacy-v1.md)
- [docs/compliance.md](docs/compliance.md)

## Deployment

Local infrastructure is in `infra/local/`.

Staging deployment manifests and notes are in:

- `infra/staging/k8s/`
- `infra/staging/argocd/`
- `infra/staging/otel-collector/`
- `infra/staging/README.md`

## License

No license file is currently included.
