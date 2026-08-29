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
| GET/POST | `/api/chat/messages` | General room history / send message |
| GET | `/api/chat/stream` | Authenticated real-time SSE stream |
| GET/POST/PATCH | `/api/workforce/*` | Shift, attendance, time-off, and checklist operations |

## RBAC

RBAC is enforced at the gateway and user administration is additionally enforced by `auth-service`.

- **Owner (`admin`)**: full access, including user/role mutation.
- **PIC (`auditor`)**: operational management for audit/issues and workforce, employee-directory reads needed for scheduling, operational sales entry, and inventory stock adjustments. PIC cannot mutate user accounts/roles, inventory master data, or supplier master data.
- **Karyawan (`staff`)**: personal workforce/self-service, General Live Chat, authenticated identity access, and the existing sales workflow permitted by the gateway. Karyawan no longer receives blanket read access to audit/inventory/dashboard data.
- Workforce service enforces identity-aware row scope in addition to gateway route authorization.


## Workforce & Shift Operations

All workforce endpoints require a valid JWT at the API Gateway. The gateway derives `X-User-ID`, `X-User-Name`, and `X-User-Role` from verified claims; clients cannot choose those downstream identity headers.

| Method | Path | Karyawan | PIC | Owner | Purpose |
|---|---|---:|---:|---:|---|
| GET | `/api/workforce/summary` | personal | team | team | day-level workforce KPIs |
| GET | `/api/workforce/shifts?from=&to=` | own | all | all | bounded shift window |
| POST | `/api/workforce/shifts` | no | yes | yes | publish shift |
| GET | `/api/workforce/attendance?from=&to=` | own | all | all | attendance history/pulse |
| POST | `/api/workforce/attendance/clock-in` | yes | yes | yes | clock into today's scheduled shift |
| POST | `/api/workforce/attendance/clock-out` | yes | yes | yes | close active attendance |
| GET | `/api/workforce/time-off` | own | all | all | time-off requests |
| POST | `/api/workforce/time-off` | yes | yes | yes | request leave/sick/permission |
| PATCH | `/api/workforce/time-off` | no | yes | yes | approve/reject request |
| GET | `/api/workforce/tasks?from=&to=` | team/assigned | all | all | shift checklist |
| POST | `/api/workforce/tasks` | no | yes | yes | create checklist item |
| PATCH | `/api/workforce/tasks` | assigned/team only | yes | yes | complete/reopen checklist |

Date query ranges are limited to 31 days. Time-off requests are limited to 14 days per request. Stations are currently `kitchen`, `prep`, `service`, `cashier`, `beverage`, and `steward`.

The current role codes remain `staff`, `auditor`, and `admin` for compatibility. Product labels are Karyawan, PIC, and Owner.

### Operational mutation boundary

To keep shift execution fast without giving the PIC ownership of reference data:

| Operation | Karyawan | PIC | Owner |
|---|---:|---:|---:|
| Record sale | yes | yes | yes |
| View inventory / suppliers | no | yes | yes |
| Adjust stock movement | no | yes | yes |
| Create inventory item | no | no | yes |
| Create supplier | no | no | yes |
| Mutate users / roles | no | no | yes |

The UI follows the same boundary. PIC sees stock-adjustment controls but not inventory/supplier master forms. Backend gateway authorization remains the source of truth.

## Live Chat identity

The API Gateway derives chat identity from verified JWT claims and overwrites trusted downstream headers. Clients only submit the message body; they cannot impersonate another display name or role through the chat payload.

```json
{
  "body": "Morning briefing dimulai pukul 09:00."
}
```

See `docs/live-chat.md` for the real-time stream design.

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

## Session Security Update

- JWT access tokens have an absolute lifetime of **30 minutes**.
- Browser auth state is stored in `sessionStorage`, not persistent `localStorage`.
- Closing the browser session removes the application login state.
- The frontend signs the user out after **15 minutes of inactivity**. Click, keyboard, scroll, touch, and route navigation reset the idle timer.
- `POST /api/auth/change-password` changes the currently authenticated user's password after verifying the current password. New passwords require at least 12 characters.
- A successful password change signs the browser out and requires a fresh login.

`BOOTSTRAP_ADMIN_PASSWORD` remains a bootstrap-only credential: it creates the initial admin when the account does not exist. Changing that Vault key does not overwrite an already-created user's password.
