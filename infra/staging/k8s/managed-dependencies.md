# Staging Managed Dependencies

Atlas staging Kubernetes manifests intentionally do not self-host stateful services.

## Postgres (required)

- Use a managed Postgres instance with automated backups, point-in-time recovery, and SSL/TLS enforced.
- Provide a least-privilege DB user scoped to the Atlas staging database.
- Rotate credentials through your secret manager, then update the `atlas-api-staging-secrets` source (not this repository).
- Configure `POSTGRES_URL` with `sslmode=require` (or stricter) and private-network hostnames.
- Ensure database migrations run before API rollout (`make api-migrate-up` in release workflow or migration job).

## Redis and Object Storage

- `REDIS_ADDR` should point to managed Redis with auth and network restrictions.
- `ASSET_STORAGE_*` and `MINIO_*` values should point to managed S3-compatible object storage with server-side encryption enabled.

## Secrets source of truth

- Store production-like credentials in 1Password (or equivalent), sync to CI secrets, and materialize into Kubernetes secrets at deploy time.
- Never commit environment files or live secret values.
