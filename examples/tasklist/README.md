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

The agent that uses these tools is **authored as code** under a
`tavora/agents/tasklist/` folder. Its `agent.jsonc → mcp` block
declares this example's `/mcp` endpoint + the bearer secret name; the
agent ships to Tavora via `tavora deploy`. At runtime, when the agent
writes `require('tasklist-example').add_task({...})` inside an
`execute_js` block, Tavora's MCP client POSTs a JSON-RPC `tools/call`
to `/mcp` with the Bearer token; the example dispatches to its SQLite
store and returns the result.

Many MCP calls can happen inside a single `execute_js` turn — the
agent doesn't spend one iteration per tool call.

> **Note:** earlier versions of this example registered the MCP
> server through `client.CreateMCPServer(...)` at boot. That endpoint
> was removed when MCP authoring moved into `agent.jsonc`; the boot
> path now does no SDK setup beyond constructing the client. See the
> [tasklist tutorial](https://docs.tavora.ai/tutorials/tasklist/) for
> the full code-first authoring walkthrough.

## Setup

1. Run the Tavora stack in another terminal:
   ```
   task dev
   ```
2. Sign in at http://localhost:8080, open `/`, mint an API key for
   your app. Add a `TASKLIST_BEARER` secret to the app's vault (any
   random value — the example uses it to gate `/mcp`).
3. Author and deploy the agent (see
   [tutorials/tasklist](https://docs.tavora.ai/tutorials/tasklist/)
   for the `agent.jsonc` shape). The deploy output prints the agent's
   server-side ID — pass it as `TASKLIST_AGENT_ID` below.
4. Export env vars and run the example:
   ```
   export TAVORA_URL=http://localhost:8080
   export TAVORA_API_KEY=tvr_...
   export TASKLIST_AGENT_ID=agent_...    # from `tavora deploy`
   export TASKLIST_BEARER=...            # same value as in the app vault
   cd examples/tasklist
   go run .
   ```
5. Open http://localhost:8090.

### Env vars

| Var | Default | Notes |
|---|---|---|
| `TAVORA_URL` | — | Tavora backend base URL |
| `TAVORA_API_KEY` | — | App-scoped API key (`tvr_...`) |
| `TASKLIST_AGENT_ID` | — | Server-side ID of the deployed tasklist agent |
| `TASKLIST_BEARER` | — | Shared secret the example uses to gate `/mcp` — must match the `TASKLIST_BEARER` secret in the Tavora app's vault |
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

Then set `APP_PUBLIC_URL=https://<your-tunnel-host>` before `go run .`
(and use the same value in your `agent.jsonc` `mcp.url`).

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
│   ├── tavora/      # SDK client construction (MCP bootstrap removed)
│   └── web/         # Router, UI, JSON API, MCP server, chat SSE
│       └── templates/index.html
└── README.md
```

The `tavora/agents/tasklist/` folder that authors the agent lives
elsewhere in your project (your repo, not this example's folder) —
the example only contains the server-side runtime.

## Caveats

- **Single-tenant store.** The example's SQLite DB is not per-Tavora-
  app. One example process = one logical task-list namespace.
- **No auth on the example's own web UI.** It's a dev toy.
- **Deliberately out of scope:** multi-user auth, session persistence,
  evals, production deployment.
