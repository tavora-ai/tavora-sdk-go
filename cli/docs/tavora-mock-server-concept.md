# tavora-mock-server — concept

A small fake Tavora server, single binary, swagger-conformant wire shapes,
zero database. Built primarily for:

1. **Onboarding** — "try the CLI without setting up Postgres + the real
   server." A new developer runs `tavora-mock-server` and `tavora init`
   against it, and the round-trip works end-to-end (with fake responses).
2. **CLI / SDK contract tests** — pin the wire shapes the CLI emits and
   consumes so a server-side rename surfaces in CI rather than at the
   user's terminal.
3. **Demos and screencasts** — deterministic UUIDs, deterministic
   timings, no flakiness.

What this is **not**:

- Not a way to validate the thinking-core. The mock doesn't run JS, doesn't
  call an LLM, doesn't enforce policies. Compute-layer correctness needs the
  real server + Postgres + a deterministic LLM stub — out of scope here.
- Not a path to prod. Every response carries `X-Tavora-Mock: true` so a
  misconfigured client can refuse to use it for anything load-bearing.
- Not a generic JSON server. The routes, schemas, and SSE streams match
  Tavora's surface exactly; arbitrary collections aren't supported.

## Starting point: `json-server-go`

`/Users/jryannel/dev/ki-beratung/intern/json-server-go` is the right
skeleton — ~1500 LOC, chi router, slog + lumberjack, YAML config, JWT auth,
admin `/_admin/reset` + `/_admin/snapshot` endpoints. We fork it (vendor a
copy under `tavora-cli/internal/mock/`) and specialize:

### Reuse verbatim

- HTTP scaffolding: `internal/server/`, `internal/middleware/`,
  `internal/logger/`. Same chi router setup, same request-id + structured
  logs, same lumberjack rotation.
- Auth helpers: `internal/auth/` (JWT issue/verify with HS256). The mock
  doesn't validate identity — it accepts any bearer — but we still want to
  issue tokens on `/auth/login` so the CLI's API-key + JWT exchange flows
  through unchanged.
- Admin endpoints: `/_admin/reset` (restore seed state) and
  `/_admin/snapshot` (dump in-memory state to JSON). Already the perfect
  shape for autonomous test loops.

### Replace

- `internal/store/store.go` and `internal/handlers/rest.go` (the generic
  CRUD-on-arbitrary-keys part). Tavora has a fixed schema; replace with
  per-route handlers that emit shapes derived from
  `tavora-go/api/swagger.json`.

### Add

- **Tavora-route mounts**: hand-written routes for the SDK surface:
  - `GET  /api/sdk/project`
  - `GET  /api/sdk/capabilities`
  - `PUT  /api/sdk/source-sync`
  - `POST /api/sdk/source-validate`
  - `POST /api/sdk/source-deploy`
  - `POST /api/sdk/deployments`
  - `GET  /api/sdk/deployments/{slug}/env`, `PUT/DELETE /{slug}/env/{key}`
  - `POST /api/sdk/agent-sessions`, `POST /api/sdk/agent-sessions/{id}/runs`
    (SSE — see below)
  - `GET  /health`, `GET  /api/version`
- **SSE support**: `/agent-sessions/{id}/runs` and `/boards/{id}/copilot`
  stream agent events (`session`, `text`, `step`, `step_result`,
  `response`, `usage`, `done`). The mock emits a scripted sequence from a
  YAML scenario file, with configurable per-event delays so a screencast
  pauses naturally.
