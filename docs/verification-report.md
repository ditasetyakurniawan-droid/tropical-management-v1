# Verification report

Generated during the remediation review on 2026-08-29.

## Checks executed successfully in the review environment

The following checks completed successfully after the remediation patches:

```bash
gofmt -l internal services
bash -n scripts/*.sh build-tropical-management.sh push-tropical-management.sh
node # JSON parse/lock consistency checks for web/package.json and web/package-lock.json

go test -race -covermode=atomic -coverprofile=/tmp/coverage-subset.out \
  ./internal/httpx ./internal/logx ./services/dashboard-service

./scripts/normalize-go-coverage.sh \
  /tmp/coverage-subset.out /tmp/coverage-subset.sonar.out
```

Observed runnable-subset coverage:

```text
internal/httpx             72.3%
internal/logx              78.7%
services/dashboard-service 80.0%
combined                   76.2%
```

The Sonar coverage normalizer also validated that every normalized coverage path referenced an existing repository source file.

## Environment limitation during full-suite execution

The isolated review environment did not have outbound Go module registry/DNS access and did not already contain all required modules in its local Go module cache. Therefore a full `go test -race ./...` could not reach compilation for packages importing these dependencies:

```text
github.com/go-sql-driver/mysql v1.8.1
github.com/golang-jwt/jwt/v5 v5.3.1
golang.org/x/crypto v0.31.0
```

This was a dependency-fetch limitation, not a failing unit-test assertion. Frontend `npm ci` was subject to the same offline-registry limitation.

## Required pre-merge verification on a normal developer/CI machine

From the repository root, run:

```bash
./scripts/verify-local.sh
```

That command performs module verification, formatting validation, full Go tests with race detection and Sonar reports, `go vet`, a clean `npm ci`, and the Next.js production build.

For Docker-level smoke testing after the command above succeeds:

```bash
docker compose up --build
curl -fsS http://localhost:8080/healthz
curl -fsS http://localhost:3000 >/dev/null
```

Do not merge to `main` if `verify-local.sh` or the Jenkins Sonar Quality Gate fails.

## Beta2 targeted regression: audit due-date validation

A developer-local verification run exposed a compile failure in `audit-service` because
`validateIssue`, `updateIssue`, and their tests referenced `errInvalidDueDate` while the
constant was missing from the service's error-message constant block.

The remediation adds:

```go
errInvalidDueDate = "invalid due date; expected YYYY-MM-DD"
```

Targeted offline regression executed after the fix:

```text
internal/dbx             PASS (race), coverage 46.2%
services/audit-service   PASS (race), coverage 33.7%
```

For this isolated review environment only, the targeted audit/dbx compile used a temporary
minimal `github.com/go-sql-driver/mysql` compile stub outside the repository because the
sandbox still cannot resolve `proxy.golang.org`. The stub is not part of the repository or
runtime build. The production dependency remains `github.com/go-sql-driver/mysql v1.8.1`.

Formatting, shell syntax, Next.js package/lock consistency, the Sonar coverage normalizer,
and the previously runnable race-test subset were rerun successfully after this patch.
