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

### Stores

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| GET | `/api/sdk/stores` | 🧪 | ✅ |
| POST | `/api/sdk/stores` | 🧪 | ✅ |
| GET | `/api/sdk/stores/:id` | 🧪 | ✅ |
| PATCH | `/api/sdk/stores/:id` | 🧪 | ✅ |
| DELETE | `/api/sdk/stores/:id` | 🧪 | ✅ |

### Documents

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| POST | `/api/sdk/stores/:id/documents` (multipart) | 🧪 | ✅ |
| GET | `/api/sdk/stores/:id/documents` | 🧪 | ✅ |
| GET | `/api/sdk/documents` | 🧪 | ✅ |
| GET | `/api/sdk/documents/:id` | 🧪 | ✅ |
| DELETE | `/api/sdk/documents/:id` | 🧪 | ✅ |

### Search

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| POST | `/api/sdk/search` | 🧪 | ✅ |
| POST | `/api/sdk/stores/:id/search` | 🧪 | ✅ |

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
| DELETE | `/api/sdk/evals/:id` | 🧪 | ✅ |
| POST | `/api/sdk/evals/run` | 🧪 | ✅ |
| GET | `/api/sdk/eval-runs` | 🧪 | ✅ |
| GET | `/api/sdk/eval-runs/:id` | 🧪 | ✅ |
| GET | `/api/sdk/eval-suites` | 🧪 | ✅ |
| POST | `/api/sdk/eval-suites` | 🧪 | ✅ |
| GET | `/api/sdk/eval-suites/:id` | 🧪 | ✅ |
| DELETE | `/api/sdk/eval-suites/:id` | 🧪 | ✅ |
| POST | `/api/sdk/eval-suites/:id/versions` | 🧪 | ✅ |

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

**Test coverage: asymmetric.** The Go SDK has full unit tests using
`httptest.Server` mocks. The TS SDK does not yet have a unit-test
infrastructure set up — no `tests/` directory, no test runner config in
`package.json`. Per the contract above this is a known gap and the next
TS-side commit should land the test infra (Vitest is the natural pick:
ESM-native, no config) plus tests covering at minimum the methods most
likely to drift on shape changes (agent configs, evals, policies,
audit). Until then `🧪` only marks the Go side; the TS side stays at
`✅` (method present).

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
