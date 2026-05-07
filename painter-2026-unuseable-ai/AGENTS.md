# AGENTS Guide

## Purpose

This repository hosts the full Painter-2026 implementation. Agents must keep architecture, tests, and docs in sync.

## Mandatory Rules

1. Use contract-first development. Update `packages/openapi/openapi.yaml` before adding new API behavior.
2. Any API change must update:
   - service implementation
   - `packages/sdk-ts`
   - tests
   - docs
3. Keep service layering:
   - `transport/http`
   - `application`
   - `domain`
   - `infrastructure`
4. Do not put business logic inside `cmd/*/main.go`.
5. Every feature PR must include tests for changed code paths.
6. For legacy 1:1 migration work, always update `docs/migration/legacy-api-matrix.md` and replication diff docs.
7. Default production data stack is `MySQL + Redis`. Do not introduce SQLite as service persistence for business chains unless the user explicitly approves.
8. For scenarios that require stronger relational semantics, allow `PostgreSQL`. For asynchronous/decoupled workflows, design with `MQ` support.
9. Any persistence or infrastructure change must update service config, tests, and deploy docs together.
10. Delivery baseline is "end-to-end complete". Do not leave partial implementations (for example only route wired without real repository/service chain, or only schema without runnable tests/docs).
11. During execution, continuously read live code in changed areas and immediately report newly found partial/pseudo logic.
12. Record each finding in `docs/migration/realtime-findings-log.md`, then close them in a final convergence pass.

## Testing Bar

- Backend:
  - domain rule tests
  - application use case tests
  - transport handler tests
- Frontend:
  - store/composable/component tests
- Contract:
  - OpenAPI file validation
  - SDK generation check

## Documentation Bar

- Any new feature must be documented in VitePress under `docs/`.
- Deployment or runtime changes must update `docs/deploy/`.
- Architecture changes must update `docs/architecture/`.

## Style

- Keep code ASCII unless file already uses Unicode.
- Prefer explicit types in TypeScript and clear error handling in Go.
- Maintain consistent response envelope `{ code, message, traceId, data }`.
