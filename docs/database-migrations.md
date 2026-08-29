# Database migrations

## Decision

Database schema changes are executed by a standalone `tropical-db-migrator` before Kubernetes application rollout. DB-backed runtime services do not execute schema migrations during normal startup.

This separates schema lifecycle from pod lifecycle and makes migration failure block a release before new application code is exposed.

## Targets

```text
auth       -> tropical_auth
audit      -> tropical_audit
inventory  -> tropical_inventory
sales      -> tropical_sales
chat       -> tropical_chat
```

Each target has an independent `schema_migrations` history.

## Layout

```text
internal/migrate/
  migrate.go
  sql/
    auth/
    audit/
    inventory/
    sales/
    chat/
services/db-migrator/
```

Baseline versions currently establish the service-owned tables. New changes must use the next monotonically increasing version in the owning target.

## Metadata safety

`schema_migrations` tracks:

- `version`
- `name`
- SHA-256 `checksum`
- `dirty`
- `started_at`
- `applied_at`

Safety behavior:

- an applied version with a different checksum fails closed;
- a dirty version fails closed;
- a per-service MySQL advisory lock prevents concurrent migration execution;
- schema verification runs after migrations;
- all owned targets must complete before the migrator exits 0.

## Authoring a migration

1. Identify the owning service/schema.
2. Create `internal/migrate/sql/<target>/NNNNNN_description.sql`.
3. Keep the migration deterministic and forward-oriented.
4. Do not modify any migration that may already have been applied.
5. Add/update migrator tests for parsing, failures, checksum behavior, or schema verification as needed.
6. Run:

```bash
make fmt-check
make test
make vet
```

7. Merge normally and let Jenkins/Argo CD execute the release path.

## Compatibility rule

Prefer expand/contract over destructive one-step changes.

Example:

```text
release A: ADD new nullable column/table/index
release B: app writes/reads compatible shape + optional backfill
release C: remove old shape only after rollback window is closed
```

Avoid migrations that make the immediately previous application image unable to run unless an explicit maintenance window/rollback plan exists.

## Kubernetes execution

Argo CD runs `tropical-db-migrator` as a PreSync Job. Vault Agent uses pre-populate-only mode and renders:

```text
AUTH_DB_DSN_FILE
AUDIT_DB_DSN_FILE
INVENTORY_DB_DSN_FILE
SALES_DB_DSN_FILE
CHAT_DB_DSN_FILE
```

The Job must finish `Complete 1/1` before Deployment rollout proceeds.

## Verification

```bash
kubectl -n test-app get job tropical-db-migrator
kubectl -n test-app get pods -l app=tropical-db-migrator
```

Logs:

```bash
MIG_POD=$(kubectl -n test-app get pods \
  -l app=tropical-db-migrator \
  --sort-by=.metadata.creationTimestamp \
  -o jsonpath='{.items[-1:].metadata.name}')

kubectl -n test-app logs "$MIG_POD" -c tropical-db-migrator --timestamps
```

Expected terminal event:

```text
event=migration_run_completed targets=5
```

Database check:

```sql
SELECT version, name, checksum, dirty, started_at, applied_at
FROM schema_migrations
ORDER BY version;
```

Every applied row must have `dirty = 0`.

## Failure recovery

Do not immediately retry repeatedly and do not manually delete `schema_migrations` rows.

Classify the failure first:

- Vault/DSN injection failure;
- DB connectivity/privilege failure;
- target points to wrong schema;
- checksum immutable-history violation;
- dirty previous migration;
- SQL/schema verification failure;
- lock timeout/concurrent operation.

Repair configuration or add a forward repair migration. Manual metadata repair is an exceptional operation and requires understanding exactly what SQL did or did not commit.

## P0.2 incident lesson

During cutover, `CHAT_DB_DSN` pointed to `tropical_auth`. Auth version `000001` was already recorded there, so chat version `000001` produced a checksum mismatch. The migrator correctly stopped the release. The fix was to create/use `tropical_chat` and correct Vault, not to disable checksum protection.

## Transitional cleanup

Legacy per-service `migrate()` functions/tests were intentionally retained for one rollout after startup calls were removed. After the validated rollout window, remove dead legacy migration code in a focused cleanup PR rather than allowing two competing migration implementations to persist indefinitely.
