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

    ws, err := client.GetWorkspace(ctx)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Connected to workspace: %s\n", ws.Name)
}
```

Every SDK method is a direct call on `*Client` — no nested service
namespaces. Methods are context-aware, so callers can cancel, deadline,
or trace any request.

## What the SDK covers

| Area | Methods |
|---|---|
| **Workspace** | `GetWorkspace` |
| **Agents — sessions** | `CreateAgentSession`, `ListAgentSessions`, `GetAgentSession`, `DeleteAgentSession`, `RunAgent`, `GetAgentSystemPrompt` |
| **Agents — configs + versions** | `CreateAgentConfig`, `ListAgentConfigs`, `Get/Update/DeleteAgentConfig`, `SetActiveAgentVersion` |
| **Agent versions + deployments** | `CreateAgentVersion`, `ListAgentVersions`, `GetAgentVersion`, `UpsertAgentDeployment`, `ListAgentDeployments` |
| **Skills** | `CreateSkill`, `ListSkills`, `GetSkill`, `DeleteSkill` |
| **MCP servers** | `CreateMCPServer`, `ListMCPServers`, `GetMCPServer`, `UpdateMCPServer`, `DeleteMCPServer`, `TestMCPServer` |
| **Knowledge** | `CreateStore`, `ListStores`, `GetStore`, `UpdateStore`, `DeleteStore`, `UploadDocument`, `GetDocument`, `ListDocuments`, `DeleteDocument`, `Search` |
| **Chat** | `ChatCompletion`, `CreateConversation`, `SendMessage`, `Get/List/DeleteConversation` |
| **Evals + Promotions** | `CreateSuite`, `RunEval`, `ProposePromotion`, `ApprovePromotion`, … |
| **Policies** | `UpsertToolPolicy`, `ApproveApprovalRequest`, … |
| **Studio** | `GetStudioTrace`, `ReplayFromStep`, `AnalyzeFix` |

Full reference at [docs.tavora.ai/sdk](https://docs.tavora.ai/sdk/).

## Examples

Working example apps live under [`examples/`](./examples/). Each is a
self-contained Go module — `cd examples/<name> && go run .`.

| Example | What it shows |
|---|---|
| [`chat`](./examples/chat) | Multi-turn agentic REPL — one `AgentSession` reused across turns |
| [`support-bot`](./examples/support-bot) | RAG-augmented chat over a documents folder using `Conversation` + `SendMessage` |
| [`research-assistant`](./examples/research-assistant) | Single-turn agent with `search` + `list_stores` tools |
| [`knowledge-base`](./examples/knowledge-base) | Document upload, store management, semantic search |
| [`tasklist`](./examples/tasklist) | End-to-end product template — local SQLite app exposes its domain to a Tavora agent via an MCP server registered through `CreateMCPServer` |
| [`eval-ci`](./examples/eval-ci) | CI-style eval runner — gate promotions on suite pass rate |
| [`rag-eval`](./examples/rag-eval), [`rag-eval-formats`](./examples/rag-eval-formats), [`rag-eval-judge`](./examples/rag-eval-judge) | RAG quality measurement using `Search` + `ChatCompletion` + LLM-judge scoring |
| [`e2e`](./examples/e2e) | Live-server integration tests using `testscript` — gates on `TAVORA_URL` + `TAVORA_API_KEY` env vars |

For a fully-featured interactive chat surface, see the
[`tavora-tui`](https://github.com/tavora-ai/tavora-tools/tree/main/cmd/tavora-tui)
binary in the [`tavora-tools`](https://github.com/tavora-ai/tavora-tools)
repo. The same is true for the [`tavora` CLI](https://github.com/tavora-ai/tavora-tools/tree/main/cmd/tavora).

Examples use a local `replace github.com/tavora-ai/tavora-sdk-go => ../..`
directive so they always build against the SDK in this checkout. Drop the
replace when copying an example into your own project.

## Authentication

All SDK calls send `X-API-Key: tvr_...` and target `/api/sdk/*`. Keys are
workspace-scoped — one key, one workspace. Create them in the admin UI
under **Workspace Settings → API keys**.

For browser apps, use the session-token exchange described in
[Browser-side chat](https://docs.tavora.ai/sdk/browser-app/).

## Versioning

This SDK follows semantic versioning. The API is stable; breaking changes
go in major-version bumps.

## License

[MIT](./LICENSE)
