# Tropical Management V1

Enterprise restaurant management and internal audit platform with a luxury tropical green/gold visual identity.

## Architecture

This repository is an **application monorepo** containing independently deployable microservices:

- `auth-service` — login, JWT, users, Admin/Auditor/Staff RBAC
- `audit-service` — audit checklist, findings, corrective-action workflow
- `inventory-service` — stock, reorder alerts, suppliers, movement ledger
- `sales-service` — sales entries and daily summary
- `dashboard-service` — cross-service metric aggregation
- `api-gateway` — single client entry point and role authorization
- `web` — Next.js PWA frontend

Production delivery is designed for **Jenkins CI -> Harbor -> GitOps repository -> Argo CD -> Kubernetes**. Jenkins does not need to run `kubectl apply` against production.

## Run locally

Prerequisite: Docker with Compose v2.

```bash
git clone https://github.com/ditasetyakurniawan-droid/tropical-management-v1.git
cd tropical-management-v1
docker compose up --build
```

Open `http://localhost:3000` and login with the local bootstrap account:

- email: `admin@tropical.local`
- password: `ChangeThis123!`

These credentials are **local development defaults only**. Production secrets must be injected through Vault.

### Host entry points

```text
Frontend       http://localhost:3000
API Gateway    http://localhost:8080
```

Backend microservices and MySQL are intentionally internal to the Compose network. They communicate via service DNS names such as `auth-service:8080` and `mysql:3306`; ports 8081–8085 and 3306 are not required on the host.

## Phase 3 functional hardening

Phase 3 adds:

- issue due dates, PIC, corrective actions, and status workflow
- supplier UI and inventory stock-movement ledger
- user role editing and activate/deactivate controls
- route protection and logout
- operational dashboard analytics
- luxury tropical UI with lightweight animated botanical leaves
- validation unit tests for role and issue workflow rules

See `docs/phase3-functional-hardening.md` for acceptance testing.

## Delivery repositories

- application source: `tropical-management-v1`
- Kubernetes desired state: `tropical-management-gitops`

See `docs/` for architecture, local development, API behavior, Jenkins/Argo CD flow, and Phase-3 acceptance testing.
