SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c
SERVICES := auth-service audit-service inventory-service sales-service chat-service workforce-service dashboard-service api-gateway

.PHONY: test coverage fmt fmt-check vet verify web-install web-test web-build compose-up compose-down

test:
	go test -race ./...

coverage:
	go test -json -race -covermode=atomic -coverprofile=coverage.out ./... | tee go-test.json
	./scripts/normalize-go-coverage.sh coverage.out coverage.sonar.out
	go tool cover -func=coverage.out | tee coverage-summary.txt

fmt:
	gofmt -w internal services

fmt-check:
	@test -z "$$(gofmt -l internal services)" || (echo "Go files need gofmt:"; gofmt -l internal services; exit 1)

vet:
	go vet ./...

web-install:
	cd web && npm ci

web-test: web-install
	cd web && npm run test:coverage

web-build: web-install
	cd web && npm run build

verify: fmt-check coverage vet web-test web-build

compose-up:
	docker compose up --build

compose-down:
	docker compose down -v
