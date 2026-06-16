# tavora-fake-backend — concept

A small fake of the **user's host backend**, not Tavora's API. Built so
skill authors can iterate on outbound `fetch()` calls — the ones every
non-trivial skill makes — without running their real backend, and
without faking response shapes inside the agent itself. The audience
is skill authors, not CLI developers; this is distinct from
`tavora-mock`, which fakes Tavora's `/api/sdk/*` surface.

The motivating workload looks like
`tavora-demos/tasks/tavora/agents/copilot/skills/board/main.js` —
a thin facade over the host's REST API:

```js
const BASE = context('backend_url');
function req(method, path, body) {
  const r = fetch(BASE + path, { method, headers: { 'Content-Type': 'application/json' }, body: body && JSON.stringify(body) });
  if (r.status >= 400) throw new Error('backend ' + r.status);
  return r.body ? JSON.parse(r.body) : null;
}
module.exports = {
  getBoard(id) { return req('GET', '/boards/' + id); },
  createCard(listId, title) { return req('POST', '/lists/' + listId + '/cards', { title }); },
  // ...
};
```

Today, the skill author either runs their real backend (slow, requires
prod data, costs them an inner loop), or fakes responses inside the
JS, which hides the fetch-policy binding they actually wanted to test.
`tavora-fake-backend` closes that gap.

## What this is

1. **A user-backend mock.** Serves CRUD over a `db.json` the skill
   author maintains in their `tavora/` folder. Routes match *their*
   schema (`/boards/:id`, `/lists/:id/cards`), not Tavora's.
2. **A fetchPolicies binding harness.** Verifies the
   `Authorization: Bearer ${session.jwt}` header arrived at the
   declared origin with a non-empty bearer — surfaces "you declared
   the policy but forgot to pass `session_vars.jwt` at session
   creation" at dev time rather than at the user's first prod call.
3. **A trace surface.** Every response carries
   `X-Tavora-Fake-Backend: true` so `tavora dev`'s run-view can
   annotate "this call hit the fake" on the timeline.
4. **A scenario harness.** Latency injection, error injection, "the
   next call to POST /cards returns 500" — same YAML pattern as
   `tavora-mock`'s scenarios, applied to user-defined routes.

## What this is NOT

- **Not a path to prod.** Same `X-Tavora-Fake-Backend: true` self-id
  the sandbox can refuse to consume; same `TAVORA_ALLOW_FAKE=1`
  escape hatch wired into the JS `fetch` shim. Pointing a real session
  at this is a loud error.
- **Not a `tavora-mock` replacement.** That tool fakes Tavora's API
  for CLI / SDK testing. This fakes the user's backend for skill
  testing. They live in different ports, different concept docs, and
  serve different audiences.
- **Not a multi-tenant proxy.** One process serves one origin. If a
  project mocks three different host backends, three processes run
  (auto-launched; one port each).
- **Not a backend authoring tool.** The `db.json` is for iteration,
  not production. If a skill author starts building real business
  logic in custom routes, that's a sign they need a real backend.

## Starting point: json-server-go (again)

The same `/Users/jryannel/dev/ki-beratung/intern/json-server-go` skeleton
the `tavora-mock` doc forked is **also the right starting point here
— but we keep the half tavora-mock dropped.** Generic CRUD on arbitrary
top-level keys is the entire point this time. ~70% of json-server-go
carries over verbatim; we add three things and drop two.

### Reuse verbatim

- `internal/store/store.go` — the generic JSON store with debounced
  persistence, `_page`/`_limit`/`_sort` pagination, ID strategies.
- `internal/handlers/rest.go` — generic CRUD handlers.
- `internal/middleware/`, `internal/logger/`, `internal/server/` —
  same scaffolding as `tavora-mock`.

### Replace

- Auth model. Replace "JWT users + bcrypt" with "any bearer token
  accepted; expected token shape declared by the user's fetchPolicies
  config so the mock can fail loudly on missing bearer". The mock
  doesn't validate identity; it validates *presence and shape*.

### Add

- **`X-Tavora-Fake-Backend: true` middleware** on every response,
  symmetric to `tavora-mock`'s `X-Tavora-Mock` header. The Tavora
  sandbox's fetch shim gets a one-line check that refuses to surface
  responses with this header to a production session.
