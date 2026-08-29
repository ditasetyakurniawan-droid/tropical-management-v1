# Production-readiness roadmap

This is the prioritized engineering roadmap for the homelab production-like target. "Complete" means implemented and runtime-validated, not enterprise HA.

## Completed P0 baseline

### P0.1 Runtime reliability - COMPLETE

- `/livez` and `/readyz` contract;
- DB-backed readiness checks;
- startup/readiness/liveness probes;
- graceful SIGTERM/SIGINT;
- bounded application shutdown;
- request-scoped DB contexts and DB/network timeouts;
- bounded DB connection pools;
- fail-closed sensitive configuration in production-like environments.

### P0.2 Delivery-owned database migration - COMPLETE

- standalone `db-migrator`;
- versioned SQL by service target;
- `schema_migrations`, checksum and dirty-state protection;
- MySQL advisory locking and schema verification;
- Vault-injected migration credentials;
- Argo CD PreSync execution;
- Jenkins -> Harbor -> GitOps -> Argo CD delivery validated.

### P0.2.1 Local Compose migration parity - COMPLETE

- Compose uses a one-shot migrator;
- runtime services wait for successful migration;
- local chat owns `tropical_chat`;
- local bootstrap stays free of runtime startup DDL.

### P0.3 Resource/runtime guardrails - COMPLETE for active Tropical workloads

- CPU/memory requests and limits;
- Vault Agent request/limit envelope;
- namespace `LimitRange`;
- `runAsNonRoot` where applicable;
- `allowPrivilegeEscalation: false`;
- dropped Linux capabilities;
- `RuntimeDefault` seccomp profile;
- no HPA/PDB introduced before trustworthy metrics and replica strategy.

### P0.3.1 Traffic protection - COMPLETE

- API Gateway max in-flight requests;
- dedicated SSE concurrency budget;
- login account/global rate limiting;
- bounded limiter memory;
- chat global/per-user SSE connection caps;
- 429/503 overload contracts and tests.

## Final security sprint - DEFERRED BY PRODUCT DECISION

This work is intentionally paused while Release 1.1 Workforce & Shift Operations is validated. It remains a mandatory backlog, not a cancellation.

### Toolchain and supply chain

- upgrade Go from the current unsupported baseline to a supported release;
- align Docker/Jenkins Go versions;
- add `govulncheck`;
- add filesystem/dependency and built-image vulnerability scanning;
- pin CI tooling images rather than relying on floating `latest` tags.

### Network and least privilege

- separate runtime DB identities from migrator identity;
- runtime users get only required DML;
- migrator gets required DDL on owned schemas;
- default-deny NetworkPolicy with explicit ingress/egress rules;
- review Kubernetes service accounts/RBAC/token mounting;
- secret rotation runbook.

### Mandatory resume triggers

Resume this sprint before any of the following:

- external/public application exposure;
- material increase in cluster scale or replicas;
- new payment/finance integration;
- third-party employee/payroll integration;
- storage of more sensitive employee information;
- production environment activation.

## Active product milestone

**Release 1.1 Workforce & Shift Operations** is the current product priority. See [`product-workforce-operations.md`](product-workforce-operations.md).

The decision allows visible product progress while preserving the known platform debt explicitly in-repo.

## P1 Observability and operations

- centralized structured logs into ELK;
- HTTP/service/DB/workforce metrics into Prometheus/Grafana;
- latency/error/readiness/restarts/migration/traffic-limit dashboards;
- actionable alerts;
- request correlation / OpenTelemetry where useful;
- SLOs for critical user journeys.

## P1 Data/platform resilience

- define homelab MySQL RPO/RTO;
- automated backups and restore drill;
- test DB-unavailable and disk-full behavior;
- ingress/TLS/cert lifecycle;
- explicitly decide the acceptable MySQL SPOF.

## P1 Availability

- define replica strategy from observed load;
- rolling-update maxUnavailable/maxSurge;
- PDB only for workloads with >1 replica;
- anti-affinity/topology spread where useful;
- HPA only after requests and metrics are trustworthy;
- chat remains single replica until shared pub/sub exists.

## Product/engineering governance

For every milestone define user impact, technical risk, acceptance criteria, verification evidence, rollback path and operational ownership. Product features may proceed now, but platform debt above must remain visible and must be resumed at the defined safety triggers.
