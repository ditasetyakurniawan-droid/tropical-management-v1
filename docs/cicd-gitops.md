# CI/CD and GitOps delivery model

## Responsibility split

| Component | Responsibility |
|---|---|
| GitHub app repo | source, tests, migration files, Jenkinsfile |
| Jenkins | test, vet, frontend build, Sonar gate, image build/push, GitOps image bump |
| SonarQube | code-quality/coverage gate |
| Harbor | immutable application and migrator images |
| GitOps repo | Kubernetes desired state |
| Argo CD | reconciliation and deployment orchestration |
| Vault | runtime/migrator secrets |
| Kubernetes | workload execution |
| MySQL | service-owned persistence + migration metadata |

## Current release sequence

```text
1. PR -> app main
2. Jenkins checkout/version resolution
3. backend tests + race detector
4. frontend build
5. Sonar Quality Gate
6. build application images + tropical-db-migrator
7. push immutable tag to Harbor
8. update image tags in GitOps main
9. Argo CD detects OutOfSync and auto-syncs
10. PreSync tropical-db-migrator runs first
11. migration success -> Deployment rollout
12. migration failure -> release stops before rollout
13. readiness gates traffic to new pods
```

The P0.2 Phase B rollout validated this chain without manual Argo CD sync or manual pod restart.

## Release gates

A release must not publish/deploy when any of these fail:

- formatting check;
- race-enabled backend tests;
- `go vet`;
- frontend production build;
- Sonar Quality Gate;
- image build/push;
- PreSync migration.

## Image policy

Use immutable application-version + commit-derived tags. Do not deploy `latest`. The same release tag should identify the related service images and the matching migrator image.

## Database migration rule

Never edit an already-applied migration file. Add a new version. Argo CD owns migration ordering by running the migrator as PreSync. Runtime service startup must not perform DDL in the Kubernetes release path.

For backward-incompatible changes use expand/contract:

1. add compatible schema first;
2. deploy code that can work with both forms;
3. migrate/backfill data if necessary;
4. remove obsolete schema in a later release.

This keeps application rollback feasible across schema changes.

## GitOps sync policy

The GitOps repository is the desired state. Normal releases should not require manual `kubectl rollout restart` or manual Argo CD Sync. Manual operations are reserved for diagnosis/recovery and should be followed by reconciliation back to Git.

## Rollback

Application rollback is image/desired-state based. Before rolling code backward, confirm the database schema remains backward compatible. A successfully applied forward migration is not automatically reverted by rolling back application images.

Do not mutate `schema_migrations` manually to force a rollback. Use a deliberate forward repair migration if schema repair is needed.

## Operational evidence to capture

For meaningful releases retain or link:

- Jenkins build URL/number;
- application commit SHA;
- GitOps revision;
- Harbor image tag/digest;
- Argo CD operation result;
- db-migrator Job result/log;
- post-rollout health/restart status.
