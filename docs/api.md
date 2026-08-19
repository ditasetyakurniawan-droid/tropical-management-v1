# MVP API surface

All client traffic goes through `api-gateway:8080`.

| Method | Path | Purpose |
|---|---|---|
| POST | `/api/auth/login` | Public login |
| GET | `/api/auth/me` | Current JWT claims |
| GET/POST | `/api/users` | Admin role management |
| GET/POST | `/api/audits` | Audit checklist records |
| GET/POST/PATCH | `/api/issues` | Audit issue tracker |
| GET/POST | `/api/inventory` | Inventory items |
| POST | `/api/inventory/adjust` | Stock adjustment |
| GET/POST | `/api/suppliers` | Supplier management |
| GET/POST | `/api/sales` | Sales entries |
| GET | `/api/dashboard` | Sales/audit/inventory metrics |

RBAC is enforced at the gateway and user administration is additionally enforced by auth-service. Admin has all access. Auditor may mutate audit/issue endpoints. Staff may enter sales. Authenticated roles may read operational data.
