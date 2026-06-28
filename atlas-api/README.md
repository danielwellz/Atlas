# atlas-api

Go backend service for Atlas.

## Stack

- Go 1.22+
- chi router
- zap structured logging
- testify tests

## Setup

1. Start local infra from repo root:
   ```bash
   make dev-up
   ```
2. Configure environment variables:
   ```bash
   cp .env.example .env
   ```
   By default, the API reads process environment variables only.
   To load `.env`, either:
   - set `APP_ENV=development`, or
   - set `LOAD_DOTENV=true`
3. Run database migrations:
   ```bash
   make api-migrate-up
   ```
4. (Optional) Seed local data:
   ```bash
   make api-seed
   ```
5. Import exercise catalog sample data:
   ```bash
   make api-seed-exercises
   ```
6. Run the API:
   ```bash
   LOAD_DOTENV=true make api-run
   ```
7. Hit the health endpoint:
   ```bash
   curl http://localhost:8080/api/v1/health
   ```

## Test

```bash
make api-test
```

## API Contract

OpenAPI spec: `openapi/openapi.yaml`

Generate server types/interfaces and sqlc queries:

```bash
make api-generate
```

Run only sqlc generation:

```bash
make api-sqlc-generate
```

## Endpoints

- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/refresh`
- `POST /api/v1/auth/logout`
- `GET /api/v1/me`
- `GET /api/v1/consents`
- `POST /api/v1/consents/grant`
- `POST /api/v1/consents/revoke`
- `GET /api/v1/exercises?query=&equipment=&pattern=`
- `GET /api/v1/exercises/{id}`
- `GET /api/v1/exercises/{id}/biomechanics`
- `GET /api/v1/onboarding/status`
- `PUT /api/v1/profile`
- `PUT /api/v1/goals`
- `GET /api/v1/programs`
- `POST /api/v1/programs/enroll`
- `GET /api/v1/programs/current`
- `POST /api/v1/workouts/start`
- `POST /api/v1/workouts/{workout_id}/add_set`
- `POST /api/v1/workouts/{workout_id}/complete`
- `GET /api/v1/workouts/history?limit=&cursor=`
- `GET /api/v1/workouts/{id}`
- `GET /api/v1/dashboard/summary`
- `GET /api/v1/habits`
- `POST /api/v1/habits`
- `POST /api/v1/habits/{id}/toggle_today`
- `GET /api/v1/habits/streaks`
- `PUT /api/v1/nutrition/targets`
- `GET /api/v1/nutrition/today`
- `POST /api/v1/nutrition/checkin`

## Biomechanics Asset Storage

Biomechanics animation references can be normalized through local files or S3/MinIO.

- `ASSET_STORAGE_BACKEND=local|s3` (default `local`)
- `ASSET_STORAGE_BUCKET=atlas-assets` (used by `s3`)
- `MINIO_ENDPOINT=localhost:9000`
- `MINIO_ROOT_USER=atlasminio`
- `MINIO_ROOT_PASSWORD=atlasminio`
- `MINIO_USE_SSL=false`
