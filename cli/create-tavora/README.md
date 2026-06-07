# create-tavora

The `npm create tavora` initializer. Scaffolds a fresh Tavora agents
project — a directory containing a `tavora/` folder with a starter
agent, persona, skills, and an eval case.

## Usage

```sh
npm create tavora
npm create tavora my-app
npm create tavora my-app -- --yes
```

The same works with `yarn create tavora`, `pnpm create tavora`, and
`bun create tavora` (those package managers all forward to the same
`create-tavora` package on npm).

Result:

```
my-app/
  README.md
  tavora/
    tavora.jsonc
    AGENTS.md
    .gitignore
    agents/starter/
      agent.jsonc
      persona.md
      skills/now/{skill.md,main.js}
      skills/style/skill.md
      evals/basic.json
```

## How it works

This package is intentionally tiny — under ~200 lines of zero-dep
Node. It only handles the wrapping flow (prompt for a name, make the
outer directory, write a README explaining the layout). The actual
agent-folder scaffold is delegated to `tavora init` so the templates
have a single source of truth in
[`tavora-cli/internal/codefirst/scaffold`](../internal/codefirst/scaffold/scaffold.go).

To find a `tavora` binary it tries, in order:

1. `$TAVORA_BIN` — explicit path override.
2. `tavora` on `PATH` — an already-installed CLI.
3. `npx --yes @tavora/cli` — fetch the published wrapper, which in
   turn pulls the Go binary from a GitHub Release.

## Options

| Flag | Meaning |
|---|---|
| `[name]` (positional) | Directory to create. Prompted if omitted. |
| `--in-place` | Scaffold into the current directory instead of a new one. |
| `--yes`, `-y` | Skip prompts; use defaults. |
| `--template <name>`, `-t` | Template to use (default: `starter`). |
| `--help`, `-h` | Show help. |

Pass options after `--` to keep `npm create` from intercepting them:

```sh
npm create tavora my-app -- --yes
```

## Releasing

Automated alongside `@tavora/cli` by
[`.github/workflows/release.yml`](../.github/workflows/release.yml) —
both packages publish in lockstep on every `v*` tag push, so their
versions must agree (the workflow's first step fails fast if they
don't). See [`../npm/README.md`](../npm/README.md#releasing) for the
full release procedure.

Unlike `@tavora/cli`, this package has no postinstall step and no
GitHub Release dependency — it's pure JS. The lockstep is a UX
choice (matched pair = predictable upgrades), not a technical one.
