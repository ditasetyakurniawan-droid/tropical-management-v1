SHELL := /bin/bash
SERVICES := auth-service audit-service inventory-service sales-service chat-service dashboard-service api-gateway

.PHONY: test fmt vet web-install web-build compose-up compose-down

test:
	go test ./...

fmt:
	gofmt -w internal services

vet:
	go vet ./...

web-install:
	cd web && npm install

web-build:
	cd web && npm run build

compose-up:
	docker compose up --build

compose-down:
	docker compose down -v
