# Product direction: Tropical Steak House Internal OS

## Product context

Tropical Management is being shaped as the internal operating system for **Tropical Steak House, Temanggung**, not as a generic restaurant CRUD application.

Public business references used for product discovery indicate a single-location restaurant on Jl. Gatot Subroto, Temanggung, with a strong steak/chicken/pasta/rice menu mix, lunch and dinner operations, delivery/takeaway/booking support, and operating hours extending into the evening. Public trademark material also associates the Tropical Steak House identity with dark green, gold, and white. These signals support an internal product centered on shift readiness, consistent service quality, food-stock discipline, and fast manager coordination.

This repository does not scrape or store customer review data. Public references are used only to shape product priorities and interaction design.

## Product benchmark

The product direction deliberately borrows proven operating patterns rather than copying screens:

| Reference | Pattern adopted | Tropical adaptation |
|---|---|---|
| 7shifts | role-based schedule, employee self-service, time off, stations, team communication | simple 7-day scheduling by restaurant station, PIC approval, employee shift home |
| Restaurant365 | workforce + restaurant operations in one management surface | people metrics live beside sales, quality, and stock for PIC/Owner |
| Toast | integrated restaurant operations and inventory workflows | keep sales/inventory as operational modules that can later feed staffing and food-cost decisions |
| Mekari Talenta | attendance, shift, leave, employee self-service | clock in/out, personal attendance history, time-off requests with supervisor approval |
| Moka / ESB | restaurant inventory/procurement and back-office discipline | future recipe/HPP, purchase order, receiving, waste and stock-count workflows |

The design principle is **one internal workspace, different information density by responsibility**.

## Access personas

The backend keeps the existing technical role codes to avoid a destructive auth migration. The UI exposes business language instead:

| Technical role | Product persona | Primary responsibility |
|---|---|---|
| `staff` | Karyawan | execute today's shift and assigned work |
| `auditor` | PIC | run the shift, maintain service/quality discipline, approve operational requests |
| `admin` | Owner | business oversight, access management, full operational control |

### Karyawan view

Karyawan should not land on an executive dashboard. Their workspace prioritizes:

- today's shift and station;
- clock in / clock out;
- assigned or team checklist;
- personal attendance history;
- time-off / permission request;
- team chat;
- operational sales entry only where the current coarse role model permits it.

### PIC view

PIC receives operational control, not account administration:

- seven-day team schedule;
- station coverage;
- team attendance pulse;
- approve/reject time-off requests;
- create and monitor shift checklists;
- audit/issues operational controls;
- inventory visibility plus stock-adjustment recording for receiving, usage, waste, and correction;
- sales visibility plus operational sales entry;
- read employee directory for scheduling;
- no user/role mutation;
- no inventory-master or supplier-master mutation. Those remain Owner-controlled reference data.

### Owner view

Owner receives everything available to PIC plus:

- executive sales, quality, stock and workforce summary;
- user lifecycle and role management;
- full business access.

## Release 1.1: Workforce & Shift Operations

This release adds a dedicated `workforce-service` and `tropical_workforce` schema.

### Functional scope

1. **Shift scheduling**
   - PIC/Owner publishes shifts.
   - Assignment includes date, start/end time and restaurant station.
   - Current stations: Kitchen, Preparation, Service, Kasir, Beverage, Steward.

2. **Attendance**
   - Karyawan clocks in only when a scheduled shift exists for the current day.
   - One active attendance record per shift/day is enforced by persistence rules.
   - Clock out closes the active attendance record.

3. **Time off / permission self-service**
   - Karyawan submits leave, sickness or permission request.
   - PIC/Owner approves or rejects.
   - Request window is bounded and validated.

4. **Shift checklist / standard work**
   - PIC/Owner creates station tasks.
   - Tasks can target one employee or the whole team.
   - Karyawan can update only tasks assigned to them or the team.
   - PIC/Owner can reopen/complete tasks for operational control.

