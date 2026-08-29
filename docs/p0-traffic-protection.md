# P0 Traffic Protection

This guardrail bounds expensive or long-lived traffic before feature growth. It
is intentionally in-process and dependency-free for the current single-replica
homelab deployment.

## Protections

### API gateway concurrency

`GATEWAY_MAX_IN_FLIGHT` limits concurrent normal API requests. Saturation fails
fast with HTTP 503 and `Retry-After` instead of building an unbounded in-process
queue. Long-lived `/api/chat/stream` requests use a separate
`GATEWAY_MAX_SSE_IN_FLIGHT` budget so they cannot consume every normal API slot.
Chat-service then applies its stricter per-user cap. Health/readiness endpoints
are registered outside the gateway handler and remain available during API
saturation.

Defaults: `100` normal requests and `100` SSE streams.

### Login token buckets

Auth login uses two token buckets before database and bcrypt work:

- per normalized account: `AUTH_LOGIN_RATE_LIMIT_ATTEMPTS`, default `5`
- global safeguard: `AUTH_LOGIN_GLOBAL_RATE_LIMIT_ATTEMPTS`, default `100`
- refill window: `AUTH_LOGIN_RATE_LIMIT_WINDOW`, default `1m`
- bounded account-key memory: `AUTH_LOGIN_RATE_LIMIT_MAX_KEYS`, default `10000`

Rejected attempts return HTTP 429 and `Retry-After`. The account identifier is
not written to rate-limit logs. Malformed JSON is rejected before the limiter and
does not reach the database/bcrypt path.

### Chat SSE connections

Chat limits active SSE streams globally and per trusted gateway user ID:

- `CHAT_MAX_SSE_CONNECTIONS`, default `100`
- `CHAT_MAX_SSE_CONNECTIONS_PER_USER`, default `3`

Rejected streams return HTTP 429 and `Retry-After`. The limiter map is bounded by
the maximum number of active connections.

## Scaling note

All limits are process-local. That is appropriate while auth/chat/gateway remain
single-replica. If these services scale horizontally, cluster-wide enforcement
should move to an ingress/gateway control and/or a shared limiter store. Do not
assume these defaults are cluster-global after replica scaling.

## Deliberate exclusions

This change does not add IP-based limiting because trusted-proxy and client-IP
semantics are not yet formalized. It also does not add Redis, HPA, NetworkPolicy,
or DB-migrator lifecycle changes.
