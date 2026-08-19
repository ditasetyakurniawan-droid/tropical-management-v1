# Tropical Management V1

**Tropical Management** is a restaurant operations and internal-audit platform built as independently deployable Go microservices with a Next.js PWA frontend. The current product milestone includes functional hardening, a luxury tropical UI, and General Live Chat.

## Current architecture

```text
Browser / PWA
      |
      v
Next.js Web :3000
      |
      v
API Gateway :8080
      |
      +-- auth-service
      +-- audit-service
      +-- inventory-service
      +-- sales-service
      +-- dashboard-service
      +-- chat-service
             |
             +-- persistent history
             +-- real-time SSE

Backend services -> MySQL
```

A microservice is defined here by independent runtime/API/data/deployment boundaries. The application stays in one source monorepo until independent team ownership or release cadence justifies splitting repositories.

## Implemented modules

- Interactive dashboard: Sales, Audit Score, Open Findings, Inventory Alerts
- Internal Audit: checklist, score, finding workflow, PIC, due date, corrective action
- Inventory: stock, reorder alerts, suppliers, stock movement ledger
- Sales: operational daily sales entries
- Users/RBAC: Admin, Auditor, Staff; role editing and activate/deactivate
- General Live Chat: authenticated users, role badges, per-user accent colors, persistent messages, real-time SSE
- Responsive luxury tropical UI/PWA foundation

## Run locally

Prerequisite: Docker + Docker Compose v2.

```bash
git clone https://github.com/ditasetyakurniawan-droid/tropical-management-v1.git
cd tropical-management-v1
docker compose up -d --build
docker compose ps
curl -fsS http://localhost:8080/healthz
```

Open:

```text
Web        http://localhost:3000
Live Chat  http://localhost:3000/chat
API        http://localhost:8080
```

Development-only bootstrap account:

```text
admin@tropical.local
ChangeThis123!
```

Do not reuse local development credentials in production.

## Service catalog

| Runtime | Responsibility |
|---|---|
| `auth-service` | Login, JWT, users, RBAC |
| `audit-service` | Audit checklist, findings, corrective-action workflow |
| `inventory-service` | Items, suppliers, stock, stock movements |
| `sales-service` | Daily operational sales |
| `dashboard-service` | Cross-service HTTP aggregation |
| `chat-service` | General room, MySQL history, SSE real-time delivery |
| `api-gateway` | Single API entry point, JWT validation, RBAC, reverse proxy |
| `web` | Next.js PWA frontend |

## Developer documentation

Start here when continuing development:

- [`docs/DEVELOPMENT-NOTES.md`](docs/DEVELOPMENT-NOTES.md) - short daily cheat sheet
- [`docs/DEVELOPER-GUIDE.md`](docs/DEVELOPER-GUIDE.md) - complete development handbook and current project documentation
- [`docs/architecture.md`](docs/architecture.md) - architecture boundaries
- [`docs/api.md`](docs/api.md) - API surface
- [`docs/live-chat.md`](docs/live-chat.md) - General Live Chat design and acceptance
- [`docs/local-development.md`](docs/local-development.md) - local Docker workflow
- [`docs/cicd-gitops.md`](docs/cicd-gitops.md) - Jenkins/GitOps direction
- [`docs/phase3-functional-hardening.md`](docs/phase3-functional-hardening.md) - Phase 3 acceptance notes

## Development commands

```bash
# Backend
go test ./...
go vet ./...

# Frontend
cd web && npm install && npm run build

# Rebuild one backend service
docker compose build audit-service && docker compose up -d audit-service

# Rebuild frontend
docker compose build web && docker compose up -d web

# Logs
docker compose logs -f api-gateway
```

Backend service ports are intentionally not published to the host. Test internal health through the Compose network when necessary.

## Repository split

- `tropical-management-v1` - application code, Docker build definitions, tests, Jenkins pipeline
- `tropical-management-gitops` - Kubernetes desired state, Kustomize overlays, Argo CD definitions

Production delivery target:

```text
GitHub -> Jenkins -> Harbor -> GitOps commit -> Argo CD -> Kubernetes
```

Jenkins should not directly apply production manifests.

## Database target

Production MySQL is external. Target database ownership:

```text
auth-service       -> tropical_auth
audit-service      -> tropical_audit
inventory-service  -> tropical_inventory
sales-service      -> tropical_sales
chat-service       -> tropical_chat
```

Production credentials must be scoped per service and injected from Vault.

## Important Live Chat scaling note

The current SSE broker is in-memory. Message history is persisted in MySQL, but live fan-out between multiple chat-service replicas requires shared pub/sub such as Redis or NATS. Until that hardening is implemented, deploy `chat-service` with one replica.

## Next technical milestone

1. Add `*_FILE` secret support to Go services.
2. Inject DB/JWT secrets from Vault.
3. Push immutable service images to Harbor from Jenkins.
4. Automate Jenkins updates to the GitOps repository.
5. Add real Kubernetes Deployment/Service manifests.
6. Configure Argo CD reconciliation.
7. Add Ingress, SSE proxy settings, NetworkPolicy, HPA/PDB, and observability.

See [`docs/DEVELOPER-GUIDE.md`](docs/DEVELOPER-GUIDE.md) for the full roadmap and implementation rules.
