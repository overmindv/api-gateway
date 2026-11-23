# Learner Goals and Study Plans Gateway Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose the recommendations learner profile, goals, diagnostic, study-plan, progress and feedback contracts through authenticated GraphQL operations for OVM-105 and OVM-106.

**Architecture:** Add typed REST DTOs and capability interfaces to the recommendations client, preserving upstream codes and per-request concurrency headers. Add an additive GraphQL schema and thin resolvers that take the actor only from JWT context and map upstream responses without inventing fallback state.

**Tech Stack:** Go 1.25, net/http, gqlgen, GraphQL, standard `testing`/`httptest`.

**Spec:** `docs/superpowers/specs/2026-11-23-learner-goals-plans-gateway-design.md`

## Global Constraints

- No public `userId` argument; every downstream `X-User-ID` comes from authenticated middleware context.
- Preserve upstream versions, reason codes, freshness and traceability fields exactly.
- Writes forward `Idempotency-Key` and, where required, `X-Expected-Version`; gateway never derives versions.
- Existing practice-goal, generated-plan and saved-plan GraphQL operations remain compatible.
- No changes to `task-hunter`.

## Review Focus

- A caller trying to pass another learner identity has no schema field or client parameter capable of overriding the JWT actor; pin in GraphQL operation tests.
- Reusing an idempotency key with changed input preserves `IDEMPOTENCY_CONFLICT`; pin in recommendations client/error presenter tests.
- A stale plan revision preserves `VERSION_CONFLICT`; pin in study-plan operation tests.
- Catalog outage fallback preserves the exact combination `stale=false`, `freshness.status=unavailable`, `blockedReason=catalog_unavailable`, `nextStep=null`; pin in mapping tests.
- A diagnostic report and plan explanation preserve evidence identifiers/watermarks; pin in mapping tests.

---

### Task 1: Per-request headers and upstream error codes

**Files:**
- Modify: `internal/client/httpx/httpx.go`
- Modify: `internal/client/httpx/httpx_test.go`
- Modify: `internal/client/recommendations/client.go`
- Modify: `internal/client/recommendations/client_test.go`

**Interfaces:**
- Produces: `httpx.DoJSONWithHeaders[T](..., actor Actor, headers ...Header) (T, error)` used by learner writes.
- Produces: `recommendations.Error{Code string}` decoded from upstream JSON while retaining a safe status-derived fallback.

- [ ] **Step 1: Write failing transport tests** proving custom headers are sent, actor headers cannot be supplied from a body, and upstream JSON `code` is decoded.
- [ ] **Step 2: Run `go test ./internal/client/httpx ./internal/client/recommendations`** and verify failures are caused by the missing header API/error decoding.
- [ ] **Step 3: Implement `DoJSONWithHeaders`** and make `DoJSON` delegate to it; apply request headers after fixed service headers but reject/overwrite actor and authorization header attempts.
- [ ] **Step 4: Make `recommendations.Error.Code` JSON-decodable** while retaining status fallback when a safe upstream code is absent.
- [ ] **Step 5: Run `go test ./internal/client/httpx ./internal/client/recommendations`** and verify PASS.
- [ ] **Step 6: Commit** as `feat(recommendations): preserve learner request metadata`.

### Task 2: OVM-105 learner profile, goals and diagnostic

**Files:**
- Create: `internal/client/recommendations/learner.go`
- Create: `internal/client/recommendations/diagnostic.go`
- Modify: `internal/client/recommendations/client_test.go`
- Modify: `api/graphql/schema.graphqls`
- Create: `internal/graphql/learner.go`
- Create: `internal/graphql/learner_test.go`
- Modify generated: `internal/graphql/generated/generated.go`
- Modify generated: `internal/graphql/model/models_gen.go`

**Interfaces:**
- Produces: `recommendations.LearnerService` for profile/goal/template operations.
- Produces: `recommendations.DiagnosticService` for get/start/skip/finalize.
- Consumes: `DoJSONWithHeaders` from Task 1.
- Produces: additive GraphQL operations named in the approved spec.

