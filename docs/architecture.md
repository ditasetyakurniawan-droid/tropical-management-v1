# Architecture

## 1. Architectural principles

Tropical Management uses independently deployable services while retaining a source monorepo. Service ownership is based on runtime/API/data boundaries. Database tables are not shared across service ownership boundaries.

The system follows these operational rules:

1. Browser traffic enters through Web/API Gateway rather than directly reaching backend services.
2. Each stateful service owns one MySQL schema.
3. Runtime configuration and credentials come from Vault in Kubernetes.
4. Runtime pods do not own schema evolution.
5. Database migration must succeed before application rollout.
6. GitOps is the deployment source of truth.
7. Health checks distinguish process liveness from traffic readiness.

## 2. Logical application topology

```text
Browser
  |
  +--> Next.js Web
          |
          +--> API Gateway
                 |
                 +--> auth-service -------> tropical_auth
                 +--> audit-service ------> tropical_audit
                 +--> inventory-service --> tropical_inventory
                 +--> sales-service ------> tropical_sales
                 +--> dashboard-service --> service HTTP APIs
                 `--> chat-service -------> tropical_chat
```

The API Gateway is the main application API boundary. Backend Kubernetes Services should remain internal unless there is a deliberate exception.

## 3. Service ownership

| Service | Owns | Must not own |
|---|---|---|
| auth | identity, JWT, users, RBAC | audit/inventory/sales/chat tables |
| audit | audits, issues/findings | users or inventory tables |
| inventory | suppliers, items, movements | sales tables |
| sales | sales entries | inventory tables |
| chat | chat history + real-time SSE broker | auth tables |
| dashboard | aggregation | persistent business tables |
| gateway | routing/authz boundary | business persistence |

Cross-service data access should happen through service APIs, not direct SQL against another service's schema.

## 4. Database ownership

Production-like MySQL currently runs externally on `DB-dt (192.168.100.70)`.

```text
auth-service       -> tropical_auth
audit-service      -> tropical_audit
inventory-service  -> tropical_inventory
sales-service      -> tropical_sales
chat-service       -> tropical_chat
```

`chat-service` must not point to `tropical_auth`. During P0.2 this misconfiguration was detected by migration checksum protection and corrected by creating `tropical_chat` and updating the Vault DSN.

## 5. Database lifecycle

Schema evolution is a delivery concern:

```text
Argo CD Sync
   |
   +--> PreSync db-migrator
   |      +--> read Vault DSN files
   |      +--> acquire per-target MySQL lock
   |      +--> validate schema_migrations
   |      +--> apply pending versioned SQL
   |      +--> verify expected schema
   |      `--> exit 0 only when every target is clean
   |
   `--> Deployment rollout
```

A failed migration stops the release before new application pods roll out. See [`database-migrations.md`](database-migrations.md).

## 6. Runtime health lifecycle

All Go backends expose `/healthz`, `/livez`, and `/readyz`.

- Liveness answers whether the process is alive and does not depend on MySQL or downstream APIs.
- Readiness answers whether the pod can safely receive traffic.
- DB-backed services use `PingContext` for readiness.
- SIGTERM/SIGINT is handled by the shared HTTP server lifecycle.
- Kubernetes gives the pod 30 seconds termination grace; application shutdown budget is 20 seconds.

This prevents dependency outages from becoming restart loops and reduces dropped in-flight requests during rollouts.

## 7. Secret/configuration boundary

Kubernetes runtime secrets are sourced from HashiCorp Vault through Vault Agent injection. Applications consume injected files rather than committing plaintext credentials in GitOps.

The migration Job uses `agent-pre-populate-only` so Vault renders the files before the migrator starts and does not leave a permanent sidecar that would prevent Job completion.

Current transitional limitation: runtime and migration database privileges are not yet fully separated. The target state is a DML-only runtime identity plus a dedicated DDL-capable migrator identity.

## 8. Deployment boundary

```text
tropical-management-v1
  source + tests + Docker + migrations + Jenkinsfile
             |
             v
         immutable images
             |
             v
tropical-management-gitops
  Kustomize + Vault patches + Argo CD hooks
             |
             v
          Kubernetes
```

Jenkins can update desired image tags in GitOps. It should not use `kubectl apply` as the long-term production deployment mechanism.

## 9. Live Chat availability constraint

The chat history is persistent in MySQL, but the SSE broker is process-local. Horizontal chat replicas would not share fan-out state. Keep one chat replica until shared pub/sub such as Redis or NATS is introduced.

## 10. Current known gaps

The architecture is production-like, not production-complete. The next important gaps are:

- align fresh local Docker Compose bootstrap with the standalone migrator;
- define requests/limits and availability policy per workload;
- split runtime DB users from migration DB user and reduce privileges;
- add PodDisruptionBudgets and NetworkPolicies;
- centralize structured logs/metrics/traces and alerting;
- remove MySQL single-node SPOF or explicitly accept it for homelab;
- mature ingress/TLS and external exposure policy;
- add shared pub/sub before chat horizontal scaling.
