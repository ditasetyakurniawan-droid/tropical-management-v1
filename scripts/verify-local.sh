#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

echo "==> Go module verification"
go mod download
go mod verify

echo "==> Go format check"
UNFORMATTED="$(gofmt -l internal services)"
if [[ -n "$UNFORMATTED" ]]; then
  echo "Go files are not gofmt-clean:" >&2
  printf '%s\n' "$UNFORMATTED" >&2
  exit 1
fi

echo "==> Go tests + race detector + Sonar reports"
go test -json -race -covermode=atomic -coverprofile=coverage.out ./... | tee go-test.json
./scripts/normalize-go-coverage.sh coverage.out coverage.sonar.out
test -s coverage.out
test -s coverage.sonar.out
test -s go-test.json
go tool cover -func=coverage.out | tee coverage-summary.txt

echo "==> Go vet"
go vet ./...

echo "==> Frontend clean install + production build"
(
  cd web
  npm ci
  npm run build
)

echo "==> Verification passed"
