# tasklist — agent-driven Reminders-style demo

A small Go web app that uses the **Tavora Go SDK** to let an agent drive a
task-list UI. Ask the agent *"Create a task list of all large German cities
to visit"* and watch it call `create_task_list` + six `add_task`s in real
time.

The stack: `chi` router, `modernc.org/sqlite` storage, Go `html/template` +
Tailwind via CDN for the UI, Server-Sent Events for the live agent
stream, and an **embedded MCP server** for the tools the agent calls.

## How it works

The example hosts its own MCP server at `/mcp` (streamable HTTP
transport) that exposes six tools:

| Tool | What it does |
|---|---|
| `create_task_list` | Create a list |
| `list_task_lists` | Enumerate lists |
| `delete_task_list` | Delete a list + its tasks |
| `add_task` | Append a task to a list |
| `list_tasks` | Enumerate a list's tasks |
| `complete_task` | Mark a task done |

On startup the example calls `client.CreateMCPServer(...)` once —
registering the URL, transport, and a Bearer auth config with the Tavora
product the API key is scoped to. After that, every agent run in the
product auto-discovers these tools (see `internal/agent/mcp.go` in the
main repo — `mcpSvc.LoadSandboxPacks` dials each enabled MCP server,
lists its tools, and exposes them inside the Goja sandbox as
`require('<server-name>').<tool>(args)`). No per-agent wiring.

When the agent writes `require('tasklist-example').add_task({...})`
inside an `execute_js` block, Tavora's MCP client POSTs a JSON-RPC
`tools/call` to `/mcp` with the Bearer token; the example dispatches to
its SQLite store and returns the result. The result surfaces in the
sandbox trace and goes back to the agent's reasoning loop, streaming
into the browser via SSE. Many MCP calls can happen inside a single
`execute_js` turn — the agent doesn't spend one iteration per tool
call.

Registration is idempotent: re-running the example does not create
duplicate MCP server records. Config drift (URL change after a port
swap) triggers delete + create.

## Why MCP and not "webhook skills"

Tavora's SDK also exposes `CreateSkill(type: "webhook", ...)`, but in the
current code path webhook skills are only loaded into a legacy
`ToolRegistry` used for session-creation validation — they are never
wired into the ADK agent at run time. MCP servers are. If you try to
pass a webhook-skill name in `CreateAgentSession.Tools` you get
`unknown tool: …`. MCP is the correct extension mechanism today.

## Setup

1. Run the Tavora stack in another terminal:
   ```
   task dev
   ```
2. Sign in at http://localhost:8080, open `/platform`, mint an API key
   for your product.
3. Export env vars and run the example:
   ```
   export TAVORA_URL=http://localhost:8080
   export TAVORA_API_KEY=tvr_...
   cd examples/tasklist
   go run .
   ```
4. Open http://localhost:8090.

### Env vars

| Var | Default | Notes |
|---|---|---|
| `TAVORA_URL` | — | Tavora backend base URL |
| `TAVORA_API_KEY` | — | Product-scoped API key (`tvr_...`) |
| `APP_PORT` | `8090` | Port the example listens on |
| `APP_PUBLIC_URL` | `http://localhost:$APP_PORT` | Base URL Tavora uses to reach the example's `/mcp` endpoint — set to an ngrok/cloudflared URL when pointing at a hosted Tavora |
| `APP_DB` | `tasklist.db` | SQLite file path; `:memory:` for ephemeral |

### Hosted Tavora (non-localhost)

The `/mcp` URL Tavora hits needs to be reachable from wherever Tavora
runs. For a local example against a cloud Tavora, expose it:

```
cloudflared tunnel --url http://localhost:8090
# or: ngrok http 8090
```

Then set `APP_PUBLIC_URL=https://<your-tunnel-host>` before `go run .`.

## Try it

Good prompts to try in the chat panel on the right:

- *Create a task list of all large German cities to visit.*
- *Add "Check passport" and "Book trains" to that list.*
- *Which lists do I have?*
- *Mark "Berlin" as done.*
- *Delete the "groceries" list.*

Watch the chat log — each tool call streams in as it happens
(`→ add_task(...)` / `← add_task ok`), and the sidebar refreshes live.

## Layout

```
examples/tasklist/
├── main.go
├── internal/
│   ├── store/       # SQLite schema + CRUD
│   ├── tavora/      # MCP server registration (bootstrap)
│   └── web/         # Router, UI, JSON API, MCP server, chat SSE
│       └── templates/index.html
└── README.md
```

## Caveats

- **Product-wide tool visibility.** Because Tavora loads MCP servers at
  the product level (not per-agent), every agent in this product will
  see the tasklist tools. Fine for a demo; for production Tavora would
  need per-version MCP binding (analogous to `AgentVersion.skills_json`).
- **Single-tenant store.** The example's SQLite DB is not per-Tavora-
  product. One example process = one logical task-list namespace.
- **No auth on the example's own web UI.** It's a dev toy.
- **Deliberately out of scope:** multi-user auth, session persistence,
  versioned agents, evals, policies, production deployment.
