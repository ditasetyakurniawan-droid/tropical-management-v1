# Local Compose networking

Only the browser-facing web application and API gateway are published to the host.

Published host ports:
- `3000` -> Next.js web
- `8080` -> API gateway

Internal-only services:
- MySQL: `mysql:3306`
- auth-service: `auth-service:8080`
- audit-service: `audit-service:8080`
- inventory-service: `inventory-service:8080`
- sales-service: `sales-service:8080`
- dashboard-service: `dashboard-service:8080`

This avoids local port collisions and more closely matches the production topology where internal services are reached through cluster networking rather than exposed directly to clients.
