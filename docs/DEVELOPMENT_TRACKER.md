# Tropical Management Development Tracker

> Canonical engineering tracker for Tropical Steak House Internal OS.

Last updated: 2026-08-30

---

## 1. Current Release

| Field | Current State |
|---|---|
| Release | `1.1.0-beta1` |
| Application branch | `main` |
| Application revision | `ed910abda3bd05e1c9778cc0866ce4a082cba4c1` |
| Workforce image | `harbor-dt.co.id/devops-apps/tropical-workforce:1.1.0-beta1-ed910abda3bd` |
| Target environment | `test-app` |
| Deployment state | **Active** |
| Release stage | **Functional Validation / UAT** |

### Release conclusion

Release 1.1 has completed its development, CI, database migration,
GitOps delivery, smoke-test, and runtime activation gates.

The release is currently deployed in the internal test environment.

Tahap berikutnya bukan infrastructure debugging lagi.
Focus berikutnya adalah functional validation dan user acceptance testing
untuk workflow Karyawan, PIC, dan Owner.

---

## 2. Release 1.1 Scope

Release 1.1 introduces **Workforce & Shift Operations**.

### Delivered capabilities

- Workforce service
- Employee shift scheduling
- Clock-in and clock-out workflow
- Attendance history
- Time-off / permission requests
- Shift checklist
- Seven-day team coverage
- Manager approval queue
- Role-aware navigation
- Role-based API authorization
- Workforce database migrations
- Gateway integration
- Vault-based database configuration

---

## 3. Role Model

The backend keeps stable role identifiers while the product uses
restaurant-friendly labels.

| Backend Role | Product Role | Primary Responsibility |
|---|---|---|
| `staff` | Karyawan | Personal shift and daily operations |
| `auditor` | PIC | Team and shift operations |
| `admin` | Owner | Operational and administrative control |

### Karyawan

Expected access:

- own shift
- clock-in / clock-out
- own attendance history
- submit time-off request
- assigned shift checklist
- allowed operational workflows

Karyawan must not receive Owner/PIC administrative privileges.

### PIC

Expected access:

- team schedule
- attendance monitoring
- create shifts
- create shift tasks
- review time-off requests
- operational inventory actions
- audit / issue workflows

PIC must not manage account roles or unrestricted master data.

### Owner

Expected access:

- all PIC operational capabilities
- user lifecycle and role management
- inventory and supplier master data
- executive visibility

---

## 4. API Surface

Workforce API:

```text
GET   /api/workforce/summary
GET   /api/workforce/shifts
POST  /api/workforce/shifts

GET   /api/workforce/attendance
POST  /api/workforce/attendance/clock-in
POST  /api/workforce/attendance/clock-out

GET   /api/workforce/time-off
POST  /api/workforce/time-off
PATCH /api/workforce/time-off

GET   /api/workforce/tasks
POST  /api/workforce/tasks
PATCH /api/workforce/tasks
```

Health endpoints:

```text
GET /livez
GET /readyz
```

---

## 5. Verification Status

| Gate | Status |
|---|---|
| Backend tests | ✅ Passed |
| Frontend tests | ✅ Passed |
| Go vet | ✅ Passed |
| Frontend production build | ✅ Passed |
| Dependency audit | ✅ Passed |
| SonarQube | ✅ Passed |
| Container build | ✅ Passed |
| Harbor push | ✅ Passed |
| GitOps image update | ✅ Passed |
| Workforce DB migration | ✅ Passed |
| Argo CD sync | ✅ Passed |
| Argo CD health | ✅ Healthy |
| Workforce deployment | ✅ 1/1 |
| `/livez` | ✅ HTTP 200 |
| `/readyz` | ✅ HTTP 200 |
| Workforce UI route | ✅ Loaded |
| Owner UI mode | ✅ Verified |
| Full functional/UAT | ⏳ In progress |

---

## 6. Functional Validation Checklist

This is the current release gate.

### Owner

- [ ] Open Workforce dashboard
- [ ] Create employee shift
- [ ] Create shift checklist
- [ ] Review time-off request
- [ ] Verify dashboard counters
- [ ] Verify seven-day coverage
- [ ] Verify Owner-only administration

### PIC

- [ ] Open Workforce dashboard
- [ ] View team schedule
- [ ] Create shift
- [ ] Create checklist task
- [ ] Approve/reject time-off request
- [ ] Verify prohibited Owner-only actions return forbidden

### Karyawan

- [ ] View own shift
- [ ] Clock in
- [ ] Clock out
- [ ] View attendance history
- [ ] Submit permission / leave request
- [ ] Complete assigned task
- [ ] Verify PIC/Owner functions are inaccessible

---

## 7. Database State

Workforce uses a dedicated schema:

```text
tropical_workforce
```

Migration targets currently managed by the centralized migrator:

```text
auth
audit
inventory
sales
chat
workforce
```

Release 1.1 migration completed successfully for all six targets.

No database credentials or secret values belong in this repository.

---

## 8. Deployment Model

```text
GitHub main
    ↓
Jenkins
    ↓
Tests + SonarQube
    ↓
Container build
    ↓
Harbor
    ↓
GitOps image update
    ↓
Argo CD
    ↓
PreSync DB Migrator
    ↓
Kubernetes test-app
```

The workforce workload is now active in `test-app`.

Production remains intentionally dormant.

---

## 9. Known Technical Debt

These items are **not blockers for internal Release 1.1 validation**,
but they must remain visible in the engineering backlog.

### Security hardening

Deferred final P0 security work:

- supported Go toolchain upgrade
- `govulncheck`
- dependency and container image scanning
- Kubernetes NetworkPolicy
- ServiceAccount / RBAC review
- separate runtime and migration database privileges
- secret rotation runbook

Resume this security gate before:

- public/external exposure
- production activation
- materially more sensitive employee data
- major scaling
- payment integration
- external payroll/employee integration

### Product / backend follow-up

Review before a later stable release:

- date-range inclusive boundary handling
- maximum time-off duration boundary
- past time-off validation
- employee ID/name validation during shift creation
- overnight shift support

### Platform technical debt

Kustomize currently reports deprecated syntax for:

- `commonLabels`
- `patchesStrategicMerge`

These warnings are non-blocking but should be migrated later.

---

## 10. Release State Model

Use this lifecycle for future development tracking:

```text
PLANNED
   ↓
IN DEVELOPMENT
   ↓
LOCAL QUALITY PASSED
   ↓
CI PASSED
   ↓
MERGED TO MAIN
   ↓
MIGRATED
   ↓
DEPLOYED
   ↓
SMOKE PASSED
   ↓
FUNCTIONAL / UAT
   ↓
RELEASE ACCEPTED
```

Current Release 1.1 state:

```text
FUNCTIONAL / UAT
```

---

## 11. Next Milestone

After Release 1.1 functional acceptance:

1. Close Release 1.1 UAT findings.
2. Update release status to **RELEASE ACCEPTED**.
3. Resume deferred P0 security hardening.
4. Start the next product increment from a documented product backlog.

---

## Documentation Language Convention

Engineering documentation should be **English-first**.

Bahasa Indonesia boleh digunakan untuk:

- operational explanation
- restaurant-specific terminology
- user-facing workflow clarification

Technical names, architecture concepts, API terminology, release states,
and infrastructure documentation should remain in English for consistency.
