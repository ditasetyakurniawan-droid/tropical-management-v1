# Tropical Management V1

Tropical Management is a restaurant operations and internal-audit platform built as independently deployable Go microservices with a Next.js PWA frontend. This repository owns application code, tests, Docker build definitions, versioned database migrations, and the Jenkins CI pipeline.

> Current engineering baseline: P0.1 runtime reliability and P0.2 delivery-owned database migrations are validated end-to-end in the `test-app` Kubernetes environment.

## System architecture

```text
Browser / PWA
      |
      v
Next.js Web
      |
      v
API Gateway
      |
      +-- auth-service ---------> tropical_auth
      +-- audit-service --------> tropical_audit
      +-- inventory-service ----> tropical_inventory
      +-- sales-service --------> tropical_sales
      +-- dashboard-service ----> HTTP aggregation only
      +-- chat-service ---------> tropical_chat

MySQL: DB-dt / 192.168.100.70
Runtime secrets: HashiCorp Vault
```

The application remains a source monorepo. A service boundary is defined by runtime, API, data ownership, and deployable image, not by repository count.

## Service catalog

| Runtime | Responsibility | Persistent store |
|---|---|---|
| `auth-service` | Login, JWT, users, RBAC | `tropical_auth` |
| `audit-service` | Audit checklist, findings, corrective-action workflow | `tropical_audit` |
| `inventory-service` | Items, suppliers, stock, stock movements | `tropical_inventory` |
| `sales-service` | Daily operational sales | `tropical_sales` |
| `dashboard-service` | Cross-service HTTP aggregation | none |
| `chat-service` | General room, MySQL history, SSE real-time delivery | `tropical_chat` |
| `api-gateway` | API entry point, JWT validation, RBAC, reverse proxy | none |
| `web` | Next.js PWA frontend | none |
| `db-migrator` | Versioned schema changes before deployment | all owned schemas |

## Production-like delivery path

```text
GitHub application main
        |
        v
      Jenkins
 test -> vet -> Sonar -> build
        |
        v
 Harbor immutable images
        |
        v
GitOps image-tag commit
        |
        v
      Argo CD
        |
        +--> PreSync: tropical-db-migrator
        |         |
        |         +--> Vault-injected DB DSNs
        |         +--> schema_migrations/checksum/dirty/lock
        |         `--> failure stops the release
        |
        `--> Kubernetes Deployment rollout
```

Jenkins does not apply Kubernetes manifests directly. The GitOps repository is the deployment source of truth and Argo CD performs reconciliation.

## Runtime health contract

All Go backends expose:

- `GET /healthz`: compatibility health endpoint.
- `GET /livez`: process liveness only. External dependency failure must not create restart loops.
- `GET /readyz`: traffic readiness. DB-backed services perform bounded MySQL readiness checks.

Kubernetes uses startup `/livez`, readiness `/readyz`, liveness `/livez`, and `terminationGracePeriodSeconds: 30`. The shared server lifecycle handles SIGINT/SIGTERM and gives in-flight HTTP requests a 20-second graceful shutdown budget.

See [`docs/architecture.md`](docs/architecture.md) and [`docs/production-readiness-p0-health.md`](docs/production-readiness-p0-health.md).

## Database migration contract

Schema ownership is no longer part of normal service startup in the Kubernetes delivery path.

- Migrations are versioned under `internal/migrate/sql/<target>/`.
- `services/db-migrator` executes the five DB targets before application rollout.
- `schema_migrations` records version, migration name, SHA-256 checksum, dirty state, start time, and applied time.
- Applied migration files are immutable. A checksum mismatch fails closed.
- Dirty migrations fail closed.
- MySQL advisory locks prevent concurrent migrators for the same target.
- Argo CD runs the migrator as a `PreSync` Job.
- Vault Agent renders DB DSNs as files for the migration Job.

See [`docs/database-migrations.md`](docs/database-migrations.md).

## Repository split

- `tropical-management-v1`: application code, tests, Dockerfiles, migrations, Jenkins pipeline.
- `tropical-management-gitops`: Kubernetes desired state, Kustomize overlays, Vault injection, Argo CD hooks/applications.

## Developer documentation

Start here:

- [`docs/DEVELOPMENT-NOTES.md`](docs/DEVELOPMENT-NOTES.md): daily workflow and quick checks.
- [`docs/architecture.md`](docs/architecture.md): boundaries, ownership, runtime lifecycle.
- [`docs/database-migrations.md`](docs/database-migrations.md): migration authoring and recovery rules.
- [`docs/cicd-gitops.md`](docs/cicd-gitops.md): CI/CD and release lifecycle.
- [`docs/api.md`](docs/api.md): API surface.
- [`docs/live-chat.md`](docs/live-chat.md): live chat design and constraints.
- [`docs/observability-and-testing.md`](docs/observability-and-testing.md): logs, request tracing, tests, Sonar.
- [`docs/coverage-policy.md`](docs/coverage-policy.md): quality-gate policy.
- [`docs/engineering-review.md`](docs/engineering-review.md): engineering review and residual risks.
- [`docs/production-readiness-roadmap.md`](docs/production-readiness-roadmap.md): prioritized hardening roadmap.
- [`docs/p0-traffic-protection.md`](docs/p0-traffic-protection.md): API concurrency, login rate limits, and SSE connection caps.

## Quality gates

```bash
make fmt-check
make test
make vet
```

`make test` is expected to run race-enabled Go tests. Jenkins also runs frontend build and Sonar Quality Gate before publishing images.

## Local development

Docker Compose remains useful for developer feedback, but after P0.2 the local path must be kept aligned with the Kubernetes migration lifecycle.

Known follow-up: a fresh Compose volume currently needs a one-shot `db-migrator` path, and local `CHAT_DB_DSN` must point to `tropical_chat`. Track this as P0.2.1 before treating fresh local bootstrap as equivalent to cluster bootstrap.

Do not solve that by re-enabling schema migration inside runtime service startup.

## Live Chat scaling constraint

The current SSE broker is in-memory. MySQL persists history, but live fan-out between multiple `chat-service` replicas requires Redis, NATS, or equivalent shared pub/sub. Keep chat at one replica until that is implemented.

## Current engineering status

- P0.1 runtime health + graceful shutdown: **complete and rollout-validated**.
- P0.2 versioned delivery-owned DB migration: **complete and E2E validated in Kubernetes**.
- P0.2.1 local Compose migration alignment: **next cleanup**.
- P0.3 Kubernetes resource/availability hardening: **next platform milestone**.
- P0.4 least privilege + workload/network security: **planned**.
- P1 observability, DB resilience, TLS/ingress maturity: **planned**.

See [`docs/production-readiness-roadmap.md`](docs/production-readiness-roadmap.md).
