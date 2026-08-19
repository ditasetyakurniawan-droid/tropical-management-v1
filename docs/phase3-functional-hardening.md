# Phase 3 — Functional Hardening

This phase turns the Phase-2 runnable MVP into a more usable operational product while keeping the microservice boundaries intact.

## Product changes

### Internal Audit
- Checklist scoring remains 0–100 for Cleanliness, SOP, and Food Quality.
- Findings now support severity, PIC, due date, corrective action, and workflow status.
- Supported workflow: `open -> in_progress -> resolved -> verified -> closed`.
- Findings are sorted with critical/high severity first.

### Inventory
- Supplier management is available from the UI.
- Stock adjustments require a non-zero delta and business reason.
- Every adjustment is recorded in `stock_movements`.
- Stock cannot be adjusted below zero.

### Users / RBAC
- Admin can create users, change role, activate, and deactivate accounts.
- Inactive accounts cannot log in.
- Supported roles remain Admin, Auditor, and Staff.

### Frontend / UX
- Protected routes validate the current JWT using `/api/auth/me`.
- Logout clears local session state.
- Dashboard adds lightweight sales, audit, finding, and inventory analytics.
- The visual system uses deep tropical green, restrained gold, glass surfaces, botanical depth, and animated leaves.
- Leaf motion respects `prefers-reduced-motion`.

## Local acceptance test

```bash
git checkout feature/phase3-functional-hardening-luxury-ui
docker compose down --remove-orphans
docker compose build --no-cache
docker compose up -d
docker compose ps
curl -fsS http://localhost:8080/healthz
```

Open `http://localhost:3000` in a browser.

### Suggested manual test sequence

1. Login using the local development admin.
2. Create Staff and Auditor users; change role and toggle activation.
3. Add a supplier and inventory item.
4. Post positive and negative stock movements with reasons.
5. Create an audit and a critical finding with PIC + due date.
6. Move the finding through `in_progress`, `resolved`, `verified`, and `closed`.
7. Add sales entries and confirm dashboard analytics react to the new data.
8. Confirm logout redirects to `/login` and protected pages cannot be opened without a token.

## Quality gate

```bash
go test ./...
go vet ./...
cd web
npm install
npm run build
```

## Intentionally deferred

These are subsequent phases, not hidden in this PR:
- file/evidence upload for audit findings
- full audit template/question designer
- purchase order / goods receipt workflow
- password reset UX and session revocation
- Vault `*_FILE` secret consumption
- Harbor image publication
- Kubernetes workload manifests and Argo CD automated sync