5. **Role-aware home experience**
   - Karyawan receives a personal day view.
   - PIC receives shift-command information.
   - Owner receives business intelligence plus people operations.

## Data and service ownership

```text
workforce-service -> tropical_workforce

shifts
attendance
time_off_requests
shift_tasks
```

The workforce service trusts identity only from API Gateway headers derived from a verified JWT. Browser-supplied identity headers are overwritten at the gateway.

## Product rules

- Managers are `admin` or `auditor` for workforce operations.
- Only `admin` may mutate application users and roles.
- Karyawan data queries are scoped to the authenticated employee where personal data is involved.
- Time ranges are bounded to protect DB/query cost.
- New workforce mutations use validated JSON and bounded DB contexts.
- No payroll calculation is introduced in this release. Payroll is compliance-sensitive and should be integrated/exported later rather than casually reimplemented.

## Next product backlog after 1.1 review

Priority is based on restaurant impact and implementation readiness.

### P1 Product: food cost and purchasing

- recipe/BOM per menu item;
- ingredient consumption model;
- purchase order and receiving;
- supplier price history;
- stock count and variance;
- waste/spoilage log;
- food-cost and theoretical-vs-actual usage.

This is the strongest next operations feature because steak operations are sensitive to meat yield, storage, portion consistency and purchase-price variance.

### P1 Product: manager logbook and service recovery

- opening/closing manager log;
- equipment/maintenance notes;
- event/private booking handover;
- guest complaint/service recovery record;
- handover acknowledgement between PIC shifts.

### P1 Product: workforce maturity

- recurring availability;
- shift swap/open-shift request with PIC approval;
- late/no-show status;
- break and overtime policy fields;
- department/job-position scopes beyond the current three coarse application roles;
- training/SOP acknowledgement and competency checklist.

### P2 Product: reservation and table operations

Only after internal people/inventory flows are stable:

- reservation book;
- wait list;
- table/guest notes;
- booking handover to service team.

## Platform work intentionally deferred

The product team chose to ship visible operational value before the final security-hardening sprint. These items remain mandatory platform backlog and are **not cancelled**:

- supported Go toolchain upgrade;
- `govulncheck` and container/dependency vulnerability gates;
- NetworkPolicy and Kubernetes service-account review;
- runtime DB identity vs migrator DB identity separation;
- secret rotation runbook;
- resource tuning from observed metrics;
- central metrics/alerting and backup/restore drill.

Resume the final security sprint **before public/external exposure, major horizontal scale, new third-party integrations, or handling materially more sensitive employee data**.

## Product acceptance criteria for Release 1.1

- Karyawan cannot see Owner access-management navigation.
- PIC cannot mutate users/roles or inventory/supplier master data.
- PIC can record operational sales and stock adjustments.
- Owner can manage users/roles and inventory/supplier master data.
- Karyawan can see only their personal shifts/attendance/time-off plus team/assigned checklist.
- PIC/Owner can schedule shifts and review requests.
- Workforce DB migration is versioned and owned by `db-migrator`.
- Gateway has an explicit workforce route and tested role policy.
- local Docker Compose can bootstrap the workforce schema and service.
- backend race tests, vet and frontend coverage/build must pass before merge.
- Sonar Quality Gate remains a release blocker.

## Research references

Public product discovery references, accessed August 2026:

- Tropical Steak House, Restaurant Guru: https://restaurantguru.com/Tropical-Steak-House-Temanggung
- Indonesian DGIP trademark bulletin, Tropical Steak House label/application.
- 7shifts restaurant scheduling/workforce documentation: https://www.7shifts.com/
- Restaurant365 restaurant operations/workforce platform: https://www.restaurant365.com/
- Toast restaurant platform: https://pos.toasttab.com/
- Mekari Talenta employee self-service and attendance documentation: https://www.talenta.co/
- Moka POS/back-office: https://www.mokapos.com/
- ESB restaurant technology: https://www.esb.id/