- **fetchPolicies introspection endpoint** —
  `GET /_meta/expected-headers` returns the declared header shape so
  the dev's `tavora dev` overlay can show
  `Authorization: Bearer ${session.jwt}` next to the live call.
- **Scenario layer** — YAML files describe latency, error-injection,
  and one-shot route overrides (e.g. "the next call to
  `POST /cards` returns 422 with `{code: validation_failed}`"). Same
  shape as `tavora-mock`'s scenarios so docs and tooling cross-pollinate.
- **Custom routes** alongside CRUD — a `routes.yaml` per mock lets the
  user define non-CRUD endpoints (e.g. `POST /search` returning a
  fixed list) the json-server-go default routing wouldn't otherwise
  cover.

### Drop

- The bcrypt user store — not the right auth model here.
- The fixed Tavora-route mounts that `tavora-mock` added — this tool
  doesn't speak the Tavora SDK surface at all.

## Layout

Two halves: the per-mock config in the user's project, and the tool
itself in `tavora-cli`.

### In the user's project

```
tavora/
├── agents/
├── skills/
└── mocks/
    ├── tasks-backend/             # one folder per mocked origin
    │   ├── db.json                # CRUD seed data
    │   ├── routes.yaml            # optional: custom non-CRUD routes
    │   └── scenarios/             # optional: error/latency injection
    │       └── card-validation-fail.yaml
    └── calendar-backend/
        └── db.json
```

`tavora.json` declares the binding from origin → mock folder:

```json
{
  "mocks": {
    "tasks-backend": {
      "origin": "http://localhost:50201",
      "expectedHeaders": {
        "Authorization": "Bearer ${session.jwt}"
      }
    }
  }
}
```

The same origin gets wired into the agent's `context('backend_url')`
when `tavora dev` runs against it (see "Integration" below).

### In tavora-cli

```
tavora-cli/
├── cmd/
│   ├── tavora/
│   ├── tavora-mock/                # existing (option 1)
│   └── tavora-fake-backend/        # NEW — single-file main.go
└── internal/
    ├── mock/                       # existing (option 1)
    ├── mockcommon/                 # NEW — shared scaffolding
    │   ├── middleware/             # request-id, logger, recoverer
    │   ├── scenarios/              # YAML scenario loader (generic)
    │   └── selfid/                 # X-Tavora-{Mock,Fake-Backend} headers
    └── fakeback/                   # NEW — the fork
        ├── server/                 # chi router + route mounts
        ├── store/                  # generic JSON store (from json-server-go)
        ├── crud/                   # CRUD handlers (from json-server-go)
        ├── routes/                 # custom-route loader (routes.yaml)
        ├── fetchpolicy/            # header-binding verifier
        └── scenarios/              # injection presets
```

Single binary: `tavora-fake-backend`. One process per mocked origin
(re-launching on a different port is cheap; multi-origin proxying in
one process adds a routing layer we don't need yet).

## Routing model

Two layers, applied in order:

1. **CRUD over `db.json`.** Top-level keys become resources. For
   `db.json` with `{ "boards": [...], "lists": [...], "cards": [...] }`,
   the mock automatically serves:

   - `GET /boards` (list) · `GET /boards/:id` (get)
   - `POST /boards` · `PUT /boards/:id` · `PATCH /boards/:id` · `DELETE /boards/:id`
   - same for `lists`, `cards`

   This is json-server-go's default, kept intact.

2. **Custom routes from `routes.yaml`.** A skill author whose backend
   does `POST /search?q=foo` (not RESTful CRUD) writes:

   ```yaml
   routes:
     - method: POST
       path: /search
       status: 200
       body:
         results:
           - { id: "card_1", title: "Buy milk" }
           - { id: "card_2", title: "Walk dog" }
     - method: POST
       path: /lists/{listId}/reorder
       status: 204
   ```

   Custom routes take precedence over CRUD when the path matches both.

## fetchPolicies binding verification

The skill author's `agent.jsonc` declares:

```jsonc
{
  "fetchPolicies": [
    { "origin": "http://localhost:50201",
      "headers": { "Authorization": "Bearer ${session.jwt}" } }
  ]
}
```

The fake backend reads `tavora.json`'s `expectedHeaders` for its
origin and, on every incoming request, asserts:

- The declared headers are present.
- Template-shaped values (`Bearer ${session.jwt}`) start with the
  declared prefix and the trailing slot is non-empty.

If a header is missing or empty, the response is 412 Precondition
Failed with a structured body the run-view annotates clearly:

```json
{
  "code": "fetchpolicy_unbound",
  "message": "expected Authorization: Bearer <session.jwt> but the bearer slot was empty",
  "hint": "pass session_vars.jwt at session creation; see agent.jsonc fetchPolicies"
}
```

This is the value-add over a generic CRUD server: the failure mode
the skill author keeps hitting in real dev (forgotten `session_vars`,
miswritten template, fetchPolicies origin mismatch) becomes a loud
runtime error at the mock layer.

## Scenarios

Same YAML shape as `tavora-mock`'s scenarios, scoped to the mock's
route surface. v0.5-equivalent shipping set:

- `latency.yaml` — every response gets a configurable delay; useful
  for stress-testing the skill's timeout handling.
- `card-validation-fail.yaml` — `POST /cards` returns 422 with a
  structured body the skill must handle.
- `auth-expired.yaml` — every response returns 401, simulating an
  expired JWT after the session was created.
- `flaky.yaml` — random 500s on N% of writes, for retry-logic
  validation.

Scenario selected via `?scenario=<name>` query or `--scenario` flag,
or programmatically via `POST /_admin/scenarios/activate { name }`.
The same `/_admin/reset` and `/_admin/snapshot` admin surface from
`tavora-mock` carries over verbatim — useful for `tavora dev`'s
"reset between runs" affordance.

## Self-identification

Every response carries `X-Tavora-Fake-Backend: true`. The sandbox's
fetch shim, when a production session pulls a response with that
header, errors out unless `TAVORA_ALLOW_FAKE=1` is set in the
agent's environment (or the session is flagged
`metadata.dev_mode=true` server-side). Same belt-and-suspenders
pattern as `tavora-mock`'s `X-Tavora-Mock` header.

This means the dev's run-view can render fake-vs-real fetches
differently in the trace overlay — a small UX win that makes the
"am I hitting the real backend?" question answerable without leaving
the agent UI.

## Integration with `tavora dev`

The killer feature, and the reason this tool ships as part of
`tavora-cli` rather than as a generic standalone:

```
$ tavora dev
[dev] scaffolding tavora/mocks/tasks-backend → http://localhost:50201
[dev] watching tavora/ for changes...
[dev] agent ready. trace at http://localhost:7777/dev
```

`tavora dev`, when it sees `tavora/mocks/<name>/`, auto-launches a
`tavora-fake-backend` subprocess per mock, assigns ports from a small
range (50201, 50202, …) or honors the `tavora.json` `mocks[].origin`
value, and substitutes the assigned origin into the agent's
`context('backend_url')` for every dev session it creates. The skill
author edits `db.json`, the next `tavora dev` invocation (or a
hot-reload trigger) restarts the fake — same inner loop as the agent
source.

Manual launch (advanced — debugging the mock itself, or running
without `tavora dev`):

```
$ tavora-fake-backend --root tavora/mocks/tasks-backend --port 50201
```

## Layered validation (where this fits)

Recapping the four-layer model from the `tavora-mock` concept:

1. **Wire round** — CLI emits X, server returns Y. → `tavora-mock`.
2. **Compute round** — message → think-block → JS execution →
   response. → real server + deterministic LLM stub.
3. **Code-first round** — edit folder → sync → invoke. → real CLI
   hitting real server in a temp dir (small e2e harness).
4. **Multi-tenant round** — session_vars + fetchPolicies + JWT
   egress. → **`tavora-fake-backend` plus the real Tavora server**
   captures this layer cleanly. The fake-backend is the "target
   backend capturing headers" the original doc referenced as an
   unbuilt component.

So the two mocks together cover layers 1 and 4 of the validation
ladder. Layers 2 and 3 still need the real Tavora server (with a
deterministic LLM stub and an e2e harness, respectively) — out of
scope here, separate proposals.

## Open questions

- **One process per origin vs. one process per project.** Today's
  plan: one per origin, simpler. A future "all-mocks-in-one" mode
  could route by Host header and listen on a single port. Defer
  until a project mocks more than 2-3 origins and the proliferation
  becomes annoying.
- **Hot reload of `db.json`.** Should edits to the JSON file be
  reflected without a restart? `json-server-go` doesn't do this
  natively. Add a fsnotify watcher with a 200ms debounce — same
  pattern `tavora dev` already uses for skill source. ~½ day.
- **Recording mode.** Should the mock have a "proxy to real backend
  + capture responses to db.json" mode for the bootstrap case?
  Probably yes, but later — v1 is bring-your-own-`db.json`. The
  capture mode is a separate ~day of work.
- **Schema validation.** If the skill author has an OpenAPI spec for
  their real backend, can the mock validate that incoming requests +
  outgoing responses conform? Defer to a v2; the v0.5 audience is
  small projects without formal API specs.

## Scope estimate

| Phase | Scope | Estimate |
|-------|-------|----------|
| v0.1  | Extract `internal/mockcommon/` from `tavora-mock`'s middleware + scenarios; refactor `tavora-mock` to import from it. Set up `internal/fakeback/` with json-server-go's CRUD + store carrying over verbatim. `cmd/tavora-fake-backend/main.go`. `X-Tavora-Fake-Backend` header. | ~½ day |
| v0.2  | `routes.yaml` custom-route loader + per-method auth disablement. `db.json` round-trip with PUT/PATCH/DELETE working. | ~½ day |
| v0.3  | fetchPolicies binding verifier — read `tavora.json` `expectedHeaders`, return 412 with `fetchpolicy_unbound` on missing/empty bearer slot. | ~½ day |
| v0.4  | Scenario layer: latency injection, error injection, one-shot route overrides. `/_admin/scenarios/activate`. | ~½ day |
| v0.5  | `tavora dev` integration: auto-launch per `tavora/mocks/<name>/`, port assignment, `context('backend_url')` substitution. fsnotify hot-reload of `db.json`. | ~1 day |
| v0.6  | Sandbox fetch shim adds `X-Tavora-Fake-Backend` refusal hook (mirror `tavora-mock`'s SDK hook). | ~½ day |

Total: ~3.5 dev days for a v0.6 that closes layer 4 of the validation
ladder and gives skill authors a real inner loop.

## Decision points

- **Standalone vs. extend `tavora-mock`.** Standalone. Different
  audience (skill authors vs CLI developers), different routing
  model (generic CRUD vs hand-written Tavora routes), different
  binary contract (auto-launched by `tavora dev` vs spun up by
  tests). Two clean tools beat one overloaded one. Shared
  scaffolding lives in `internal/mockcommon/` so the duplication
  cost stays bounded.
- **Extend upstream json-server-go vs. fork again.** Fork. Upstream
  isn't going to learn fetchPolicies binding or Tavora's
  `tavora.json` config shape, and pushing that knowledge upstream
  pollutes a generic project. Fork-and-customize is the same
  playbook as `tavora-mock`, and it worked there.
- **Per-mock folder vs. flat `db.json`.** Per-folder. A trivial mock
  is one file (`db.json`), but the moment a skill author adds custom
  routes or scenarios, the folder shape absorbs those without
  config-file gymnastics. Costs nothing at the simple case (one
  `db.json` inside the folder is the same line count as one
  top-level `db.json`).
- **`tavora-cli` subcommand vs. separate binary.** Separate binary,
  but reachable from `tavora dev` via subprocess and from operators
  via `tavora-fake-backend ...`. A `tavora mock-backend run`
  subcommand could be added later as a friendly alias if users ask
  for it; not worth the cobra wiring on day one.
- **Name.** `tavora-fake-backend` is descriptive but verbose.
  Alternatives: `tavora-host-stub` (matches "host backend"
  terminology in the board skill comments), `tavora-mockback`,
  `fauxback`. Going with `tavora-fake-backend` for the concept doc;
  open to a rename before v0.1 ships if a clearly better option
  shows up.
