# SDK Contract

The Tavora platform exposes its SDK API under `/api/sdk/*`. Two public
SDKs consume it:

- [`tavora-sdk-go`](https://github.com/tavora-ai/tavora-sdk-go) — Go SDK,
  unit-tested with `httptest.Server` mocks (see `testhelper_test.go`).
- [`tavora-sdk-ts`](https://github.com/tavora-ai/tavora-sdk-ts) — TypeScript
  SDK. Unit-test infrastructure is **not yet set up** in the TS SDK; the
  Go SDK has full coverage. See "Drift summary" below.

**Convention** — every endpoint listed below must:

1. Have a method on `Client` in **both** SDKs (Go and TS).
2. Have at least one unit test in **both** SDK test suites, asserting on
   request shape (method + path + body) and response parsing.
3. Be rejected at PR review if it exists in one SDK but not the other.

When the server adds an endpoint, add a row here in the same PR. When an SDK
method lands, the test is part of the same change — not a follow-up.

**Enforcement (since 2026-05-05).** A route-walker test in
`tavora-go/internal/server/contract_walk_test.go` boots the live chi
router, walks every `/api/sdk/*` route, and diffs against the tables
below. Drift in either direction (table row with no router entry, or
router entry with no table row) fails CI. Documented exceptions live
in the test's `walkerExceptions` map with a one-line reason; silent
skips are forbidden.

**Package layout (since 2026-05-05).** Handlers serving `/api/sdk/*`
live in `tavora-go/internal/platform/sdk/`; admin-UI handlers live in
`tavora-go/internal/platform/admin/`. An architest rule enforces the
import direction (admin → sdk allowed; sdk → admin forbidden). The
legacy `internal/platform/handlers/` directory is being drained one
feature at a time — `documents.go` migrated as the worked example.

## Coverage today

The table groups endpoints by feature. `✅` = method implemented in that
SDK. `🧪` = method implemented and unit-tested in that SDK.
`—` = not yet present. "Feature area" maps to the `internal/*/`
package on the server side.

### App

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| GET | `/api/sdk/app` | 🧪 | ✅ |
| GET | `/api/sdk/metrics` | 🧪 | ✅ |
| POST | `/api/sdk/app/seed` | 🧪 | ✅ |
| PUT | `/api/sdk/app/llm-vault` | ⏳ | ⏳ |

The `/api/sdk/app/llm-vault` endpoint designates which of the app's
secret_vaults sources LLM provider credentials. When set, the runtime
LLM resolver reads keys by convention (`openai_api_key`,
`anthropic_api_key`, `gemini_api_key`, `openrouter_api_key`,
`edenai_api_key`, `ollama_base_url`) from that vault per call, falling
back to the server-wide env vars for any provider not in the vault.
Cached for 60 s per (app, provider). This is what flips the
*"BYO model keys"* claim from gated to live.

### Storage / Files — ❌ removed 2026-05-11

Removed by the positioning rewrite. Customer file storage moves to the
customer's backend (PocketBase Storage / Supabase Storage / their own
S3 / R2), exposed to the agent via MCP when the agent needs to read
or write bytes. The `/api/sdk/files/*` surface is gone in migration
`00061_drop_files_table.sql`; the `internal/platform/sdk/files.go`
handler and the `files` table were deleted in the same wave.

Document ingest for RAG was never coupled to the `files` table in
practice (the `documents.file_id` column added by migration 00049
was never wired into the pipeline), so removing Files leaves the
Documents surface entirely intact.

### Indexes (RAG containers)

`/api/sdk/indexes/:id` is an app-scoped container of RAG-indexed
documents — what other ecosystems call "vector stores." Pre-customer
this surface was named `stores`; renamed for naming-coherence
(Storage = files; Indexes = RAG; Collections = JSON), see
`central-store/docs/RESOURCE_MODEL.md`.

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| GET | `/api/sdk/indexes` | 🧪 | ✅ |
| POST | `/api/sdk/indexes` | 🧪 | ✅ |
| GET | `/api/sdk/indexes/:id` | 🧪 | ✅ |
| PATCH | `/api/sdk/indexes/:id` | 🧪 | ✅ |
| DELETE | `/api/sdk/indexes/:id` | 🧪 | ✅ |

### Documents

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| POST | `/api/sdk/indexes/:id/documents` (multipart) | 🧪 | ✅ |
| GET | `/api/sdk/indexes/:id/documents` | 🧪 | ✅ |
| GET | `/api/sdk/indexes/:id/documents/:docId` | 🧪 | ✅ |
| GET | `/api/sdk/indexes/:id/documents/by-name/:name` | 🧪 | ✅ |
| GET | `/api/sdk/indexes/:id/documents/by-name/:name/versions` | 🧪 | ✅ |
| DELETE | `/api/sdk/indexes/:id/documents/:docId` | 🧪 | ✅ |
| GET | `/api/sdk/documents` | 🧪 | ✅ |
| GET | `/api/sdk/documents/:id` | 🧪 | ✅ |
| DELETE | `/api/sdk/documents/:id` | 🧪 | ✅ |

The per-store `/api/sdk/indexes/:id/documents/:docId` routes (GET, DELETE) are alias forms of the app-level routes. SDK consumers should prefer the top-level form; the per-store form remains for admin tooling and future tier-1 consumers that already know the store.

Documents carry user-supplied provenance via the multipart `metadata`
field (free-form JSON, recommended keys: `source`, `task`, `type`,
`tags.*`). Re-uploading with the same `name` to the same store creates
a new `version`; older versions remain (`is_latest=false`) and are
fetchable via `?version=N` on the by-name endpoints. `if_version` on
upload is optimistic concurrency: 409 on mismatch.

`DELETE` is soft by default (sets `deleted_at`, drops `is_latest` so a
future upload with the same name starts cleanly) and idempotent — 204
whether the row existed or was already gone. `?hard=true` removes the
row + the on-disk file.

Non-markdown indexable file types (PDF, DOCX, XLSX, etc.) generate an
**extracted markdown sibling** on upload: a second documents row with
`content_type=text/markdown`, `parent_id` pointing at the original,
`metadata.derived_from="extraction"`, and the same `name` suffixed
`.md`. Chunks attach to the sibling so search hits cite the editable
form. The original is marked `status="stored"` (raw bytes preserved,
not chunked). List the pair via `?parent_id=<original_id>` or filter
to derived rows with `?derived_from=extraction`.

Non-indexable types (`.json`, source code, etc.) upload successfully
but skip both extraction and chunking; their `status` settles to
`"stored"` and they never spawn siblings.

Every uploaded document is hashed server-side; the hex sha256 is
exposed as `content_sha256` on the response. Find duplicates with
`?content_sha256=<hex>` or the sugar `?duplicate_of=<id>` (resolves
the source's hash and excludes the source itself).

`POST /api/sdk/search` (and the per-store variant) accepts an optional
`result_type`:
- `"chunk"` (default) — one row per chunk, current shape.
- `"document"` — one row per distinct document, server-deduped, with
  the best chunk inlined as `best_chunk.preview`. Use when the agent
  asks "what artifacts are about X" rather than "what passages are
  about X".

### Search

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| POST | `/api/sdk/search` | 🧪 | ✅ |
| POST | `/api/sdk/indexes/:id/search` | 🧪 | ✅ |

### Collections — ❌ removed 2026-05-11

Removed by the positioning rewrite. Customer persistent structured records
belong in the customer's backend (PocketBase / Supabase / their own
Postgres), exposed to the agent via MCP. The `/api/sdk/collections` surface
is gone in migration `00060_drop_collection_primitives.sql`; the
`internal/collections/` package and `sandbox/pack_collections*` were
deleted in the same wave. Agent working memory now lives in
`memory_stores` (Stage 2 of the composable-primitives plan).

### Secret vaults (envelope-encrypted, app-scoped)

App-scoped vaults of named secrets the agent can read via the sandbox
`secret(name)` primitive when its session is pinned to a vault
(Stage 3 of the composable-primitives plan). Storage is envelope
encryption: a per-row DEK (AES-256-GCM) wrapped with the platform
KEK (`TAVORA_SECRET_KEK` today; KMS adapter behind `secrets.Sealer`
later). The SDK API NEVER returns plaintext — set takes a value and
returns the redacted view (name, kek_id, timestamps); list returns
the same redacted view in bulk; there's no get-plaintext endpoint
by design. The only way to retrieve a value is from inside a
running agent session that pinned the vault. Endpoints return 503
when `TAVORA_SECRET_KEK` is unset.

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| POST | `/api/sdk/secret-vaults` | 🧪 | ✅ |
| GET | `/api/sdk/secret-vaults` | 🧪 | ✅ |
| GET | `/api/sdk/secret-vaults/:id` | 🧪 | ✅ |
| PATCH | `/api/sdk/secret-vaults/:id` | 🧪 | ✅ |
| DELETE | `/api/sdk/secret-vaults/:id` | 🧪 | ✅ |
| GET | `/api/sdk/secret-vaults/:id/secrets` | 🧪 | ✅ |
| PUT | `/api/sdk/secret-vaults/:id/secrets/:name` | 🧪 | ✅ |
| DELETE | `/api/sdk/secret-vaults/:id/secrets/:name` | 🧪 | ✅ |

### Tenants (the one-line facade)

Stage 5 of the composable-primitives plan. Customers pass an opaque
`tenant_ref` string on session create and the platform isolates state
(memory, secrets, audit, future rate limits) behind it. The platform
never models the customer's user/org schema — the ref is opaque,
UTF-8, 1–256 bytes. First touch lazy-creates a per-tenant memory store
and secret vault and records a `tenant_pins` row; later sessions with
the same ref resolve to the same state. Customers who'd rather pre-
provision (e.g. backfill from their own user table) use the explicit
endpoints below. Anything the facade auto-does, the primitive APIs can
also do — the facade is pure sugar.

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| POST | `/api/sdk/tenants` | 🧪 | ✅ |
| GET | `/api/sdk/tenants` | 🧪 | ✅ |
| GET | `/api/sdk/tenants/:ref` | 🧪 | ✅ |
| PATCH | `/api/sdk/tenants/:ref` | 🧪 | ✅ |
| DELETE | `/api/sdk/tenants/:ref` | 🧪 | ✅ |

### Memory stores (app-scoped persistent key-value buckets)

Named, app-scoped KV buckets the agent can pin via `memory_store_id` on
session create (Stage 2 of the composable-primitives plan). Distinct
from legacy per-session `agent_memory` (which dies with the session)
and from `collections` (JSON document buckets). Each entry is a
`(memory_store_id, key) → value` row; entries cascade-delete with their
store. Sessions that don't pin a `memory_store_id` keep using the
legacy per-session memory path — the new tables coexist with the old.

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| POST | `/api/sdk/memory-stores` | 🧪 | ✅ |
| GET | `/api/sdk/memory-stores` | 🧪 | ✅ |
| GET | `/api/sdk/memory-stores/:id` | 🧪 | ✅ |
| PATCH | `/api/sdk/memory-stores/:id` | 🧪 | ✅ |
| DELETE | `/api/sdk/memory-stores/:id` | 🧪 | ✅ |
| GET | `/api/sdk/memory-stores/:id/entries` | 🧪 | ✅ |
| PUT | `/api/sdk/memory-stores/:id/entries/:key` | 🧪 | ✅ |
| DELETE | `/api/sdk/memory-stores/:id/entries/:key` | 🧪 | ✅ |

### Chat + Conversations

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| POST | `/api/sdk/chat/completions` | 🧪 | ✅ |
| POST | `/api/sdk/conversations` | 🧪 | ✅ |
| GET | `/api/sdk/conversations` | 🧪 | ✅ |
| GET | `/api/sdk/conversations/:id` | 🧪 | ✅ |
| DELETE | `/api/sdk/conversations/:id` | 🧪 | ✅ |
| POST | `/api/sdk/conversations/:id/messages` | 🧪 | ✅ |

### Agent sessions (SSE run)

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| GET | `/api/sdk/agents/system-prompt` | 🧪 | ✅ |
| POST | `/api/sdk/agents` | 🧪 | ✅ |
| GET | `/api/sdk/agents` | 🧪 | ✅ |
| GET | `/api/sdk/agents/:id` | 🧪 | ✅ |
| DELETE | `/api/sdk/agents/:id` | 🧪 | ✅ |
| POST | `/api/sdk/agents/:id/run` (SSE) | 🧪 | ✅ |

The `CreateAgentSession` input on both SDKs accepts an optional
`agent_version_id` to pin the session to an immutable agent version
(persona + model + skills_json filtering all server-resolved).

### Agent configs (versioned agents, Phase 11)

Live config (persona, skills, stores, provider, model) lives on the
agent row directly since the PR3 agent-simplification ship. The
draft+publish flow replaces the old propose-and-approve promotion
state machine — see `docs/agent-simplification-plan.md` in tavora-go.
`agent_versions` rows are now append-only history snapshots, written
on each publish.

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| POST | `/api/sdk/agent-configs` | 🧪 | ✅ |
| GET | `/api/sdk/agent-configs` | 🧪 | ✅ |
| GET | `/api/sdk/agent-configs/:id` | 🧪 | ✅ |
| PATCH | `/api/sdk/agent-configs/:id` | 🧪 | ✅ |
| DELETE | `/api/sdk/agent-configs/:id` | 🧪 | ✅ |
| PUT | `/api/sdk/agent-configs/:id/active-version` | 🧪 | ✅ |
| POST | `/api/sdk/agent-configs/:id/versions` | 🧪 | ✅ |
| GET | `/api/sdk/agent-configs/:id/versions` | 🧪 | ✅ |
| GET | `/api/sdk/agent-configs/:id/versions/:vid` | 🧪 | ✅ |
| PATCH | `/api/sdk/agent-configs/:id/draft` | 🧪 | ✅ |
| DELETE | `/api/sdk/agent-configs/:id/draft` | 🧪 | ✅ |
| POST | `/api/sdk/agent-configs/:id/publish` | 🧪 | ✅ |
| POST | `/api/sdk/agent-configs/:id/revert` | 🧪 | ✅ |
| PATCH | `/api/sdk/agent-configs/:id/settings` | 🧪 | ✅ |
| POST | `/api/sdk/agent-configs/:id/eval-runs` | 🧪 | ✅ |
| GET | `/api/sdk/agent-configs/:id/eval-runs` | 🧪 | ✅ |

### MCP servers

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| GET | `/api/sdk/mcp-servers` | 🧪 | ✅ |
| POST | `/api/sdk/mcp-servers` | 🧪 | ✅ |
| GET | `/api/sdk/mcp-servers/:id` | 🧪 | ✅ |
| PATCH | `/api/sdk/mcp-servers/:id` | 🧪 | ✅ |
| DELETE | `/api/sdk/mcp-servers/:id` | 🧪 | ✅ |
| POST | `/api/sdk/mcp-servers/:id/test` | 🧪 | ✅ |

### Skills

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| GET | `/api/sdk/skills` | 🧪 | ✅ |
| POST | `/api/sdk/skills` | 🧪 | ✅ |
| GET | `/api/sdk/skills/:id` | 🧪 | ✅ |
| DELETE | `/api/sdk/skills/:id` | 🧪 | ✅ |
| GET | `/api/sdk/skills/authoring-guide` | 🧪 | ✅ |

`POST /api/sdk/skills/validate` exists server-side but is not exposed
in either SDK — it's used by the admin UI's skill editor only. Add an
SDK method if a CLI consumer needs offline validation.

### Scheduled runs

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| GET | `/api/sdk/scheduled-runs` | 🧪 | ✅ |
| POST | `/api/sdk/scheduled-runs` | 🧪 | ✅ |
| GET | `/api/sdk/scheduled-runs/:id` | 🧪 | ✅ |
| PATCH | `/api/sdk/scheduled-runs/:id` | 🧪 | ✅ |
| DELETE | `/api/sdk/scheduled-runs/:id` | 🧪 | ✅ |

### Evals

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| GET | `/api/sdk/evals` | 🧪 | ✅ |
| POST | `/api/sdk/evals` | 🧪 | ✅ |
| GET | `/api/sdk/evals/:id` | 🧪 | ✅ |
| PATCH | `/api/sdk/evals/:id` | 🧪 | ✅ |
| DELETE | `/api/sdk/evals/:id` | 🧪 | ✅ |
| POST | `/api/sdk/evals/run` | 🧪 | ✅ |
| GET | `/api/sdk/eval-runs` | 🧪 | ✅ |
| GET | `/api/sdk/eval-runs/:id` | 🧪 | ✅ |
| GET | `/api/sdk/eval-suites` | 🧪 | ✅ |
| POST | `/api/sdk/eval-suites` | 🧪 | ✅ |
| GET | `/api/sdk/eval-suites/:id` | 🧪 | ✅ |
| PATCH | `/api/sdk/eval-suites/:id` | 🧪 | ✅ |
| PATCH | `/api/sdk/eval-suites/:id/judge` | ❌ | ❌ |
| DELETE | `/api/sdk/eval-suites/:id` | 🧪 | ✅ |
| POST | `/api/sdk/eval-suites/:id/versions` | 🧪 | ✅ |
| GET | `/api/sdk/eval-suites/:id/versions` | 🧪 | ✅ |

### Promotions — ❌ removed 2026-05-12

Removed by the agent-simplification ship. The propose/approve/reject
state machine was retired in favor of the draft+publish flow on
`/api/sdk/agent-configs/:id/{draft,publish,revert}`. See those rows
under "Agent configs" above.

### Tool policies + approvals — ❌ removed 2026-05-13

Removed by the MVP slim-down (`docs/mvp-slimdown-plan.md` Step 2).
The `internal/policy/` Go package, the SDK methods, the
`PoliciesPage` + `ApprovalsPage` admin surfaces, and the
`tool_policies` + `approval_requests` tables are all deleted.
The sandbox runs tools without a policy gate. The Advisor
(`internal/advisor/`) and its rules engine went with this pass.

### Prompt templates

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| GET | `/api/sdk/prompt-templates` | 🧪 | ✅ |
| POST | `/api/sdk/prompt-templates` | 🧪 | ✅ |
| GET | `/api/sdk/prompt-templates/:id` | 🧪 | ✅ |
| PATCH | `/api/sdk/prompt-templates/:id` | 🧪 | ✅ |
| DELETE | `/api/sdk/prompt-templates/:id` | 🧪 | ✅ |

### Studio

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| GET | `/api/sdk/studio/:sessionID` | 🧪 | ✅ |
| POST | `/api/sdk/studio/:sessionID/replay` (SSE) | 🧪 | ✅ |
| POST | `/api/sdk/studio/:sessionID/analyze` | 🧪 | ✅ |

### Audit log (Phase 13)

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| GET | `/api/sdk/audit-log` | 🧪 | ✅ |
| GET | `/api/sdk/audit-log/export` | 🧪 | ✅ |

## Drift summary

**Method coverage: parity (2026-05-04).** Every endpoint exposed by the
Go SDK has a corresponding TS SDK method. The historical ~70% Go-only
gap (agent configs, evals, scheduled runs, prompt templates, studio,
audit) closed in this pass. Tool policies + approvals were removed
entirely on 2026-05-13 (see above).

**Test coverage: Vitest landed for documents + errors (2026-05-05).**
The TS SDK now has Vitest configured (`vitest.config.ts`,
`test/test-server.ts` mock harness) with parity coverage for the
documents endpoints (upload with provenance, list with all filters,
get/get-by-name/list-versions, search both modes, delete soft+hard,
structured-error round-trip). The errors module has its own tests for
`asVersionConflict`, `isNotFound`, `isUnauthorized`. Run with
`pnpm test`.

Remaining gaps (other endpoint families — agent configs, evals,
audit) still lean on the Go SDK as the only tested path;
adding TS tests for those is future work, but the Vitest infra is now
in place so each landing is one file rather than a project setup.

**Error type parity (2026-05-05).** Both SDKs now expose:

- `code` — server-supplied error code string (e.g. `"version_conflict"`,
  `"NOT_FOUND"`).
- `details` / `Details` — every other top-level field from the JSON
  error body (e.g. `current_version` on a 409). Lets agents recover
  programmatically without parsing human-readable strings.
- `AsVersionConflict(err) -> (*VersionConflictError, bool)` (Go) /
  `asVersionConflict(err): VersionConflict | null` (TS) — typed
  recovery helper that returns the structured `current_version`.

`message` (Go) / `apiMessage` (TS) hold the raw server message
unwrapped. The TS SDK's `Error.message` keeps the formatted
`tavora: ... (status N)` wrapper for log lines.

## Out of scope for this doc

- Request/response payload schemas — use `types.go` and `tavora-sdk-ts/src/types.ts`
  as the source of truth. They must stay aligned (TS field names mirror Go
  JSON tags — see the comment at the top of `tavora-sdk-ts/src/types.ts`).
- Error semantics — both SDKs surface `TavoraAPIError`-like types with an
  HTTP status and optional server-supplied `code`. Covered by
  `client_test.go` on the Go side; pending in TS until test infra lands.
- Live-server integration tests — see `tavora-sdk-go/examples/e2e/`. Those
  verify the server honors what the SDKs send; they complement (not
  replace) the unit-level contract.
