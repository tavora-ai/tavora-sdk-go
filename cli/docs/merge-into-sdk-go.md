# Proposal: merge `tavora-cli` into the `tavora-sdk-go` repo

**Status:** Proposed / research only (not scheduled)
**Date:** 2026-06-06
**Author:** research session (Claude)

## Summary

Co-locate `tavora-cli` and `tavora-sdk-go` in **one repo, as two modules**. `tavora-cli`
only depends on `tavora-sdk-go`, so keeping them in separate repos forces a cross-repo
local-dev link (a gitignored `go.work` with an absolute or depth-sensitive `use` path),
which is fragile under git worktrees. Merging turns that cross-repo link into an
**in-repo** one — a committed, relative-path `go.work` that resolves correctly in any
worktree, exactly like `tavora-platform`'s `replace => ../platform` pattern.

## Motivation

- `tavora-cli` → `tavora-sdk-go` is the *only* dependency edge; nothing depends back on cli.
- Cross-repo worktrees of cli can't carry the sdk with them. A committed *relative*
  symlink/replace/`use` mis-resolves because the worktree sits at a different depth; an
  *absolute* path is machine-specific. So local-sdk co-dev needs a manual per-worktree
  `go.work` today.
- The sdk repo **already** uses a multi-module layout — `examples/{chat,knowledge-base,
  tasklist}` are separate modules with committed `replace github.com/tavora-ai/tavora-sdk-go
  => ../..`. The cli fits the same shape.

## Hard constraints (these drive the design)

1. **`tavora-sdk-go` has ZERO direct dependencies** — a deliberate, valuable property.
   Library consumers `go get` it with no transitive bloat. Must be preserved.
2. **`tavora-cli` is `go install`-able** (`go install github.com/tavora-ai/tavora-cli@latest`)
   and pulls a heavy TUI stack (cobra, bubbletea, charmbracelet/*).
3. **The SDK module path must stay at the repo root.** `github.com/tavora-ai/tavora-sdk-go`
   maps to the repo root for the module proxy — moving it breaks every external consumer.
   ⇒ The merge direction is forced: **cli moves into the sdk repo**, not the reverse.

## Target structure

```
tavora-sdk-go/                 module github.com/tavora-ai/tavora-sdk-go   (zero deps — unchanged)
├── go.mod
├── go.work        ← NEW, COMMITTED:  use ( . ./cli ./examples/chat ./examples/knowledge-base ./examples/tasklist )
├── cli/                       module github.com/tavora-ai/tavora-sdk-go/cli
│   ├── go.mod                 require github.com/tavora-ai/tavora-sdk-go v0.2.x   — NO replace
│   ├── cmd/tavora/…           (binary still named `tavora`)
│   ├── cmd/tavora-fake-backend/…
│   ├── create-tavora/         (its own module if it is one today)
│   ├── internal/…
│   └── Taskfile.yml
└── examples/…                 (unchanged)
```

## Key design decisions & rationale

### 1. Two modules, NOT one
Making cli part of the sdk *module* would drag cobra/bubbletea into `tavora-sdk-go/go.mod`,
destroying the zero-dependency property for every library consumer. A **separate
`cli/go.mod`** keeps the heavy deps quarantined; the sdk module is untouched.

### 2. Committed, in-repo `go.work` (not a committed `replace`)
- The `use` paths (`.`, `./cli`, `./examples/*`) are **relative and in-repo**, so they move
  *with* any worktree and resolve at any depth. This is the opposite of the cross-repo parent
  `go.work` that caused the tavora-platform worktree footgun (see that repo's
  `project_worktree_go_setup` notes).
- **`go install …/cli/cmd/tavora@latest` ignores `go.work` entirely** (workspace mode is off
  for `@version` installs) and resolves via cli's `require sdk v0.2.x` from the proxy — so the
  published install keeps working.
- A committed `replace => ..` in `cli/go.mod` would instead **break `go install`** (the `..`
  path doesn't exist in the module cache). That's why cli uses the `go.work` mechanism, while
  tavora-platform — whose products are never `go install`-ed — can use committed `replace`.

### 3. `cli/go.mod` keeps `require sdk v0.2.x`, no replace
Publishable. Local dev/worktrees override it via the committed `go.work`; releases and external
installs use the pinned published sdk version.

## What this buys

- Cross-repo worktree friction **gone**: a worktree carries cli + sdk together; the committed
  in-repo `go.work` resolves locally — no gitignored per-worktree `go.work`, no absolute paths.
  Same ergonomics as tavora-platform.
- SDK stays dependency-free for library consumers.
- cli's heavy deps stay isolated in `cli/go.mod`.
- Delete the `tavora-cli/go.work` + its `.gitignore` entry.

## Costs / breaking changes

- **Install path changes**: `go install github.com/tavora-ai/tavora-cli@latest`
  → `go install github.com/tavora-ai/tavora-sdk-go/cli/cmd/tavora@latest`.
  Update README, docs, CI, and any Homebrew/install scripts.
- **Release tagging**: cli releases under a Go submodule tag `cli/vX.Y.Z`, separate from the
  sdk's `vX.Y.Z`. More ceremony; document the release flow.
- **Repo history**: decide whether to preserve cli git history (`git subtree`/filter-repo
  import) or do a clean `git mv` snapshot.

## Migration steps (rough)

1. Branch on `tavora-sdk-go`.
2. Import the cli tree under `cli/` (subtree merge to keep history, or plain copy).
3. Rewrite cli's module path `github.com/tavora-ai/tavora-cli` → `…/tavora-sdk-go/cli`
   across `go.mod` + all internal self-imports.
4. Set `cli/go.mod` to `require github.com/tavora-ai/tavora-sdk-go v0.2.x` (no replace).
5. Add committed repo-root `tavora-sdk-go/go.work` with `use ( . ./cli ./examples/* )`.
6. Port `Taskfile.yml`, docs, CI workflows; update the install path everywhere.
7. Verify (below), tag `cli/v…`, publish.
8. Leave the old `tavora-cli` repo intact until verified; then archive it (optionally add a
   README pointer to the new install path).

## Verification

- `cd tavora-sdk-go && go build ./...` and `cd cli && go build ./...` green.
- `cd cli && go list -m -f '{{.Dir}}' github.com/tavora-ai/tavora-sdk-go` → prints the in-repo
  sdk dir (workspace active for local dev).
- In a **fresh worktree** of the merged repo: same `go list` prints the *worktree's* sdk dir —
  no manual `go.work` needed.
- `go install github.com/tavora-ai/tavora-sdk-go/cli/cmd/tavora@<tag>` from a clean machine
  succeeds (proves the published path + pinned sdk version, go.work ignored).

## Rollback

The old `tavora-cli` repo stays untouched during migration; if anything regresses, keep using
it. No destructive step until the new path is verified and the old repo is deliberately archived.

## Open questions

- `create-tavora/` — is it a separate module/binary? If so it becomes `cli/create-tavora/` with
  its own go.mod and a `use ./cli/create-tavora` line.
- Preserve cli git history, or clean snapshot?
- Reconcile Go toolchain versions across the merged repo (sdk + cli are both `go 1.25.0` today).
