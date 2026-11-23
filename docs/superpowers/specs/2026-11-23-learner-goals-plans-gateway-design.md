# OVM-105 / OVM-106: learner goals, diagnostics and study plans in GraphQL

## Intent

Expose the versioned learner onboarding and study-plan capabilities already owned by
`recommendations` through the public `api-gateway` GraphQL API. The frontend must be
able to onboard the authenticated learner, run a diagnostic, read an explainable
versioned plan, and submit explicit feedback without knowing an internal service URL.

The gateway remains a typed BFF. It authenticates the browser, forwards only the actor
from the verified JWT, validates the public GraphQL shape, applies a bounded deadline,
and maps upstream error codes. It does not reproduce recommendation or planning logic.

## Goals

- Add owner-only typed GraphQL queries and mutations for learner profile, goals,
  diagnostic sessions, current study plan, plan explanation, feedback, replanning,
  and explicit step progress.
- Preserve optimistic versions and idempotency keys end to end.
- Expose plan revision, next step, progress, reason codes, catalog freshness and the
  fallback returned by `recommendations`.
- Preserve machine-readable safe upstream errors such as `VERSION_CONFLICT`,
  `IDEMPOTENCY_CONFLICT`, `STEP_BLOCKED`, `RESOURCE_UNAVAILABLE`, `PLAN_LIMIT`, and
  `SERVICE_UNAVAILABLE` while keeping technical messages private.
- Keep all existing practice/learning-plan operations compatible.

## Non-goals

- No frontend screens; those remain OVM-152, OVM-153 and OVM-154.
- No changes to recommendation algorithms, persistence, migrations, or internal REST
  behavior.
- No admin or cross-user learner access.
- No aggregation of catalog titles into plan resources in this change; IDs and
  revisions remain canonical and frontend follow-up may resolve display metadata.
- No gateway-generated plan or diagnostic fallback when recommendations is down.
- No automatic conversion of a resource click into completed activity or learning
  evidence.

## Constraints

- `recommendations` is the implementation and contract owner. Its current internal
  HTTP routes and JSON fields are the source of truth.
- Every GraphQL operation derives `actor.UserID` from `middleware.RequireAuth(ctx)`;
  no learner/user ID appears in GraphQL arguments or inputs.
- Gateway calls have an operation-scoped timeout no longer than the configured
  recommendations client timeout; resolver code uses a bounded child context.
- Generated gqlgen files are mechanical exceptions to the normal 5–7 file commit
  guideline and are produced with `make generate`.

## Approaches considered

### 1. Typed one-to-one BFF contract — selected

Define stable GraphQL types and operations corresponding to the existing profile,
goal, diagnostic and study-plan resources. Keep REST-specific headers and paths in the
recommendations client. This gives frontend codegen a strict contract, keeps actor and
error handling centralized, and leaves business rules in recommendations.

Trade-off: the public schema is larger and mapping code is explicit, but evolution and
privacy boundaries are reviewable.

### 2. Coarse workflow facade

Combine onboarding, diagnostic and plan generation into a few gateway workflows.
This shortens frontend call sequences but moves orchestration, retry semantics and
partial-state decisions into the BFF. It was rejected because recommendations already
owns those state transitions and version checks.

### 3. Opaque JSON passthrough

Expose arbitrary JSON payloads from recommendations. This is quick but removes GraphQL
typing, generated frontend models and field-level review. It was rejected.

## Public GraphQL contract

Names below describe the intended public surface; exact gqlgen Go names follow normal
generation conventions.

### Queries

- `learnerGoalTemplates: [LearnerGoalTemplate!]!`
- `learnerProfile: LearnerProfile!`
- `learnerGoals: [LearnerGoal!]!`
- `learnerGoal(id: ID!): LearnerGoal!`
- `learnerDiagnostic(goalId: ID!): LearnerDiagnosticSession!`
- `currentLearningPlan(goalId: ID!): LearnerStudyPlan!`
- `learningPlanExplanation(goalId: ID!, planVersion: Int!): LearnerStudyPlanExplanation!`

All require authentication. A missing profile is represented by
`LearnerProfile.availability = null`; a diagnostic not yet started remains an upstream
`DIAGNOSTIC_NOT_STARTED` error rather than a fabricated null session.

### Mutations

- `saveLearnerProfile(input, expectedVersion, requestId)`
- `createLearnerGoal(input, requestId)`
- `updateLearnerGoal(id, input, expectedVersion, requestId)`
- `deleteLearnerGoal(id, expectedVersion, requestId)`
- `startLearnerDiagnostic(goalId, goalVersion, budgetMinutes)`
- `skipLearnerDiagnostic(goalId, goalVersion)`
- `finalizeLearnerDiagnostic(goalId, goalVersion, sessionId, allowPartial)`
- `generateLearningPlan(goalId, goalVersion, availabilityVersion, planVersion, requestId)`
- `submitLearningPlanFeedback(goalId, input, requestId)`
- `startLearningPlanStep(goalId, planVersion, ordinal)`
- `completeLearningPlanStepActivity(goalId, planVersion, ordinal)`
- `recordLearningPlanStepAssessment(goalId, planVersion, ordinal)`

