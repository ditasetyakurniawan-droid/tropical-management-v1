# Production-readiness roadmap

This is a prioritized engineering roadmap for the homelab production-like target. "Complete" means implemented and runtime-validated, not that the system has enterprise HA guarantees.

## P0.1 Runtime reliability - COMPLETE

- `/livez` and `/readyz` contract
- DB-backed readiness checks
- startup/readiness/liveness probes
- graceful SIGTERM/SIGINT
- 20s application shutdown budget
- 30s Kubernetes termination grace
- controlled rollout validation

## P0.2 Delivery-owned database migration - COMPLETE on Kubernetes

- standalone `db-migrator`
- versioned SQL by service target
- `schema_migrations`
- checksum enforcement
- dirty-state protection
- MySQL advisory locking
- schema verification
- Jenkins migrator image
- Vault-injected migration credentials
- Argo CD PreSync execution
- fail-closed migration behavior validated
- `tropical_chat` ownership corrected
- runtime startup migration calls disabled
- automatic Jenkins -> Harbor -> GitOps -> Argo CD -> PreSync -> rollout E2E validated

## P0.2.1 Local Compose parity - NEXT

Acceptance criteria:

- fresh `docker compose` volume can initialize all schemas using the same migrator implementation;
- Compose has a one-shot migrator or explicit developer migration command;
- local `chat-service` uses `tropical_chat`;
- runtime services remain free of startup DDL;
- local README/commands reflect the new bootstrap lifecycle.

## P0.3 Workload resources and availability

Define per-service:

- CPU/memory requests and limits based on observed baseline;
- replica strategy;
- rolling-update maxUnavailable/maxSurge;
- PDB where replicas > 1;
- anti-affinity/topology-spread where useful;
- HPA only after requests and metrics are trustworthy.

Do not blindly set every service to multiple replicas. `chat-service` remains single replica until shared pub/sub exists.

## P0.4 Security and least privilege

- separate runtime DB identities from migrator identity;
- runtime users get only required DML;
- migrator gets required DDL on owned schemas;
- non-root/read-only-root-filesystem/securityContext where compatible;
- NetworkPolicy for namespace ingress/egress boundaries;
- review Kubernetes service accounts/RBAC;
- secret rotation runbook;
- image/dependency vulnerability scanning policy.

## P1 Observability and operations

- centralized structured application logs into ELK;
- service/HTTP/DB metrics into Prometheus/Grafana or equivalent;
- dashboards for latency/error/readiness/restarts/migration failures;
- alerting with actionable thresholds;
- OpenTelemetry traces/request correlation where it provides debugging value;
- SLOs for critical user journeys.

## P1 Data/platform resilience

- explicitly decide MySQL homelab RPO/RTO;
- automated backups and restore drill;
- remove/mitigate single DB-host SPOF if higher availability is a goal;
- test disk-full and DB-unavailable behavior;
- ingress/TLS/cert lifecycle;
- external exposure and firewall policy.

## P1 Chat horizontal scale

Introduce Redis/NATS or another shared pub/sub layer before `chat-service` replica count exceeds one. Add reconnect/load testing for SSE.

## Product/engineering governance

For every milestone define:

- user/business impact;
- technical risk being reduced;
- acceptance criteria;
- verification evidence;
- rollback path;
- operational ownership.

Avoid adding features that materially increase operational surface before the P0 reliability/security gaps are closed unless the feature is required for product validation.