- [ ] **Step 1: Write failing recommendations-client tests** for paths, methods, actor, request/expected-version headers, same-user reads and repeated diagnostic start.
- [ ] **Step 2: Run `go test ./internal/client/recommendations`** and verify failures identify missing learner/diagnostic methods.
- [ ] **Step 3: Implement typed DTOs, capability interfaces and client methods** matching `/v1/learner/goal-templates`, `/profile`, `/goals` and `/goals/{id}/diagnostic`.
- [ ] **Step 4: Run the client tests** and verify PASS.
- [ ] **Step 5: Write failing GraphQL resolver/operation tests** for JWT actor authority, version/idempotency propagation, `no_tasks`/`skipped`, cross-user not-found, and diagnostic report traceability.
- [ ] **Step 6: Add additive schema types/inputs/operations and resolvers**, then run `make generate`.
- [ ] **Step 7: Run `go test ./internal/graphql ./internal/client/recommendations`** and verify PASS.
- [ ] **Step 8: Commit** as `feat(graphql): expose learner goals and diagnostics`.

### Task 3: OVM-106 current plan, explanation, feedback and progress

**Files:**
- Create: `internal/client/recommendations/study_plan.go`
- Modify: `internal/client/recommendations/client_test.go`
- Modify: `api/graphql/schema.graphqls`
- Create: `internal/graphql/study_plan.go`
- Create: `internal/graphql/study_plan_test.go`
- Modify generated: `internal/graphql/generated/generated.go`
- Modify generated: `internal/graphql/model/models_gen.go`

**Interfaces:**
- Produces: `recommendations.StudyPlanService` with current/generate/explain/feedback/start/complete/assess methods.
- Consumes: per-request headers and actor authority from Tasks 1-2.
- Produces: typed `currentLearningPlan`, explanation, generation, feedback and progress GraphQL operations.

- [ ] **Step 1: Write failing recommendations-client tests** for current/generate/explain/feedback/progress paths, version bodies, idempotency headers and error-code preservation.
- [ ] **Step 2: Run `go test ./internal/client/recommendations`** and verify failures identify missing study-plan methods.
- [ ] **Step 3: Implement typed DTOs, enums, capability interface and client methods** for the study-plan REST contract.
- [ ] **Step 4: Run the client tests** and verify PASS.
- [ ] **Step 5: Write failing GraphQL resolver/operation tests** for reason codes, evidence watermark, exact outage fallback, stale revision, no-candidates, feedback/skip/replan and resource-click non-authority.
- [ ] **Step 6: Add additive schema types/inputs/operations and resolvers**, then run `make generate`.
- [ ] **Step 7: Run `go test ./internal/graphql ./internal/client/recommendations`** and verify PASS.
- [ ] **Step 8: Commit** as `feat(graphql): expose adaptive study plans`.

### Task 4: Documentation and release verification

**Files:**
- Modify: `README.md` or the existing GraphQL contract document selected by repository convention.
- Modify: this plan ledger only through the execution harness.

**Interfaces:**
- Consumes: completed OVM-105 and OVM-106 operations.
- Produces: documented auth/version/error semantics and verified repository state.

- [ ] **Step 1: Document the new GraphQL surface**, emphasizing JWT actor authority, idempotency, optimistic concurrency and exact upstream fallback semantics.
- [ ] **Step 2: Run `make generate` and verify `git diff --exit-code` after staging-aware generated-file inspection** so generation is deterministic.
- [ ] **Step 3: Run `make test`, `make lint`, and `make build`** and verify all PASS.
- [ ] **Step 4: Run `python3 ../docs/scripts/bootstrap_workspace.py --check`** and verify workspace instructions have no drift.
- [ ] **Step 5: Commit** as `docs(graphql): document learner planning contract` if documentation changed after Task 3.
- [ ] **Step 6: Request fresh whole-branch code review**, fix Critical/Important findings with RED→GREEN tests, and rerun the full verification suite.

