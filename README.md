# Tropical Management V1

Enterprise restaurant management and internal audit platform with a tropical green/gold visual identity.

## Architecture

This repository is an **application monorepo** containing independently deployable microservices:

- `auth-service` — login, JWT, users, Admin/Auditor/Staff RBAC
- `audit-service` — audit checklist and issue tracker
- `inventory-service` — stock, reorder alerts, suppliers
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

Useful endpoints:

```text
Frontend       http://localhost:3000
API Gateway    http://localhost:8080
Auth           http://localhost:8081
Audit          http://localhost:8082
Inventory      http://localhost:8083
Sales          http://localhost:8084
Dashboard      http://localhost:8085
MySQL          localhost:3306
```

See `docs/` for architecture, local development, API behavior, and Jenkins/Argo CD flow.
