# Tropical Management - Development Notes / Cheat Sheet

Use this file as the short daily reference while coding.

## Local URLs

```text
Web          http://localhost:3000
API Gateway  http://localhost:8080
Live Chat    http://localhost:3000/chat
```

Only ports 3000 and 8080 are published to the host. Backend services and MySQL are internal to Docker Compose.

## Services

```text
auth-service        users, login, JWT, RBAC
audit-service       audits, findings, corrective action
inventory-service   items, suppliers, stock movements
sales-service       daily sales
dashboard-service   HTTP aggregation
chat-service        persistent General Live Chat + SSE
api-gateway         JWT validation, RBAC, reverse proxy
web                 Next.js PWA
```

## Start / stop

```bash
docker compose up -d --build
docker compose ps
docker compose down
```

## Logs

```bash
docker compose logs -f <service>
docker compose logs --tail=100 <service>
```

## Rebuild only what changed

```bash
# frontend
docker compose build web && docker compose up -d web

# backend example
docker compose build audit-service && docker compose up -d audit-service

# live-chat related
docker compose build chat-service api-gateway web
docker compose up -d chat-service api-gateway web
```

## Health

```bash
curl -fsS http://localhost:8080/healthz
```

Internal service example:

```bash
docker run --rm \
  --network tropical-management-v1_default \
  curlimages/curl:8.10.1 \
  http://chat-service:8080/healthz
```

## Backend quality

```bash
go test ./...
go vet ./...
```

## Frontend quality

```bash
cd web
npm install
npm run build
```

## Database ownership target

```text
auth-service       tropical_auth
audit-service      tropical_audit
inventory-service  tropical_inventory
sales-service      tropical_sales
chat-service       tropical_chat (production target)
```

## Add a feature

1. Decide which service owns it.
2. Add backend business rule + validation.
3. Add tests.
4. Add/update API Gateway route/RBAC if needed.
5. Add frontend.
6. Update docs.
7. Build only impacted services first.
8. Run full acceptance before PR.

## Add a microservice

1. `services/<service>/main.go`
2. Define owned DB and API prefix.
3. Add Docker Compose service.
4. Add API Gateway route.
5. Add Jenkins image build/push list.
6. Add GitOps runtime config/workload later.
7. Add docs/tests.

## Security rules

- Never commit password/token/private key/kubeconfig.
- Frontend UI hiding is not authorization.
- Gateway/service must enforce access.
- One service must not directly query another service's database.
- Production secrets come from Vault.
- Production images should use immutable Git SHA tags.

## Live Chat

- All authenticated Admin/Auditor/Staff users can chat.
- Browser sends only message body.
- Gateway injects trusted user ID/name/role.
- SSE stream: `/api/chat/stream`.
- Until Redis/NATS shared pub/sub exists, keep chat-service at one Kubernetes replica.

## Git workflow

```text
main
  -> feature/<name>
  -> local test
  -> push
  -> PR
  -> review
  -> merge
```

## Next priority

```text
1. Vault *_FILE support
2. Per-service DB credentials
3. Jenkins -> Harbor immutable images
4. Jenkins -> GitOps tag update
5. Kubernetes Deployments/Services
6. Argo CD sync
7. NetworkPolicy/HPA/PDB/Ingress
8. Observability
```
