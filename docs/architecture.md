# Architecture

## Application boundary

A microservice is defined here by an independent runtime, API contract, database ownership boundary and deployable image - not by having a separate Git repository.

```text
Next.js PWA
    |
API Gateway :8080
    |
    +-- auth-service       -> tropical_auth
    +-- audit-service      -> tropical_audit
    +-- inventory-service  -> tropical_inventory
    +-- sales-service      -> tropical_sales
    +-- chat-service       -> tropical_chat (production target)
    +-- dashboard-service  -> HTTP aggregation only
```

Production MySQL is external on `DB-dt (192.168.100.70)`. Each stateful service should receive a scoped DSN through Vault-injected configuration; credentials must never be committed.

## Client boundary

The browser communicates with the API Gateway. Backend microservices stay internal to the application network and should normally be exposed only through ClusterIP services in Kubernetes.

```text
Browser -> Web -> API Gateway -> owning microservice
```

## Live Chat boundary

`chat-service` owns chat history and real-time delivery. API Gateway validates the JWT and forwards trusted user identity headers. The browser cannot choose its displayed chat name or role.

The current SSE broker is in-memory, so multi-replica chat requires a future shared pub/sub layer such as Redis or NATS. Until then, production chat-service should use one replica.

## Repository strategy

Start with two repositories:

1. `tropical-management-v1` - application source, tests, Docker build definitions and Jenkinsfile.
2. `tropical-management-gitops` - Kubernetes/Kustomize manifests, environment overlays and Argo CD Applications/ApplicationSets.

Split individual services into separate source repositories later only when independent team ownership/release cadence makes the extra repository overhead worthwhile.
