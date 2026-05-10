# Agentic Chat (Tavora SDK example)

A minimal multi-turn REPL that drives a Tavora agent from the Go SDK.
Every turn runs the agent's full reasoning loop (`execute_js`, tool
calls, MCP skills, fetch, search — whatever the product has
configured), so the same conversation can mix simple Q&A with
code-reasoning work.

This is the reference implementation for building your own
SDK-consumer chat UI. See `main.go` — the whole thing is ~160 LOC.

## Setup

```bash
export TAVORA_URL=http://localhost:8080          # or https://api.tavora.ai
export TAVORA_API_KEY=tvr_...                    # product-scoped API key
```

Create the API key in the Tavora admin UI: sign in at
`/platform`, open your product, go to **Settings → API keys**,
**New key**. Keys start with `tvr_`.

## Run

```bash
# Simplest — inherits product defaults
go run .

# With custom title + system prompt + tools
go run . \
  --title "Trip planning" \
  --system-prompt "You plan trips. Use search() for live data." \
  --tools search,list_stores

# Override the model
go run . --model gemini-2.5-flash
```

At the `You:` prompt, type a message. REPL commands:

| Command         | Effect                                     |
| --------------- | ------------------------------------------ |
| `/exit` `/quit` | Leave the chat                             |
| `/reset`        | Start a fresh session (drops history)      |
| `/help`         | Show the command list                      |

## What you'll see

Agent output goes to **stdout**; trace events go to **stderr** so you
can pipe either separately. A typical turn:

```
You: What's 2 + 2? And then tell me the current year.
  [execute_js] var year = new Date().getFullYear(); 2 + 2 + " / " + year
  [execute_js_result] 4 / 2026

Agent: 4. The current year is 2026.

[3 steps · 412 prompt + 38 completion tokens]
```

Agentic reasoning (`execute_js` + sandbox primitives) is always on —
the `--tools` flag controls *additional* ADK function-call tools, not
the core sandbox. MCP servers registered in your product auto-expose
as `require('<server-name>')` inside `execute_js` (see
`examples/tasklist` for that pattern).

## SDK shape this example demonstrates

- `tavora.NewClient(url, apiKey)` — client init.
- `client.GetProduct(ctx)` — sanity check the API key.
- `client.CreateAgentSession(ctx, CreateAgentSessionInput{...})` —
  one session per REPL run (or per `/reset`). History is server-side.
- `client.RunAgent(ctx, sessionID, message, onEvent)` — SSE stream
  of `AgentEvent`s. Event types you'll see:
  - `execute_js` / `execute_js_result` — sandbox work
  - `tool_call` / `tool_result` — ADK function tools
  - `sandbox_event` — primitive-level events (fetch, ai, web_search, …)
  - `response` — the model's user-facing reply
  - `error` — a turn failed
  - `done` — turn finished, carries the `RunSummary`

## Production notes

- API keys grant full access to the scoped product. Treat them like
  database credentials — don't bake them into distributed binaries.
- `RunAgent` blocks until the turn completes or errors. Wrap the call
  in `context.WithTimeout` if you need a per-turn deadline.
- `CreateAgentSession` is cheap but not free (DB write + config load).
  For high-volume fanout, reuse one session across many turns; only
  create a new session when you want a clean history.
