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

### Workspace

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| GET | `/api/sdk/space` | 🧪 | ✅ |
| GET | `/api/sdk/metrics` | 🧪 | ✅ |
| POST | `/api/sdk/workspace/seed` | 🧪 | ✅ |

### Storage (Files)

`/api/sdk/files/*` — workspace-scoped raw blob storage. Bytes-in /
bytes-out, sha256-keyed dedup short-circuit on upload. Distinct from
Documents (RAG-indexed views) and Indexes (RAG containers); Files is
the universal-bytes primitive everything else can reference.

Files live inside named **buckets** within a workspace — caller-defined
strings like `screenshots`, `runs/42/`, or `user-attachments`. Buckets
are S3-shaped: just a name (no per-bucket config), used as both a query
filter and an on-disk path segment under
`<upload>/<workspace>/files/<bucket>/<file_id>/`. Uploads default to
`bucket=default` when the form field is omitted.

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| POST | `/api/sdk/files` (multipart, optional `bucket` form field) | 🧪 | ✅ |
| GET | `/api/sdk/files` (optional `?bucket=` or `?bucket_prefix=`) | 🧪 | ✅ |
| GET | `/api/sdk/files/buckets` | 🧪 | ✅ |
| GET | `/api/sdk/files/:id` | 🧪 | ✅ |
| GET | `/api/sdk/files/:id/content` (raw bytes) | 🧪 | ✅ |
| DELETE | `/api/sdk/files/:id` | 🧪 | ✅ |

Upload returns the existing File row (HTTP 200) when the same
`(workspace, bucket, content_sha256)` is already present; the same
bytes posted to a different bucket are intentionally a fresh row.
Otherwise creates a new row (HTTP 201). `?hard=true` on DELETE
force-removes a file the RESTRICT FK from `documents.file_id` would
otherwise block.

### Indexes (RAG containers)

`/api/sdk/indexes/:id` is a workspace-scoped container of RAG-indexed
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

The per-store `/api/sdk/indexes/:id/documents/:docId` routes (GET, DELETE) are alias forms of the workspace-level routes. SDK consumers should prefer the top-level form; the per-store form remains for admin tooling and future tier-1 consumers that already know the store.

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

### Collections (workspace-scoped JSON document store)

Mongo-style document buckets the agent uses for typed working memory
(lists of leads, scraped rows, normalized records). Distinct from
`indexes` (vector RAG) and from `data` (per-run scratch). Filter
operators: `$gt`, `$gte`, `$lt`, `$lte`, `$ne`, `$in`. Callbacks
(`.onInsert` / `.onUpdate` / `.onRemove` / `.onQuery`) are JS-only
and have no SDK equivalent — they're session-scoped goja hooks that
fire inside the same agent run that registered them.

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| GET | `/api/sdk/collections` | 🧪 | ✅ |
| POST | `/api/sdk/collections` | 🧪 | ✅ |
| DELETE | `/api/sdk/collections/:name` | 🧪 | ✅ |
| POST | `/api/sdk/collections/:name/documents` | 🧪 | ✅ |
| POST | `/api/sdk/collections/:name/find` | 🧪 | ✅ |
| POST | `/api/sdk/collections/:name/update` | 🧪 | ✅ |
| POST | `/api/sdk/collections/:name/remove` | 🧪 | ✅ |

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
| POST | `/api/sdk/agent-configs/:id/deployments` | 🧪 | ✅ |
| GET | `/api/sdk/agent-configs/:id/deployments` | 🧪 | ✅ |

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
| DELETE | `/api/sdk/scheduled-runs/:id` | 🧪 | ✅ |

### Evals

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| GET | `/api/sdk/evals` | 🧪 | ✅ |
| POST | `/api/sdk/evals` | 🧪 | ✅ |
| GET | `/api/sdk/evals/:id` | 🧪 | ✅ |
| DELETE | `/api/sdk/evals/:id` | 🧪 | ✅ |
| POST | `/api/sdk/evals/run` | 🧪 | ✅ |
| GET | `/api/sdk/eval-runs` | 🧪 | ✅ |
| GET | `/api/sdk/eval-runs/:id` | 🧪 | ✅ |
| GET | `/api/sdk/eval-suites` | 🧪 | ✅ |
| POST | `/api/sdk/eval-suites` | 🧪 | ✅ |
| GET | `/api/sdk/eval-suites/:id` | 🧪 | ✅ |
| PATCH | `/api/sdk/eval-suites/:id` | 🧪 | ✅ |
| DELETE | `/api/sdk/eval-suites/:id` | 🧪 | ✅ |
| POST | `/api/sdk/eval-suites/:id/versions` | 🧪 | ✅ |
| GET | `/api/sdk/eval-suites/:id/versions` | 🧪 | ✅ |

### Promotions (Phase 12)

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| GET | `/api/sdk/promotions/pending` | 🧪 | ✅ |
| POST | `/api/sdk/promotions` | 🧪 | ✅ |
| GET | `/api/sdk/promotions/:id` | 🧪 | ✅ |
| POST | `/api/sdk/promotions/:id/approve` | 🧪 | ✅ |
| POST | `/api/sdk/promotions/:id/reject` | 🧪 | ✅ |

### Tool policies + approvals (Phase 14)

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| GET | `/api/sdk/tool-policies` | 🧪 | ✅ |
| PUT | `/api/sdk/tool-policies` | 🧪 | ✅ |
| DELETE | `/api/sdk/tool-policies/:id` | 🧪 | ✅ |
| GET | `/api/sdk/approval-requests/pending` | 🧪 | ✅ |
| POST | `/api/sdk/approval-requests/:id/approve` | 🧪 | ✅ |
| POST | `/api/sdk/approval-requests/:id/reject` | 🧪 | ✅ |

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
gap (agent configs, evals, policies/approvals, scheduled runs, prompt
templates, studio, audit) closed in this pass.

**Test coverage: Vitest landed for documents + errors (2026-05-05).**
The TS SDK now has Vitest configured (`vitest.config.ts`,
`test/test-server.ts` mock harness) with parity coverage for the
documents endpoints (upload with provenance, list with all filters,
get/get-by-name/list-versions, search both modes, delete soft+hard,
structured-error round-trip). The errors module has its own tests for
`asVersionConflict`, `isNotFound`, `isUnauthorized`. Run with
`pnpm test`.

Remaining gaps (other endpoint families — agent configs, evals,
policies, audit) still lean on the Go SDK as the only tested path;
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
