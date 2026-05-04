# SDK Contract

The Tavora platform exposes its SDK API under `/api/sdk/*`. Two public
SDKs consume it:

- [`tavora-sdk-go`](https://github.com/tavora-ai/tavora-sdk-go) — Go SDK,
  unit-tested with `httptest.Server` mocks (see `testhelper_test.go`).
- [`tavora-sdk-ts`](https://github.com/tavora-ai/tavora-sdk-ts) — TypeScript
  SDK, unit-tested with an injected mock `fetch` (see `tests/helper.ts`).

**Convention** — every endpoint listed below must:

1. Have a method on `Client` in **both** SDKs (Go and TS).
2. Have at least one unit test in **both** SDK test suites, asserting on
   request shape (method + path + body) and response parsing.
3. Be rejected at PR review if it exists in one SDK but not the other.

When the server adds an endpoint, add a row here in the same PR. When an SDK
method lands, the test is part of the same change — not a follow-up.

## Coverage today

The table groups endpoints by feature. `✅` = implemented and unit-tested in
that SDK. `—` = not yet present. "Feature area" maps to the `internal/*/`
package on the server side.

### Workspace

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| GET | `/api/sdk/space` | ✅ | ✅ |
| GET | `/api/sdk/metrics` | ✅ | — |
| POST | `/api/sdk/workspace/seed` | ✅ | — |

### Stores

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| GET | `/api/sdk/stores` | ✅ | ✅ |
| POST | `/api/sdk/stores` | ✅ | ✅ |
| GET | `/api/sdk/stores/:id` | ✅ | ✅ |
| PATCH | `/api/sdk/stores/:id` | ✅ | ✅ |
| DELETE | `/api/sdk/stores/:id` | ✅ | ✅ |

### Documents

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| POST | `/api/sdk/stores/:id/documents` (multipart) | ✅ | ✅ |
| GET | `/api/sdk/stores/:id/documents` | ✅ | ✅ |
| GET | `/api/sdk/documents` | ✅ | ✅ |
| GET | `/api/sdk/documents/:id` | ✅ | ✅ |
| DELETE | `/api/sdk/documents/:id` | ✅ | ✅ |

### Search

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| POST | `/api/sdk/search` | ✅ | ✅ |
| POST | `/api/sdk/stores/:id/search` | ✅ | ✅ |

### Chat + Conversations

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| POST | `/api/sdk/chat/completions` | ✅ | ✅ |
| POST | `/api/sdk/conversations` | ✅ | ✅ |
| GET | `/api/sdk/conversations` | ✅ | ✅ |
| GET | `/api/sdk/conversations/:id` | ✅ | ✅ |
| DELETE | `/api/sdk/conversations/:id` | ✅ | ✅ |
| POST | `/api/sdk/conversations/:id/messages` | ✅ | — |

### Agent sessions (SSE run)

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| GET | `/api/sdk/agents/system-prompt` | ✅ | ✅ |
| POST | `/api/sdk/agents` | ✅ | ✅ |
| GET | `/api/sdk/agents` | ✅ | ✅ |
| GET | `/api/sdk/agents/:id` | ✅ | ✅ |
| DELETE | `/api/sdk/agents/:id` | ✅ | ✅ |
| POST | `/api/sdk/agents/:id/run` (SSE) | ✅ | — |

### Agent configs (versioned agents, Phase 11)

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| POST | `/api/sdk/agent-configs` | ✅ | — |
| GET | `/api/sdk/agent-configs` | ✅ | — |
| GET | `/api/sdk/agent-configs/:id` | ✅ | — |
| PATCH | `/api/sdk/agent-configs/:id` | ✅ | — |
| DELETE | `/api/sdk/agent-configs/:id` | ✅ | — |
| PUT | `/api/sdk/agent-configs/:id/active-version` | ✅ | — |
| POST | `/api/sdk/agent-configs/:id/versions` | ✅ | — |
| GET | `/api/sdk/agent-configs/:id/versions` | ✅ | — |
| GET | `/api/sdk/agent-configs/:id/versions/:vid` | ✅ | — |
| POST | `/api/sdk/agent-configs/:id/deployments` | ✅ | — |
| GET | `/api/sdk/agent-configs/:id/deployments` | ✅ | — |

### MCP servers

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| GET | `/api/sdk/mcp-servers` | ✅ | ✅ |
| POST | `/api/sdk/mcp-servers` | ✅ | ✅ |
| GET | `/api/sdk/mcp-servers/:id` | ✅ | ✅ |
| PATCH | `/api/sdk/mcp-servers/:id` | ✅ | ✅ |
| DELETE | `/api/sdk/mcp-servers/:id` | ✅ | ✅ |
| POST | `/api/sdk/mcp-servers/:id/test` | ✅ | ✅ |

### Skills

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| GET | `/api/sdk/skills` | ✅ | — |
| POST | `/api/sdk/skills` | ✅ | — |
| GET | `/api/sdk/skills/:id` | ✅ | — |
| DELETE | `/api/sdk/skills/:id` | ✅ | — |

### Scheduled runs

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| GET | `/api/sdk/scheduled-runs` | ✅ | — |
| POST | `/api/sdk/scheduled-runs` | ✅ | — |
| GET | `/api/sdk/scheduled-runs/:id` | ✅ | — |
| DELETE | `/api/sdk/scheduled-runs/:id` | ✅ | — |

### Evals

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| GET | `/api/sdk/evals` | ✅ | — |
| POST | `/api/sdk/evals` | ✅ | — |
| DELETE | `/api/sdk/evals/:id` | ✅ | — |
| POST | `/api/sdk/evals/run` | ✅ | — |
| GET | `/api/sdk/eval-runs` | ✅ | — |
| GET | `/api/sdk/eval-runs/:id` | ✅ | — |
| GET | `/api/sdk/eval-suites` | ✅ | — |
| POST | `/api/sdk/eval-suites` | ✅ | — |
| GET | `/api/sdk/eval-suites/:id` | ✅ | — |
| DELETE | `/api/sdk/eval-suites/:id` | ✅ | — |
| POST | `/api/sdk/eval-suites/:id/versions` | ✅ | — |

### Promotions (Phase 12)

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| GET | `/api/sdk/promotions/pending` | ✅ | — |
| POST | `/api/sdk/promotions` | ✅ | — |
| GET | `/api/sdk/promotions/:id` | ✅ | — |
| POST | `/api/sdk/promotions/:id/approve` | ✅ | — |
| POST | `/api/sdk/promotions/:id/reject` | ✅ | — |

### Tool policies + approvals (Phase 14)

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| GET | `/api/sdk/tool-policies` | ✅ | — |
| PUT | `/api/sdk/tool-policies` | ✅ | — |
| DELETE | `/api/sdk/tool-policies/:id` | ✅ | — |
| POST | `/api/sdk/approval-requests/:id/approve` | ✅ | — |
| POST | `/api/sdk/approval-requests/:id/reject` | ✅ | — |

### Prompt templates

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| GET | `/api/sdk/prompt-templates` | ✅ | — |
| POST | `/api/sdk/prompt-templates` | ✅ | — |
| GET | `/api/sdk/prompt-templates/:id` | ✅ | — |
| PATCH | `/api/sdk/prompt-templates/:id` | ✅ | — |
| DELETE | `/api/sdk/prompt-templates/:id` | ✅ | — |

### Studio

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| GET | `/api/sdk/studio/:sessionID` | ✅ | — |
| POST | `/api/sdk/studio/:sessionID/analyze` | ✅ | — |

### Audit log (Phase 13)

| Method | Path | Go SDK | TS SDK |
|---|---|---|---|
| GET | `/api/sdk/audit` | ✅ | — |
| GET | `/api/sdk/audit/export` | ✅ | — |

## Drift summary

At the time of writing, the TS SDK covers **~30%** of the endpoints the Go SDK
exposes. The gaps are concentrated in four areas: agent configs, evals,
policies/approvals, and scheduled runs. These are Enterprise-tier features
that have landed in Go first and have not yet been mirrored in TS.

Before shipping a TS-consuming feature that touches any of those areas, port
the relevant SDK methods and add matching unit tests, or the contract above
is not honored.

## Out of scope for this doc

- Request/response payload schemas — use `types.go` and `sdk-ts/src/types.ts`
  as the source of truth. They must stay aligned (TS field names mirror Go
  JSON tags — see the comment at the top of `sdk-ts/src/types.ts`).
- Error semantics — both SDKs surface `TavoraAPIError`-like types with an
  HTTP status and optional server-supplied `code`. Covered by
  `client_test.go` and `sdk-ts/tests/client.test.ts`.
- Live-server integration tests — see `tavora-sdk-go/examples/e2e/` and
  `tavora-sdk-go/examples/rag-eval-formats/` (all Go examples live in the
  public SDK repo as of 2026-05-04).
  Those verify the server honors what the SDKs send; they complement (not
  replace) the unit-level contract.
