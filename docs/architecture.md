# Architecture

## Application boundary

A microservice is defined here by an independent runtime, API contract, database ownership boundary and deployable image — not by having a separate Git repository.

```text
Next.js PWA
    |
API Gateway :8080
    |
    +-- auth-service       -> tropical_auth
    +-- audit-service      -> tropical_audit
    +-- inventory-service  -> tropical_inventory
    +-- sales-service      -> tropical_sales
    +-- dashboard-service  -> HTTP aggregation only
```

Production MySQL is external on `DB-dt (192.168.100.70)`. Each service receives its DSN through Vault-injected configuration; credentials must never be committed.

## Repository strategy

Start with two repositories:

1. `tropical-management-v1` — application source, tests, Docker build definitions and Jenkinsfile.
2. `tropical-management-gitops` — Kubernetes/Kustomize manifests, environment overlays and Argo CD Applications/ApplicationSets.

Split individual services into separate source repositories later only when independent team ownership/release cadence makes the extra repository overhead worthwhile.
