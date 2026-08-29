# Local development

## Full stack

```bash
docker compose up -d --build
docker compose ps -a
```

Local startup follows the same schema ownership model as Kubernetes:

```text
MySQL healthy
  -> one-shot db-migrator
  -> migrations complete
  -> DB-backed application services start
```

`db-migrator` must finish with exit code `0`. Runtime services do not execute schema DDL during startup.

```bash
docker compose ps -a db-migrator
docker compose logs db-migrator
```

Open `http://localhost:3000`.

Local bootstrap admin:

```text
admin@tropical.local
ChangeThis123!
```

Never reuse this password outside local development.

## Host ports

Only these ports are published:

```text
3000 -> web
8080 -> api-gateway
```

MySQL and backend microservices are internal-only and use Compose DNS names such as `mysql:3306` and `chat-service:8080`.

## Gateway health

```bash
curl -fsS http://localhost:8080/healthz
```

## Internal service health

Example:

```bash
docker run --rm \
  --network tropical-management-v1_default \
  curlimages/curl:8.10.1 \
  http://auth-service:8080/healthz
```

Use the same pattern for:

```text
audit-service
inventory-service
sales-service
chat-service
dashboard-service
```

## Logs

```bash
docker compose logs -f
docker compose logs -f api-gateway
docker compose logs --tail=100 chat-service
```

## Rebuild selected services

```bash
# frontend only
docker compose build web
docker compose up -d web

# one backend
docker compose build audit-service
docker compose up -d audit-service

# live chat path
docker compose build chat-service api-gateway web
docker compose up -d chat-service api-gateway web
```

## Login API example

```bash
curl -s http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@tropical.local","password":"ChangeThis123!"}'
```

## Reset local data

Warning: this deletes the local MySQL volume. Use this only when a clean local bootstrap is intended.

```bash
docker compose down -v
docker compose up -d --build
docker compose ps -a
```

Verify the one-shot migration completed before testing the application:

```bash
docker compose ps -a db-migrator
docker compose logs db-migrator
```

Expected migration log ends with:

```text
event=migration_run_completed targets=5
```

The fresh database should contain independent migration metadata for:

```text
tropical_auth
tropical_audit
tropical_inventory
tropical_sales
tropical_chat
```

Use plain `docker compose down` when local data must be preserved.
