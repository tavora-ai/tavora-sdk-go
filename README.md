# tavora-sdk-go

The official Go SDK for the [Tavora](https://tavora.ai) agentic intelligence platform.

Build AI agents that reason by writing code — sandboxed, versioned, and
auditable across tenants. See [docs.tavora.ai](https://docs.tavora.ai).

## Installation

```sh
go get github.com/tavora-ai/tavora-sdk-go
```

## Quickstart

```go
package main

import (
    "context"
    "fmt"
    "log"

    tavora "github.com/tavora-ai/tavora-sdk-go"
)

func main() {
    client := tavora.NewClient("https://api.tavora.ai", "tvr_your-api-key")
    ctx := context.Background()

    ws, err := client.GetApp(ctx)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Connected to app: %s\n", ws.Name)
}
```

Every SDK method is a direct call on `*Client` — no nested service
namespaces. Methods are context-aware, so callers can cancel, deadline,
or trace any request.

## What the SDK covers

| Area | Methods |
|---|---|
| **App** | `GetApp` |
| **Agents — sessions** | `CreateAgentSession`, `ListAgentSessions`, `GetAgentSession`, `DeleteAgentSession`, `RunAgent`, `GetAgentSystemPrompt` |
| **Agents — configs + history** | `CreateAgentConfig`, `ListAgentConfigs`, `Get/Update/DeleteAgentConfig`, `UpdateAgentDraft`, `DiscardAgentDraft`, `PublishAgent`, `RevertAgent`, `UpdateAgentSettings`, `RunAgentEval`, `ListAgentEvalRuns` |
| **Agent versions** | `CreateAgentVersion`, `ListAgentVersions`, `GetAgentVersion`, `SetActiveAgentVersion` |
| **Skills** | `CreateSkill`, `ListSkills`, `GetSkill`, `DeleteSkill` |
| **MCP servers** | `CreateMCPServer`, `ListMCPServers`, `GetMCPServer`, `UpdateMCPServer`, `DeleteMCPServer`, `TestMCPServer` |
| **Indexes** (RAG containers) | `CreateIndex`, `ListIndexes`, `GetIndex`, `UpdateIndex`, `DeleteIndex` |
| **Documents** (RAG-indexed) | `UploadDocument`, `GetDocument`, `GetDocumentByName`, `ListDocuments`, `ListDocumentVersions`, `DeleteDocument`, `DeleteDocumentHard`, `Search`, `SearchDocuments` |
| **Chat** | `ChatCompletion`, `CreateConversation`, `SendMessage`, `Get/List/DeleteConversation` |
| **Evals** | `CreateSuite`, `NewSuiteVersion`, `CreateEvalCase`, `RunEval`, `ListEvalRuns`, … |
| **Studio** | `GetStudioTrace`, `ReplayFromStep`, `AnalyzeFix` |

Full reference at [docs.tavora.ai/sdk](https://docs.tavora.ai/sdk/).

## Resource model

Three first-class layers; the noun describes what you put in, not how
the server indexes it. See `central-store/docs/RESOURCE_MODEL.md` for
the full diagram.

| Layer | URL prefix | Use when |
|---|---|---|
| **Indexes** | `/api/sdk/indexes/:id/documents` + `/api/sdk/search` | Knowledge you want recalled by meaning. Documents inside an Index get chunked + embedded. |
| **Memory stores** | `/api/sdk/memory-stores/:id/entries` | Named persistent key-value buckets pinned to an agent session via `memory_store_id`. The agent reads/writes via `remember()` / `recall()`. |
| **Secret vaults** | `/api/sdk/secret-vaults/:id/secrets` | Envelope-encrypted credentials the agent reads via `secret(name)` in the sandbox. The API never returns plaintext. |
| **Tenants** | `/api/sdk/tenants/:ref` | Opaque per-end-customer identifier — the platform isolates memory + secrets + audit behind it. One-line facade for the explicit primitives above. |

*Note:* Storage / Files and Collections were removed in the 2026-05-11
positioning rewrite. Customer file storage and persistent structured
records belong in the customer's backend (PocketBase / Supabase /
your own DB), exposed to the agent via MCP. See `CONTRACT.md` for
the deprecation entries.

Documents carry user-supplied provenance (`source`, `task`, `type`,
free-form `metadata`) and are name-addressable with version-on-rewrite
+ optimistic-concurrency conflict detection. Search returns chunks
(default) or one row per distinct document via `result_type:
"document"` (server-deduped). Non-markdown indexable uploads spawn an
auto-generated markdown sibling document — search hits cite the
editable form.

Errors return a typed `APIError` with `Code`, `Message`, and a
`Details` map for structured fields. `AsVersionConflict(err)` extracts
the `current_version` for retry-after-reread flows.

For agent sessions: `AgentEvent.Tokens *CallTokens` reports per-step
LLM cost in real time, `EventType*` constants disambiguate the SSE
event kind, and `event.AsInputRequest()` + `RespondToAgentInput()`
implement the pause-for-input flow agents trigger via the sandbox.

## Examples

The examples split into two families.

**Code-first agent templates** — full agent definitions (persona,
skills, evals) plus a thin Go program that runs them. These live
in a separate repo so the `tavora/` folders inside them are
copyable as starters and consumable by SDKs in other languages:

- [`tavora-examples`](https://github.com/tavora-ai/tavora-examples) —
  `research-assistant`, `support-bot`, …

**SDK pattern demos** — focused programs that exercise one corner
of the SDK API. They live here, under [`examples/`](./examples/);
each is a self-contained Go module (`cd examples/<name> && go run .`).

| Example | What it shows |
|---|---|
| [`chat`](./examples/chat) | Multi-turn agentic REPL — one `AgentSession` reused across turns |
| [`knowledge-base`](./examples/knowledge-base) | Document upload, store management, semantic search |
| [`tasklist`](./examples/tasklist) | End-to-end app template — local SQLite app exposes its domain to a Tavora agent via an MCP server registered through `CreateMCPServer` |
| [`llm-judge`](./examples/llm-judge) | ~80-line LLM-as-judge primitive — score an answer against a ground-truth value on a 0–10 rubric using Gemini |
| [`e2e`](./examples/e2e) | Live-server integration tests using `testscript` — gates on `TAVORA_URL` + `TAVORA_API_KEY` env vars |

For deployable tools (interactive chat surface, CI eval gates, etc.), see
the [`tavora-tools`](https://github.com/tavora-ai/tavora-tools) repo.
Notable subcommands:

- `tavora evals run --gate` — CI eval gate against app eval cases (replaces the old `eval-ci` example).
- `tavora rag-eval formats --gate` — verify the RAG pipeline accepts and indexes each supported document format (replaces `rag-eval-formats`).
- `tavora rag-eval judge --gate` — LLM-as-judge RAG eval against structured ground truth (replaces `rag-eval-judge`).
- `tavora-tui` — interactive Bubble-Tea chat surface against a configured agent.

Examples use a local `replace github.com/tavora-ai/tavora-sdk-go => ../..`
directive so they always build against the SDK in this checkout. Drop the
replace when copying an example into your own project.

## Authentication

All SDK calls send `X-API-Key: tvr_...` and target `/api/sdk/*`. Keys are
app-scoped — one key, one app. Create them in the admin UI
under **App Settings → API keys**.

For browser apps, use the session-token exchange described in
[Browser-side chat](https://docs.tavora.ai/sdk/browser-app/).

## Versioning

This SDK follows semantic versioning. The API is stable; breaking changes
go in major-version bumps.

## License

[MIT](./LICENSE)
