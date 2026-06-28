# Argo CD Staging App

Apply the staging application manifest:

```bash
kubectl apply -f infra/staging/argocd/atlas-api-staging-application.yaml
```

Before applying, update `spec.source.repoURL` to the canonical Atlas repository URL.
