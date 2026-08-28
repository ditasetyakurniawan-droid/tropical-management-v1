# Coverage policy

This repository uses coverage as a quality signal, not as a score to game.

## Baseline and gates

- Go overall statement coverage: **minimum 65%** in local verification and Jenkins.
- Current hardening target: **65-75% overall Go coverage**.
- New/changed code in SonarQube: use the built-in **Sonar way** quality gate, which requires at least **80% coverage on new code** and no more than **3% duplication on new code**.
- Frontend library code (`web/lib`): Node's built-in coverage gate requires at least **80% line coverage**, **80% function coverage**, and **70% branch coverage**. LCOV is generated for Sonar ingestion.

The 65% Go gate is intentionally below the current measured result so the pipeline is stable while still preventing a regression back toward the previous low-coverage state. Raise it incrementally after several green builds.

## What may be excluded

Only code that is not production behavior may be excluded from coverage:

- `*_test.go` files;
- generated source (`*_generated.go`, `generated/**`);
- test-only frontend files (`web/test/**`).

Production handlers, repositories, validation, security code, service `main.go` files, and frontend production source stay in coverage scope. A file is **never excluded merely because it currently reports 0%**.

## Test strategy

Database-backed service tests use a deterministic `database/sql/driver` test double defined only in `_test.go` files. This exercises real handler and transaction code without requiring a live MySQL instance during unit tests and without adding a production dependency.

Runtime integration remains separate: Docker Compose validates real MySQL startup, migrations, service-to-service traffic, login/JWT flows, dashboard aggregation, and file logging.

## Sonar configuration

`sonar-project.properties` ingests:

- `coverage.sonar.out` for Go coverage;
- `go-test.json` for Go test execution;
- `web/coverage/lcov.info` for JavaScript coverage.

Use Sonar's **Sonar way** quality gate for Clean as You Code. For feature-branch analysis, define new code relative to `main` (Reference branch) or use pull-request analysis where supported by the installed SonarQube edition.

## Raising the gate

Recommended progression after PR #18 is stable:

1. 65% overall Go coverage immediately.
2. 70% after several consecutive green builds.
3. 75% when remaining auth/database error paths have meaningful tests.
4. Keep Sonar coverage on new code at 80% or higher throughout.

Do not raise thresholds by excluding production files. Raise them by adding meaningful tests or by refactoring tightly-coupled code into testable units.
