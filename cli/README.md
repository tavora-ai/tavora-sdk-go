# Tavora CLI

Developer tools for the [Tavora](https://tavora.ai) agentic intelligence
platform. This is the `cli/` module of the
[`tavora-sdk-go`](https://github.com/tavora-ai/tavora-sdk-go) monorepo —
it builds against the SDK in the same tree via a `replace` directive.
One binary, one module:

| Binary | Purpose |
|---|---|
| [`tavora`](./cmd/tavora) | The user CLI — manage projects, agents, skills, documents, MCP servers, evals, schedules, and the interactive `tavora tui` chat surface. Folder-aware against a local `tavora/` project. |

The previous standalone `tavora-tui` binary was retired 2026-05-17 —
the same code now lives in `internal/tui/` and runs via `tavora tui`.

Depends on the public Go SDK [`tavora-sdk-go`](https://github.com/tavora-ai/tavora-sdk-go),
which is the root module of this same repo.

## Code-first workflow (in flight)

The next major direction for `tavora` is a Convex-style code-first
authoring loop. Agent definitions live in a local `tavora/` folder
and three new verbs manage them:

```sh
tavora init       # scaffold tavora/ with one agent + persona + skill + eval
tavora dev        # watch + validate + sync a mutable dev draft
tavora deploy     # cut an immutable published version
```

Folder shape:

```
tavora/
  tavora.jsonc
  agents/
    support/
      agent.jsonc         # config: model, capabilities, skills, mcp, schedules
      persona.md          # system prompt
      skills/
        order-status.js   # module skill (.js)
        refund-policy.md  # prompt skill (.md)
      evals/
        happy-path.json
```

Skills are exactly three things: **`.js` files** (module skills,
sandboxed in Goja), **`.md` files** (prompt skills), and **MCP
config** inline in `agent.jsonc`. Indexes are referenced by name,
not authored in code. Secrets and `${VAR}` substitutions resolve
server-side from the target environment's store — never from a
local shell.

The driving value is that **AI coding tools (Cursor, Claude Code)
can edit Tavora agents the way they already edit the rest of a
codebase.** Git review, rollback, and shareable `require()`-able
skills follow as secondary benefits.

Existing CLI commands (`tavora agents list`, `tavora app show`,
`tavora evals run`, …) stay unchanged and operate on **runtime**
state. The new code-first commands manage **desired** state. The
browser UI under `platform.tavora.ai/` stays editable for
code-managed agents with a "managed in `tavora/agents/<id>/`" banner
— `tavora dev` reconciles on next sync.

### Verification loop

Authoring without verification is half a loop, so v0 ships a closed
feedback loop optimized for AI coding tools (Cursor, Claude Code):

- **Auto-written session logs.** Every dev-draft invocation writes a
  self-contained markdown file to `tavora/.runs/<ts>-<agent>-<sid>.md`
  — input, output, full trace (`think` snippets + skill calls +
  results), errors, tokens, version hash. The AI reads it with its
  native file-reading tools — matches the muscle memory it already
  has for `tsc` output, test reports, and coverage files. `.runs/`
  is gitignored and retention-capped (default 50).
- **Evals as pass/fail signal.** `evals/*.json` + `tavora test
  --draft` gives the AI a deterministic verification target — the
  agent equivalent of TDD. Failing cases dump their session log to
  `.runs/`.
- **Ad-hoc CLI inspection.** `tavora run <agent> "<input>" --draft`,
  `tavora session latest|<id>`, `tavora config show <agent>` (emits
  the resolved config so the AI can check "did my edit parse?"
  before spending tokens on a behavior test).

Status: design approved 2026-05-15; v0 implementation pending.
Going forward this is the primary integration path for SDK users.

### Backends — real, local, or fake

The agent's outbound `fetch()` calls run through `fetchPolicies` and
read their base URL from `context()` — both env-driven, so the same
`agent.jsonc` ships in dev and prod and `tavora env put BACKEND_URL
<url>` flips the target. Skill-author quickstart with the recipes:

  → [`docs/backends-quickstart.md`](docs/backends-quickstart.md)

Covers real-backend wiring, local-backend wiring (with tunnel notes
for SaaS), and the fake-backend path for iteration when the real
thing isn't ready yet. The fakeback runs in-process under `tavora
dev`, captures live traffic to a tail-able `requests.jsonl`, and
supports record-then-replay against a real upstream when
bootstrapping a new mock.

## Monorepo layout

The CLI is co-located with the SDK in one repo, as two modules:
`github.com/tavora-ai/tavora-sdk-go` (root, zero-dependency library) and
`github.com/tavora-ai/tavora-sdk-go/cli` (this module, heavy TUI stack).
The CLI is the *only* dependency edge — nothing in the SDK depends back
on it — so the SDK module stays dependency-free for library consumers
while the CLI's `cobra`/`bubbletea`/`charmbracelet` deps are quarantined
in `cli/go.mod`. A `replace github.com/tavora-ai/tavora-sdk-go => ../`
binds the CLI to the in-tree SDK so the two always build together.

## Install

Three channels:

```sh
# npm — wraps the Go binary; postinstall fetches the right artifact
npm i -g @tavora/cli            # or pnpm add -g, yarn global add

# Homebrew (tap not yet published — coming alongside first tagged release)
brew install tavora-ai/tap/tavora

# From source
git clone https://github.com/tavora-ai/tavora-sdk-go
cd tavora-sdk-go/cli
go install ./cmd/tavora
```

> **Note:** because `cli/go.mod` carries a committed `replace
> github.com/tavora-ai/tavora-sdk-go => ../`, the remote
> `go install github.com/tavora-ai/tavora-sdk-go/cli/cmd/tavora@latest`
> form does **not** work (the `../` path is absent from the module
> cache). Install from a clone, or via npm/Homebrew.

The npm package (`./npm/`) is a thin shim over the same Go binary —
it downloads the platform-specific prebuilt on `postinstall`. See
[`npm/README.md`](./npm/README.md) for the install flow + release
pipeline (cross-compile via GOOS/GOARCH, gzip artifacts, attach to
the GitHub Release, then `npm publish`).

## First-run

The CLI prefers credentials in this order: command flags
(`--api-key`, `--url`), env vars (`TAVORA_API_KEY`, `TAVORA_URL`),
then the config file at `~/.tavora.yaml` written by `tavora login`.

```sh
tavora login                       # interactive — captures key + URL
tavora project show
tavora agents list
tavora tui                         # interactive chat surface
```

In folder mode (when cwd contains a `tavora/` project), `tavora tui`
auto-syncs the folder, scopes the agent picker to local agents,
defaults the session target to the dev draft, and downloads
emitted assets into `<project>/.assets/<session-id>/`.

## Layout

```
tavora-sdk-go/cli/
├── cmd/
│   └── tavora/             # CLI (Cobra, resty)
├── internal/
│   ├── codefirst/          # source loader / validator / runs / scaffold
│   └── tui/                # interactive TUI (Bubble Tea v2 + bubbles v2 + lipgloss v2)
├── go.mod                  # module …/tavora-sdk-go/cli, replace => ../
└── README.md
```

## Development

```sh
go build ./...                       # build the tavora binary
go test ./...                        # run tests
go run ./cmd/tavora project show
go run ./cmd/tavora tui              # TUI from source
```

### Working against the SDK

No setup needed: `cli/go.mod` carries a committed `replace
github.com/tavora-ai/tavora-sdk-go => ../`, so the build always resolves
the SDK from the repo root. Edit SDK code in the root module and rebuild
the CLI — changes are picked up immediately, no tag bump or `go.work`.

## License

[MIT](./LICENSE)
