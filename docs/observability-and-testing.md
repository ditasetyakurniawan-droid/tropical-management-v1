# Observability, testing, and Sonar coverage

## Backend logs

Every Go service configures the process logger through `internal/logx`. By default each service writes to both stdout and a rotating file:

- directory: `/var/log/tropical`
- active file: `<service-name>.log`
- default maximum active size: 25 MiB
- retained rotations: 5 (`.1` through `.5`)
- file mode: `0640`

Runtime configuration:

| Variable | Default | Purpose |
|---|---:|---|
| `LOG_DIR` | `/var/log/tropical` | Log directory |
| `LOG_MAX_SIZE_MB` | `25` | Rotate after this active-file size |
| `LOG_MAX_BACKUPS` | `5` | Number of rotations retained |
| `LOG_STDOUT` | `true` | Mirror file logs to stdout |

Docker Compose mounts a named volume at `/var/log/tropical` for all backend services. Service file names are unique. Example:

```bash
docker compose logs -f api-gateway
mkdir -p logs
docker cp "$(docker compose ps -q api-gateway):/var/log/tropical/api-gateway.log" ./logs/api-gateway.log
```

HTTP access records include `request_id`, service, method, path, status, response bytes, and duration. `X-Request-ID` is generated when absent and propagated gateway -> downstream service, enabling cross-service tracing with a single correlation identifier. Panics and internal/upstream failures are written server-side without leaking stack traces or database errors to clients.

## Local verification

Run the repository verification flow from the project root:

```bash
./scripts/verify-local.sh
```

Equivalent focused commands:

```bash
make fmt-check
make coverage
make vet
make web-build
```

The Go test workflow emits four artifacts:

- `coverage.out`: untouched native Go coverage profile for local tooling;
- `coverage.sonar.out`: the same coverage with repository-relative source paths for deterministic Sonar resolution;
- `go-test.json`: `go test -json` execution report for Sonar test execution metrics;
- `coverage-summary.txt`: human-readable per-function summary.

Do not edit these reports manually. `scripts/normalize-go-coverage.sh` performs deterministic path normalization and validates every source path before the Sonar scanner runs.

## SonarQube / SonarCloud ingestion

`sonar-project.properties` is the canonical analysis configuration. Jenkins only supplies runtime connection/authentication settings. Relevant properties:

```properties
sonar.go.coverage.reportPaths=coverage.sonar.out
sonar.go.tests.reportPaths=go-test.json
```

The CI backend stage runs tests with the race detector and atomic coverage mode, verifies both generated reports are non-empty, validates the normalized coverage paths, produces a coverage summary, and enforces a minimum 65% overall Go statement coverage. The frontend stage generates LCOV with Node's built-in test runner and enforces 80% line/function and 70% branch coverage for tested library code. The Sonar stage waits for the Quality Gate and therefore blocks the main pipeline when the gate fails.

Production frontend source remains in Sonar analysis. `web/lib` now has dependency-free Node unit tests and LCOV ingestion; production UI files are **not** excluded merely to inflate the coverage percentage. See `docs/coverage-policy.md` for the exclusion policy and thresholds.

## Security-related runtime behavior

- malformed JSON, unknown JSON fields, trailing JSON values, and bodies larger than 1 MiB are rejected;
- internal database/runtime errors are logged server-side and returned as generic responses;
- JWT validation only accepts HS256 and requires an expiration claim;
- JWT secrets are rejected when shorter than 32 bytes;
- service HTTP servers use header/read/idle timeouts and a header-size limit;
- panic stack traces are logged but never returned to clients;
- API route matching requires a path-segment boundary, so `/api/sales-report` cannot accidentally match `/api/sales`;
- gateway proxy failures return a controlled JSON `502` and are correlated through `request_id`;
- user/password creation rules use a minimum password length of 12 characters.