- **Stateful narrative**: a `tavora init` → `tavora dev` → run flow has to
  return the same agent ID across calls. Keep a per-process in-memory
  state: `map[project][]agent`, `map[session][]events`. Optionally
  persist to a JSON file (cribbed from json-server-go's store), gated by
  a `--persist` flag — default off so each boot is a clean slate.
- **`X-Tavora-Mock: true` response header** on every response, set by a
  small middleware. The Go SDK gets a one-line check on each response that
  refuses to operate when the header is present AND the binary isn't a
  CI/dev build — caught at the SDK layer, not the user's terminal.
- **Deterministic IDs**: `--seed <int>` flag → all UUIDs and timestamps
  are derived from this seed. Two runs with the same seed produce the
  same trace, so demos and golden-file tests are reproducible.

### Drop

- Generic REST CRUD on arbitrary top-level keys.
- `_page`/`_limit`/`_sort`/`_order`/`_start`/`_end`/`q=` query parameters
  (Tavora doesn't use the json-server pagination dialect).
- Per-method auth knobs in the YAML config (Tavora has a fixed auth
  policy per route — encode in code, not config).

## Layout

```
tavora-cli/
├── cmd/
│   ├── tavora/              # the user CLI (unchanged)
│   └── tavora-mock/         # NEW — single-file main.go that wires the mock
└── internal/
    ├── codefirst/           # (unchanged)
    └── mock/                # NEW — the fork
        ├── server/          # chi router, route mounts
        ├── store/           # in-memory state for projects/agents/sessions
        ├── scenarios/       # YAML-driven SSE scripts
        ├── canned/          # default response generators (schema-derived)
        ├── middleware/      # mock-header injector + reused json-server-go ones
        ├── auth/            # JWT issuer (no validation)
        └── admin/           # /_admin/reset, /_admin/snapshot
```

Single binary: `tavora-mock`. Launches on `:50100` by default (next slot
in the port registry, well clear of `tavora-go` on `:50000`). Config via
flags first, optional YAML second — most users won't need a config file.

## Wire-shape source of truth

`tavora-go/api/swagger.json` is regenerated on every server change via
`task be:swagger`. The mock reads this file **at build time**: a small
`go generate` step extracts response schemas and emits Go structs into
`internal/mock/canned/schemas.go`. Two consequences:

1. Compile-time drift detection. If the server adds a required field, the
   mock won't build until the canned responses cover it.
2. Mock binary doesn't need swagger.json at runtime — the schemas are
   embedded.

Alternative considered: load swagger.json at runtime and validate every
outgoing response against the schema. Better drift detection but adds a
runtime JSON-schema dep and slows startup. Build-time is enough; pair
with a CI step that fails if `swagger.json` changed but the mock didn't
regenerate.

## Scenarios

A scenario is a YAML file describing one canned conversation between the
mock and the CLI/SDK:

```yaml
# scenarios/happy-path.yaml
name: happy-path
description: source-sync succeeds, agent runs cleanly, returns a final answer.

routes:
  - method: PUT
    path: /api/sdk/source-sync
    status: 200
    body:
      draftHash: sha256:abc123
      agents:
        - localId: copilot
          agentId: 0000-0000-0000-0001

  - method: POST
    path: /api/sdk/agent-sessions/{sid}/runs
    sse:
      - { delay: 0ms,   event: session,      sessionId: 0000-0000-0000-0010 }
      - { delay: 200ms, event: step,         tool: execute_js, code: "log('hi')" }
      - { delay: 400ms, event: step_result,  result: "hi" }
      - { delay: 500ms, event: text,         delta: "DONE\n" }
      - { delay: 600ms, event: text,         delta: "Hello." }
      - { delay: 700ms, event: response,     content: "Hello." }
      - { delay: 700ms, event: usage,        prompt: 120, completion: 5 }
      - { delay: 750ms, event: done }
```

Ship five or six canonical scenarios:

- `happy-path.yaml` — everything green.
- `service-not-configured.yaml` — `TAVORA_SECRET_KEK` missing → 503 with
  `code: service_not_configured` (regression-pins the fix we shipped).
- `unresolved-template.yaml` — `${env.BACKEND_URL}` unset → 422 with
  `issues: [{code: unresolved-template, ...}]`.
- `legacy-key.yaml` — `POST /deployments` 400 BAD_REQUEST with the
  "this API key has no recorded owner" message (the bind-flow path).
- `lazy-skill-discovery.yaml` — multi-step SSE: the agent calls
  `skillList()` then `skillGet('math')` then `require('math')`. Useful
  for screencast.
- `slow-stream.yaml` — same as happy-path but with `2s` delays so a
  screencast can pause on each step.

Scenario selected via `--scenario <name>` or `?scenario=<name>` query
parameter (the latter so a test can switch scenarios per request without
restarting the binary).

## Self-identifying as fake

Every response carries `X-Tavora-Mock: true`. The SDK adds a one-line
check in its response middleware:

```go
if resp.Header.Get("X-Tavora-Mock") == "true" && !allowMockServer {
    return nil, fmt.Errorf("tavora: refusing to operate against a mock server (set TAVORA_ALLOW_MOCK=1 to override — only valid in tests)")
}
```

The escape hatch (`TAVORA_ALLOW_MOCK=1`) is set by the mock binary's
launcher script and by the CI test harness. Anything else (production
code, a CLI a real customer runs) refuses to talk to it. This is the
"belt" — the "suspenders" is that the mock listens on `:50100` by
default, away from anything real.

## What this enables

- `tavora-mock` is on the box → developer can run `tavora init / dev /
  deploy` and see the inner-loop work end-to-end without Postgres or an
  LLM. The folder gets written, the bind step writes `.env.local`, the
  watcher debounces, source-sync responds, the run streams a canned
  trace. Onboarding goes from "set up the stack" to "run one binary."
- CI smoke test: spin up `tavora-mock --scenario happy-path`, run
  `tavora dev --once` against a fixture folder, assert no error. ~2s per
  cycle, fully hermetic.
- SDK contract tests: hit each route with the SDK, assert the response
  parses into the typed struct without `unknown field` errors. Pins
  forward-compat (the server adding a field) and backward-compat (the
  server removing one).
- Screencasts: `--scenario slow-stream` + `--seed 42` produces the same
  trace every time.

## Layered validation (where this fits)

For context — the round we're validating here is **layer 1 (wire round)**
from the four-layer model:

1. **Wire round** — CLI emits X, server returns Y. → `tavora-mock-server`
   (this doc).
2. **Compute round** — message → think-block → JS execution → response.
   → real server + deterministic LLM stub (separate proposal).
3. **Code-first round** — edit folder → sync → invoke. → real CLI hitting
   real server in a temp dir (small e2e harness).
4. **Multi-tenant round** — session_vars + fetchPolicies + JWT egress.
   → real server + a test target backend capturing headers.

The mock server only validates layer 1, but the other three layers benefit:
they get a fast inner loop for the parts that don't need to be hot, and
the real components for the parts that do.

## Open questions

- **SSE vs WebSocket** — Tavora is SSE-only today. If the platform adds
  WebSocket transport (e.g. for the studio replay surface), the mock
  needs to follow. Cheap to add later — chi has `nhooyr.io/websocket`
  patterns we can crib.
- **Scenario authoring** — YAML is fine for the canonical set; for
  authoring custom scenarios, do we ship a `tavora-mock record` mode
  that captures real-server traffic into a YAML scenario? Probably
  later — start with hand-written.
- **Auth depth** — do we want the mock to honor `X-API-Key` matching, or
  accept any non-empty key? The latter is simpler and matches the
  "fake" framing. Skip key validation for v1; add `--require-key foo`
  later if a test needs to assert auth-rejection paths.
- **Persistence** — same question. v1: in-memory only, restarts wipe.
  Add `--persist db.json` later if a test needs cross-restart state.

## Scope estimate

| Phase | Scope | Estimate |
|-------|-------|----------|
| v0.1  | Fork `json-server-go` into `tavora-cli/internal/mock/`. Mount stub routes (no SSE) that return 200 + an empty shape. `X-Tavora-Mock` header. `cmd/tavora-mock/main.go`. | ~½ day |
| v0.2  | Add SSE handler + happy-path scenario. CLI's `tavora init`/`dev`/`deploy` succeed against it end-to-end. | ~½ day |
| v0.3  | Schema generator from `swagger.json` → `canned/schemas.go`. CI step that detects drift. | ~½ day |
| v0.4  | Five canonical scenarios + `?scenario=` selector + `--seed`. | ~½ day |
| v0.5  | SDK `X-Tavora-Mock` refusal + `TAVORA_ALLOW_MOCK=1` escape hatch. | ~½ day |

Total: ~2-3 dev days for a v0.5 that covers onboarding + CI smoke tests.

## Decision points

- **Fork vs library** — Fork. `json-server-go` is built for generic CRUD;
  we strip half of it. A clean fork keeps the diff narrow.
- **Schema-driven mock vs hand-written** — Schema-driven for response
  shapes (so drift is caught at build); hand-written for the route list
  (only ~10 routes; not worth the complexity of full server-stub
  generation).
- **`tavora-cli/internal/mock/` vs separate repo** — Same repo for now.
  Mocks should live next to the contract they implement; if the mock
  outgrows the CLI repo it can be extracted.
