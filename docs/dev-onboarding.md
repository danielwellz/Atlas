# Atlas Developer Onboarding (macOS)

This is the canonical local setup for the Atlas monorepo (`atlas-api`, `atlas-mobile`, `atlas-unity`).

## Required installs

Install the following before running the apps:

1. Xcode (latest stable) and Xcode Command Line Tools  
   Run: `xcode-select --install`
2. CocoaPods  
   Example: `brew install cocoapods`
3. Node.js 22.x (required by `atlas-mobile`)  
   Example with nvm: `nvm install 22 && nvm use 22`
4. Watchman  
   Run: `brew install watchman`
5. Docker Desktop
6. Go toolchain (1.22+)

## Android Studio setup

1. Install Android Studio.
2. In SDK Manager, install:
   - Android SDK Platform 36
   - Android SDK Build-Tools 36.0.0
   - Android Emulator
   - Android SDK Platform-Tools
   - NDK (Side by side) 27.1.12297006
3. Set SDK paths in your shell profile (`~/.zshrc`):

```bash
export ANDROID_HOME="$HOME/Library/Android/sdk"
export ANDROID_SDK_ROOT="$ANDROID_HOME"
export PATH="$PATH:$ANDROID_HOME/platform-tools:$ANDROID_HOME/emulator"
```

4. Start an emulator in Android Studio (Device Manager) before running Android builds.

## First-time repo setup

1. Start local dependencies:

```bash
make dev-up
```

2. If you are migrating from an older clone that previously committed mobile deps, untrack them once:

```bash
git rm -r --cached atlas-mobile/node_modules
```

3. Install mobile JS dependencies deterministically:

```bash
make mobile-install
```

4. Install iOS pods:

```bash
cd atlas-mobile/ios && pod install
cd ../..
```

5. Run database migrations:

```bash
make api-migrate-up
```

6. Seed the database:

```bash
make api-seed
# Optional extra exercise seed data
make api-seed-exercises
```

## Environment profiles and config checks

`atlas-api` supports explicit runtime profiles:

- `APP_ENV=local`
- `APP_ENV=staging`
- `APP_ENV=prod`

Only `local` applies fallback defaults. `staging` and `prod` require explicit environment variables and fail fast when missing.

Validate config early:

```bash
make config:validate
```

Print sanitized config (secrets redacted):

```bash
make config:print
```

Use `atlas-api/.env.example` as the canonical variable reference.

## Run the apps

1. Run Atlas API:

```bash
make api-run
```

2. In a separate terminal, start Metro:

```bash
cd atlas-mobile && npm start
```

3. Run mobile app on iOS:

```bash
make mobile-run-ios
```

4. Run mobile app on Android:

```bash
make mobile-run-android
```

## Secrets and staging deployment

- Never commit real secrets (`.env` files, API keys, passwords, JWT secrets).
- Recommended secret flow:
  - Store source-of-truth secrets in 1Password.
  - Mirror deploy-time values into CI secret storage (for example GitHub Actions secrets).
  - Inject at runtime/deploy time.

Staging Kubernetes + GitOps manifests are in:

- `infra/staging/k8s/`
- `infra/staging/argocd/`
- `infra/staging/otel-collector/`
- `infra/staging/README.md`

Legacy Docker Compose fallback remains in:

- `infra/staging/docker-compose.yml`

## Code generation

Regenerate all generated artifacts from the repo root:

```bash
make generate
```

Verify generated artifacts are committed (CI uses this):

```bash
make verify-generated
```

Pinned generator versions:

- `oapi-codegen`: `v2.4.1` (in `Makefile` and `atlas-api/tools/tools.go`)
- `sqlc`: `1.27.0` (in `Makefile` docker image tag)
- `goose`: `v3.24.1` (in `Makefile` and `atlas-api/tools/tools.go`)
- `openapi-typescript`: `7.10.1` (in `atlas-mobile/package.json` + checked by `Makefile`)

## Validation commands

Run these after setup changes:

```bash
make dev-up
make config:validate
make generate
make verify-generated
kubectl kustomize infra/staging/k8s >/dev/null
cd atlas-mobile && npm ci && npm test -- --watchAll=false && npm run lint && npm run typecheck
cd ..
make api-test
```