UUID request IDs become `Idempotency-Key`. Expected profile/goal versions become
`X-Expected-Version`. Plan generation and feedback keep their versions in the body as
required by recommendations. The gateway never derives or silently increments a
version.

### Types and semantics

- Availability includes weekly minutes, timezone, version and updated timestamp.
- Goals include input fields, server ID/version/timestamps, feasibility and skills.
- Diagnostic sessions include status, baseline, selected task/version/topic IDs,
  stop rule, expiry and optional final report. The report preserves traceability fields
  from recommendations, including `submissionId`, `eventId` and
  `correctionVersion`. `no_tasks` and `skipped` are successful typed states, not
  errors.
- Study plans include plan/goal/availability versions, current/stale flags, freshness,
  pause and blocked reason, nullable next step, weeks, steps, progress timestamps,
  assessment, unmet outcomes, guidance and suggestions.
- Plan explanation includes per-step reasons/evidence/limits and
  `evidenceWatermark` returned by recommendations. No explanatory text is synthesized
  by gateway.
- Feedback actions are a GraphQL enum matching the upstream allowlist. `ordinal` and
  `weeklyMinutes` are nullable in GraphQL and their action-dependent validation remains
  authoritative in recommendations; gateway may reject structurally impossible values.
- The existing upstream response is the only safe fallback. Gateway preserves the
  upstream fields exactly rather than deriving `stale` from `freshness`: in particular,
  the valid catalog-outage fallback may have `stale = false`,
  `freshness.status = unavailable`, `blockedReason = catalog_unavailable` and no next
  step. On total service outage, gateway returns `SERVICE_UNAVAILABLE`; it does not
  reuse an unversioned cache.

## Internal client design

Add focused DTO and client files under `internal/client/recommendations`:

- onboarding/profile and goals;
- diagnostic session;
- study plan, explanation, feedback and progress.

Extend the client interface through capability interfaces so existing lightweight
GraphQL test fakes embedding `recommendations.Service` continue to compile. The real
`Client` implements all capabilities.

Add a small `httpx` request option/header mechanism, or an equivalent private helper in
the recommendations client, so per-request `Idempotency-Key` and
`X-Expected-Version` headers are set without duplicating HTTP transport behavior. The
helper must retain service auth, request ID, actor headers, redirect rejection,
response limits and safe error decoding.

Recommendations error decoding must preserve the JSON `code` supplied by upstream,
not collapse every HTTP 409 into a generic `CONFLICT`. Unknown/non-JSON/transport
failures map to `SERVICE_UNAVAILABLE`.

## Resolver and mapping design

Put the new manual resolvers in focused learner files rather than expanding existing
practice-goal resolvers. Each resolver:

1. calls `middleware.RequireAuth`;
2. checks the required recommendations capability;
3. creates a bounded child context;
4. passes only `auth.UserID` and trusted roles if needed;
5. calls the typed client method;
6. maps DTOs without adding business decisions.

Mapping functions normalize non-null lists to empty lists. Optional timestamps and
next-step values remain nullable. Date/time values use the existing schema convention
of RFC3339/date strings rather than adding a new scalar in this change.

## Error handling and privacy

- No GraphQL argument accepts `userId`, and tests assert that a browser-supplied user
  ID cannot reach recommendations.
- `ErrorPresenter` is extended to recognize `recommendations.Error`, return its safe
  code in `extensions.code`, and keep the public message generic.
- Upstream response bodies and learner payloads are not logged. Existing structured
  request logs retain method, path, status, request ID and duration only.
- Repeated writes with the same key return the upstream replayed result; the gateway
  does not deduplicate independently.

## Testing

Use TDD for each slice.

- Client tests with `httptest.Server` verify method/path/body, service token, trusted
  actor, request ID, idempotency and expected-version headers, exact replay, error-code
  preservation, and bounded cancellation.
- GraphQL resolver/operation tests verify unauthenticated rejection, owner actor
  forwarding, no cross-user input, mappings for profile/goals/diagnostic/plan,
  no-candidates, stale revision, safe fallback, service outage, and repeated diagnostic
  or idempotent writes.
- Generation checks: `make generate`, then a clean generated diff.
- Repository checks: `make test`, `make lint`, `make build`, and focused integration
  tests that do not require changing recommendations.

## Delivery and compatibility

Implement OVM-105 first, then OVM-106 on top because Jira records GW-07 as a blocker
of GW-08. Keep commits independently reviewable:

1. shared per-request HTTP header/error plumbing;
2. OVM-105 typed client, schema, generated code, resolvers and tests;
3. OVM-106 typed client, schema, generated code, resolvers and tests;
4. contract documentation and final verification adjustments if needed.

The schema additions are additive. Existing practice goal and saved learning-plan
operations remain unchanged, so frontend can migrate in later Jira tasks.

## Builder discretion

- Exact split of DTO/mapping files within the package.
- Whether per-request headers are implemented as generic `httpx` options or a private
  recommendations request helper, provided transport guarantees are not duplicated or
  weakened.
- Exact GraphQL type names for nested evidence/ranking details, provided the public
  fields and semantics above remain typed and complete, including the upstream
  traceability identifiers.
