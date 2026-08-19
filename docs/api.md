# API surface

All client traffic goes through `api-gateway:8080`.

| Method | Path | Purpose |
|---|---|---|
| POST | `/api/auth/login` | Public login |
| GET | `/api/auth/me` | Current JWT claims |
| GET/POST/PATCH | `/api/users` | Admin user lifecycle and role management |
| GET/POST | `/api/audits` | Audit checklist records |
| GET/POST/PATCH | `/api/issues` | Audit finding / corrective action workflow |
| GET/POST | `/api/inventory` | Inventory items |
| POST | `/api/inventory/adjust` | Transactional stock adjustment with reason |
| GET | `/api/inventory/movements` | Stock movement ledger |
| GET/POST | `/api/suppliers` | Supplier management |
| GET/POST | `/api/sales` | Sales entries |
| GET | `/api/dashboard` | Sales/audit/inventory metrics |

## RBAC

RBAC is enforced at the gateway and user administration is additionally enforced by `auth-service`.

- **Admin**: all access.
- **Auditor**: may mutate audit and issue endpoints.
- **Staff**: may create/update sales data allowed by the gateway.
- Authenticated roles may read operational data.

## Issue workflow

Supported severities:

```text
low | medium | high | critical
```

Supported statuses:

```text
open | in_progress | resolved | verified | closed
```

Phase-3 issue payloads may contain:

```json
{
  "id": 12,
  "audit_id": 4,
  "title": "Cold storage temperature outside SOP",
  "severity": "critical",
  "status": "in_progress",
  "assigned_to": "Kitchen Supervisor",
  "due_date": "2026-08-22",
  "corrective_action": "Calibrate unit and record 4-hour temperature verification."
}
```

## Inventory adjustment

Stock adjustments are transactional and cannot reduce stock below zero.

```json
{
  "item_id": 5,
  "delta": -3.5,
  "reason": "Kitchen usage"
}
```

Every successful adjustment creates a row in `stock_movements`.
