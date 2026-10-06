# Atlas Staging Deployment (Kubernetes + GitOps)

This directory contains the staging deployment configuration for `atlas-api`.

## Layout

- `k8s/`: Kustomize manifests for `atlas-api` + OpenTelemetry Collector.
- `argocd/`: Argo CD Application manifest for staging sync.
- `otel-collector/`: Collector pipeline config and workload manifests.
- `smoke/`: HTTP smoke test script used by CI after deploy.
- `docker-compose.yml`: legacy bootstrap fallback (not the primary staging path).

## Managed dependencies

Staging expects managed services outside the cluster manifests:

- Postgres (required)
- Redis
- S3-compatible object storage

Postgres specifics are documented in `k8s/managed-dependencies.md`.

## Secrets

Never commit real credentials.

1. Keep source-of-truth secrets in 1Password (or equivalent).
2. Sync runtime values into CI secrets.
3. Materialize Kubernetes secrets at deploy time (for example via External Secrets or sealed secrets).

Example secret shape is in `k8s/atlas-api-staging-secrets.example.yaml`.

## Bootstrap

1. Update `argocd/atlas-api-staging-application.yaml`:
   - set `spec.source.repoURL` to your repository URL
   - verify `targetRevision` points to your deployment branch
2. Apply Argo CD application manifest:

```bash
kubectl apply -f infra/staging/argocd/atlas-api-staging-application.yaml
```

3. Apply/refresh staging secret (or your secret controller equivalent).
4. Confirm Argo app health:

```bash
argocd app get atlas-api-staging
```

## Local render check

Validate kustomize output before committing:

```bash
kubectl kustomize infra/staging/k8s
```

## CI deployment flow

The `Staging Deploy (GitOps)` job in `.github/workflows/ci.yml` is **off by default**. It runs only when the repository variable `STAGING_DEPLOY_ENABLED` is `true`. Until then it shows as skipped on every push to `main`, because there is no staging cluster or Argo CD yet (T19).

When it is enabled, on push to `main` CI will:

1. Build and push `atlas-api` image to GHCR.
2. Update `infra/staging/k8s/kustomization.yaml` with the image tag.
3. Commit that manifest change back to `main` with `[skip ci]`.
4. Trigger `argocd app sync atlas-api-staging`.
5. Run staging smoke tests and an OTel span-ingestion assertion.

Required GitHub secrets for the deploy job:

- `ARGOCD_SERVER`
- `ARGOCD_AUTH_TOKEN`
- `STAGING_API_BASE_URL`
- `STAGING_OTEL_COLLECTOR_METRICS_URL`

### Turning the deploy job on

1. Finish the cluster bootstrap above: Argo CD installed, `atlas-api-staging` Application applied, staging secrets in place.
2. Add the four secrets above under GitHub → repository **Settings → Secrets and variables → Actions → Secrets** (or on the `staging` environment the job uses).
3. Under **Settings → Secrets and variables → Actions → Variables**, add a repository variable `STAGING_DEPLOY_ENABLED` with the value `true`. With the GitHub CLI: `gh variable set STAGING_DEPLOY_ENABLED --body true`.
4. Push to `main` (or re-run the latest `main` CI run). The job should now run instead of being skipped.

To pause deploys again, set the variable to anything other than `true` (for example `false`) or delete it.
