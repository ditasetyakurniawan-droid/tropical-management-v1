# Local development

## Full stack

```bash
docker compose up --build
```

First startup downloads images/modules/packages and initializes four local MySQL databases. Open `http://localhost:3000`.

Local bootstrap admin:

```text
admin@tropical.local
ChangeThis123!
```

Never reuse this password outside local development.

## Reset local data

```bash
docker compose down -v
docker compose up --build
```

## Health checks

```bash
curl http://localhost:8080/healthz
curl http://localhost:8081/healthz
curl http://localhost:8082/healthz
curl http://localhost:8083/healthz
curl http://localhost:8084/healthz
curl http://localhost:8085/healthz
```

## Login API example

```bash
curl -s http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@tropical.local","password":"ChangeThis123!"}'
```
