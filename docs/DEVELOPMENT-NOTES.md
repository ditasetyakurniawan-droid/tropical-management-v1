# Tropical Management - Development Notes / Cheat Sheet

## Current baseline

```text
P0.1 Runtime reliability       COMPLETE
P0.2 DB migration separation  COMPLETE on Kubernetes / E2E validated
P0.2.1 Local Compose parity    OPEN
```

## Service ownership

```text
auth-service        -> tropical_auth
audit-service       -> tropical_audit
inventory-service   -> tropical_inventory
sales-service       -> tropical_sales
chat-service        -> tropical_chat
dashboard-service   -> no owned DB
api-gateway         -> no owned DB
```

Never point two service migration targets at the same schema.

## Runtime health

```text
/healthz  compatibility
/livez    process liveness only
/readyz   traffic readiness; DB-backed services check MySQL
```

Kubernetes backend contract:

```text
startup    /livez
readiness  /readyz
liveness   /livez
terminationGracePeriodSeconds = 30
```

## Quality before PR

```bash
make fmt-check
make test
make vet
git diff --check
```

## Daily feature workflow

1. Decide service/domain owner.
2. Update business logic and validation.
3. Add tests.
4. If schema changes, add a new versioned migration. Never edit an applied one.
5. Update gateway route/RBAC if needed.
6. Update frontend.
7. Update docs/acceptance criteria.
8. Run focused tests, then full quality gates.
9. PR -> Jenkins -> Sonar -> Harbor -> GitOps -> Argo CD.
10. Verify migrator, rollout health, and restarts after merge.

## Add a database migration

See [`database-migrations.md`](database-migrations.md).

Short version:

```text
internal/migrate/sql/<target>/NNNNNN_description.sql
```

One migration version is immutable after it has been applied. A checksum mismatch is a deliberate release blocker.

## Release watch

```bash
kubectl -n test-app get jobs -w
kubectl -n test-app get pods -w
kubectl -n argocd get application tropical-management
```

A healthy DB-changing deployment should show:

```text
PreSync db-migrator -> Complete 1/1
then application rollout
then all expected pods Running with restart count stable
```

## Secrets

Kubernetes secrets come from Vault. Do not commit passwords, tokens, private keys, kubeconfig, Harbor credentials, or plaintext Kubernetes Secrets.

Migration Job credentials are injected as files. Runtime and migration DB credentials are still a hardening target for separation/least privilege.

## Local development warning

Fresh Docker Compose bootstrap is not yet fully aligned with P0.2. Do not reintroduce service-startup DDL to fix it. Add a one-shot Compose migrator and point local chat at `tropical_chat` as P0.2.1.

## Live Chat

Chat history is MySQL-backed. SSE fan-out is in-memory. Keep one Kubernetes chat replica until Redis/NATS or another shared pub/sub is implemented.

## Git workflow

```text
main -> feature/fix/docs branch -> local quality -> PR -> review -> merge
```

Normal post-merge deployment must be automated. Avoid manual Argo CD sync/restart unless diagnosing an exceptional condition.

## Next priorities

1. P0.2.1 fresh Compose migrator parity.
2. P0.3 workload requests/limits + replica/PDB strategy.
3. P0.4 runtime/migrator DB-user split + securityContext + NetworkPolicy.
4. P1 centralized logs/metrics/traces + alerting.
5. P1 DB resilience and ingress/TLS maturity.
6. Redis/NATS before multi-replica chat.
