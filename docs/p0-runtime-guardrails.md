# P0 Runtime Guardrails

This change hardens the application before the next major feature milestone.

## Scope

- Non-local environments fail fast when DB DSNs, JWT secret, or bootstrap admin credentials are missing.
- Secrets can be supplied either through `KEY` or `KEY_FILE`; Kubernetes should prefer `*_FILE` from Vault Agent injection.
- MySQL pool sizes and network/query timeouts are bounded and configurable.
- HTTP request database work uses request-derived contexts.
- Chat local default DSN points to `tropical_chat`.
- The web runtime runs as the non-root `node` user.

## Environment contract

Kubernetes overlays must set `APP_ENV` explicitly.

Supported values:

- `local`
- `development`
- `test`
- `test-app`
- `staging`
- `production`

Only `local` and `development` allow source-code development fallbacks for sensitive values. All other environments require the value or its `_FILE` variant.

## Database runtime controls

| Variable | Default |
| --- | ---: |
| `DB_MAX_OPEN_CONNS` | `10` |
| `DB_MAX_IDLE_CONNS` | `5` |
| `DB_CONN_MAX_LIFETIME` | `3m` |
| `DB_CONN_MAX_IDLE_TIME` | `1m` |
| `DB_PING_ATTEMPTS` | `30` |
| `DB_PING_DELAY` | `2s` |
| `DB_PING_TIMEOUT` | `3s` |
| `DB_QUERY_TIMEOUT` | `5s` |
| `DB_CONNECT_TIMEOUT` | `3s` |
| `DB_READ_TIMEOUT` | `5s` |
| `DB_WRITE_TIMEOUT` | `5s` |

These are conservative bootstrap values for a homelab. Tune them from measured latency, database `max_connections`, replica count, and connection-wait metrics before horizontal scaling.

## Rollout order

1. Merge and build this application revision.
2. Ensure the active GitOps overlay sets `APP_ENV=test-app` and keeps Vault `*_FILE` injections.
3. Apply explicit container resource requests/limits in GitOps.
4. Render the overlay locally with Kustomize.
5. Deploy to `test-app`.
6. Check startup, readiness, logs, Vault injection, DB connectivity, login, CRUD flows, and chat SSE.
7. Observe memory/CPU and DB connection behavior before tuning resource values.

## Intentionally deferred

The following belong in separate P0/P1 changes so rollback remains simple:

- rate limiting and max in-flight request policy
- NetworkPolicy
- PDB/HPA and replica changes
- Go major-version upgrade
- Prometheus application metrics
- read-only root filesystem
