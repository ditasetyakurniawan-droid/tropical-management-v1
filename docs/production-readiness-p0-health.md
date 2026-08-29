# P0 Production Readiness: health probes and graceful shutdown

This change separates process liveness from traffic readiness and makes backend services handle Kubernetes termination gracefully.

## Endpoint contract

All Go backend services expose:

- `GET /healthz`: legacy compatibility endpoint. Existing external checks can keep using it during migration.
- `GET /livez`: process liveness only. It does not check MySQL or downstream services.
- `GET /readyz`: traffic readiness.
  - auth, audit, inventory, sales, and chat perform a bounded MySQL `PingContext`.
  - API gateway and dashboard report ready after successful process initialization and do not couple readiness to all downstream services.

Readiness dependency failures return HTTP 503 without returning the underlying database error to the caller.

## Graceful shutdown

All Go backend services now handle `SIGINT` and `SIGTERM` through `httpx.RunServer`.

On Kubernetes pod termination:

1. Kubernetes sends SIGTERM.
2. The HTTP server starts graceful shutdown.
3. In-flight requests receive up to 20 seconds to complete.
4. If graceful shutdown exceeds the timeout, the server is force-closed.
5. The Kubernetes workload should use `terminationGracePeriodSeconds: 30` so the application shutdown budget fits inside the pod termination budget.

## Recommended Kubernetes probes

For every Go backend container on port 8080:

```yaml
startupProbe:
  httpGet:
    path: /livez
    port: 8080
  periodSeconds: 5
  timeoutSeconds: 2
  failureThreshold: 24
  successThreshold: 1
readinessProbe:
  httpGet:
    path: /readyz
    port: 8080
  periodSeconds: 5
  timeoutSeconds: 2
  failureThreshold: 3
  successThreshold: 1
livenessProbe:
  httpGet:
    path: /livez
    port: 8080
  periodSeconds: 10
  timeoutSeconds: 2
  failureThreshold: 3
  successThreshold: 1
```

The 120-second startup budget is intentional. Current DB-backed services can wait for MySQL before starting the HTTP server, so liveness must not kill them while startup is still in progress.

## Verification before merge

Run from the application repository root:

```bash
./scripts/verify-local.sh
```

At minimum:

```bash
gofmt -l internal services
go test -race ./...
go vet ./...
```

After Jenkins publishes the new immutable image and ArgoCD deploys it, verify:

```bash
kubectl -n test-app get pods -o wide
kubectl -n test-app get deploy
kubectl -n test-app describe pod <one-backend-pod>
```

The pod description should show:

- Startup: `/livez`
- Readiness: `/readyz`
- Liveness: `/livez`

Then test a controlled restart:

```bash
kubectl -n test-app rollout restart deployment/tropical-auth
kubectl -n test-app rollout status deployment/tropical-auth --timeout=180s
kubectl -n test-app get pods -w
```

Repeat for the remaining services after auth is confirmed healthy.
