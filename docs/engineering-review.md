# Engineering review and remediation summary

This document records the high-priority changes applied during the repository review. It is intended as a merge-review aid, not as a substitute for the Sonar Quality Gate or a production penetration test.

## Critical / high-priority findings remediated

| Area | Finding | Remediation |
|---|---|---|
| Sonar coverage | Test coverage could remain at 0% because report path manipulation and scanner configuration were split across Jenkins and Sonar properties. | Generate native `coverage.out`, generate a separately validated `coverage.sonar.out`, publish `go-test.json`, and keep all Sonar scope/report settings in `sonar-project.properties`. |
| Unit tests | Several tests exercised duplicate/fake handlers rather than production handlers, producing little useful production coverage. | Replaced/expanded tests to invoke real handlers and validation/middleware paths across gateway, auth, audit, inventory, sales, chat, dashboard, `httpx`, `logx`, and `dbx`. |
| Observability | Services only depended on process output, making incident correlation difficult. | Added rotating per-service file logs, stdout mirroring, HTTP access logs, panic recovery logs, and `X-Request-ID` correlation/propagation. |
| Information disclosure | Multiple HTTP 500 paths returned raw `err.Error()` values. | Internal errors are logged and clients receive generic 500 responses. |
| JWT dependency | `github.com/golang-jwt/jwt/v5` was pinned at 5.2.1. | Upgraded to 5.3.1 and hardened validation to HS256 + required expiration. |
| Next.js dependency | Frontend was pinned at Next.js 16.3.0. | Upgraded to 16.3.3 and updated the lockfile/native SWC package pins. |
| HTTP hardening | Services used basic `http.ListenAndServe` defaults. | Centralized hardened server defaults for header/read/idle timeout and maximum header size while preserving SSE compatibility. |
| Route matching | Prefix-only matching could classify lookalike gateway paths as valid service routes. | Added segment-boundary-aware prefix matching. |
| Data validation | Sales/audit handlers accepted invalid dates and negative operational values in some paths. | Added strict date/value validation and method restrictions on internal summary routes. |
| DB semantics | Duplicate-key handling was conflated with general database failure in places. | Added MySQL error-code classification so conflict and infrastructure errors map to different HTTP responses. |
| Frontend build reproducibility | Docker frontend dependency stage used `npm install`. | Switched to `npm ci` with committed lockfile. |

## CI behavior after remediation

The intended main-branch order is:

```text
checkout -> version -> Go test/race/coverage -> Next production build
-> Sonar scan + blocking Quality Gate -> immutable image build/push -> GitOps update
```

Coverage/test artifacts are archived by Jenkins for diagnostics. A local engineer can reproduce the quality workflow with `./scripts/verify-local.sh`.

## Residual risks / recommended follow-up

1. Add a frontend unit/component test harness and LCOV ingestion. Frontend source remains visible to Sonar and is intentionally not excluded from coverage calculations.
2. Replace the in-memory chat SSE fan-out with Redis/NATS before scaling chat-service beyond one replica.
3. Add integration tests against ephemeral MySQL (for example Testcontainers) for repository/SQL behavior, migrations, duplicate constraints, and transaction semantics.
4. Add centralized log aggregation (Loki/ELK/OpenSearch) and metrics/tracing (OpenTelemetry) in production; local rotating files are for node/container diagnostics, not the final observability platform.
5. Enforce secret injection through a production secret manager and rotate any credentials that have ever been committed or broadly shared.
6. Pin CI container images by digest where supply-chain policy requires fully immutable build tooling.
