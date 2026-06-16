# Backends quickstart — the env-driven dev/prod toggle

This is the one-page recipe for wiring your agent's `fetch()` calls
across environments. Same `agent.jsonc` in dev and prod — one env
var flips the target.

## TL;DR

```jsonc
// agent.jsonc
"fetchPolicies": [{
  "origin":  "${env.BACKEND_URL}",
  "headers": { "Authorization": "Bearer ${session.jwt}" }
}],
"context": {
  "backend_url": "${env.BACKEND_URL}"
}
```

```bash
# Dev — point at a local backend (or fakeback)
tavora env put BACKEND_URL http://localhost:8090

# Prod — point at the real thing (run from a separate deployment binding)
tavora env put BACKEND_URL https://api.example.com
```

Same file, two values. Server resolves `${env.BACKEND_URL}` at
session-create against the deployment's env store.

---

## How the wiring works

Three resolution sites, each with a different security stance:

| Where | Reads | Resolved at | LLM-visible? |
|---|---|---|---|
| `fetchPolicies[i].origin` | `${env.X}` | session-create | No |
| `fetchPolicies[i].headers["..."]` | `${env.X}`, `${session.X}` | fetch egress | No |
| `context: { key: "..." }` | `${env.X}` only | session-create | **Yes** |
| `CreateAgentSessionInput.Context` | literal (caller-supplied) | session-create | **Yes** |
| `CreateAgentSessionInput.SessionVars` | literal (caller-supplied) | fetch egress only | No |

The skill code reads via `context('backend_url')` (LLM-visible side)
or implicitly via the policy match on origin (LLM never sees the
bearer). Both come from the same `BACKEND_URL` env entry, so prod
and dev stay symmetric.

**Secrets**: the `context: {}` block rejects references to env
entries marked `is_secret=true` at sync time with a
`secret_in_context` validation issue. Keep secrets in fetchPolicies
header templates (where they never reach the LLM).

---

## Three setups, ranked by friction

### 1. Real backend (the default for most teams)

You have a backend running somewhere reachable from the Tavora
server. SaaS customers: a public URL. Self-hosted: anything the
server can resolve.

```bash
tavora env put BACKEND_URL https://api.real-backend.com
tavora dev          # syncs agent.jsonc with resolved values
tavora run agent "do the thing"
```

No mocks, no fakes, no extra tooling. This is what production runs
against. Most iteration should happen here.

### 2. Local backend during dev

You run your backend locally (`task dev`, `docker compose up`, etc.)
and want the agent's fetches to hit it.

**Self-hosted Tavora**: easy — set `BACKEND_URL=http://localhost:8090`
on the dev deployment. The server is on your machine; it can reach
localhost.

**SaaS Tavora**: the cloud server can't reach your laptop directly.
Two options:
- **Tunnel** your local backend (`ngrok`, `cloudflared`, `tailscale`)
  and `tavora env put BACKEND_URL <tunnel-url>`.
- **Use a public staging URL** as your dev target instead.

### 3. Fake backend (when iteration speed matters)

`tavora-fake-backend` is a small json-server-style mock that auto-
launches under `tavora dev` when a `tavora/mocks/<name>/` folder
exists. Useful when:
- Your real backend takes 30+ seconds to start.
- You're offline (planes, trains, …).
- You want to script error scenarios (`tavora-fake-backend
  --scenario auth-expired`).
- You're bootstrapping a new skill without the backend ready yet.

Fakeback **only works when the Tavora server can reach the fake's
URL** — same constraint as #2. SaaS customers: tunneling defeats
much of the speed win; consider option #2 instead. Self-hosted +
Tavora team: works directly.

Quick recipe:

```bash
mkdir -p tavora/mocks/my-backend
cat > tavora/mocks/my-backend/db.json <<'EOF'
{ "boards": [ { "id": "b1", "title": "Today" } ] }
EOF

tavora env put BACKEND_URL http://127.0.0.1:50201
# On the Tavora server's env, set: TAVORA_ALLOW_FAKE=1
# (this is a one-time operator opt-in for self-hosted; SaaS skips this)

tavora dev    # banner shows: my-backend  http://127.0.0.1:50201
tavora run agent "list my boards" --session-var jwt=demo
```

The fakeback writes every request to `tavora/mocks/my-backend/
requests.jsonl` — `tail -f` it (use `tail -F` to survive rotation)
to see the agent's outbound traffic live. Sensitive headers
(`Authorization`, `Cookie`, …) are redacted by default.

For more on fakeback's surface — custom routes, scenarios,
recording a real upstream — see `docs/tavora-fake-backend-concept.md`.

---

## The `--session-var` flag

When the agent's fetchPolicies reference `${session.X}` (canonical
case: `Bearer ${session.jwt}` for the end-user's bearer), the host
backend would normally inject the value at `CreateAgentSession`.
`tavora run` is the AI verification loop — there's no host backend —
so you supply the value on the command line:

```bash
tavora run agent "ping the backend" \
  --session-var jwt=eyJhbGc...token \
  --session-var tenant_id=tenant_42
```

Each `--session-var NAME=VALUE` lands in the session's `session_vars`
map. The template `Bearer ${session.jwt}` resolves against it at
fetch egress, and the resolved bearer never appears in JS scope or
the run trace.

---

## Common errors and what they mean

**`fetchpolicy_unbound` 412 from the fake backend**

You declared `Authorization: Bearer ${session.jwt}` in
fetchPolicies but didn't pass `--session-var jwt=...` to
`tavora run` (or your host backend isn't supplying it). The
fakeback's binding verifier catches this loudly so you don't burn
an hour debugging "the agent says it works but nothing happens."

**`unresolved-template` 422 from `tavora dev` sync**

agent.jsonc references `${env.BACKEND_URL}` but the deployment
doesn't have it set. Hint in the error names the key; run the
suggested `tavora env put` command.

**`secret_in_context` 422 from `tavora dev` sync**

Your `context: {}` block references an env entry stored with
`is_secret=true`. Context is LLM-visible; secrets must not flow
there. Either move the reference to a fetchPolicies header (where
the LLM never sees it) or re-put the env entry without the
`--secret` flag.

**"agent fetch hit a fake backend" runtime error**

The sandbox's fetch shim saw `X-Tavora-Fake-Backend: true` on a
response and the server isn't opted in. Set `TAVORA_ALLOW_FAKE=1`
on the Tavora server's environment (self-hosted only).

---

## Why this design

- **Same source file across environments.** No env-specific
  branching in agent.jsonc means PR review sees the agent's
  contract independent of which deployment runs it.
- **Server-side resolution.** Values live in the encrypted
  `deployment_env` store, not in developer shells. Rotating
  `BACKEND_URL` is one `tavora env put` away, not a code change.
- **Distinct lanes.** fetchPolicies origin and context are
  separately authored, separately resolved, separately gated. A
  context-block check (refusing secret references) is mechanical,
  not policy-by-convention.
- **The skill code stays unchanged.** `const BASE =
  context('backend_url')` reads from a uniform per-session map —
  same `.js` file in dev, staging, prod.

See:
- `tavora-go/docs/agent-env-context-concept.md` — design rationale
  for the resolution boundaries.
- `tavora-cli/docs/tavora-fake-backend-concept.md` — fakeback
  feature surface (routes, scenarios, recording, request log).
