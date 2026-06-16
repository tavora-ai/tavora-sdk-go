# Changelog

All notable changes to the Tavora Go SDK are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
this project follows pre-v1 [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
where minor bumps may carry breaking changes.

## [Unreleased]

### Added

- **`SessionVars` on `CreateAgentSessionInput`** — per-session string
  values the server substitutes into `fetchPolicies` header templates
  as `${session.<key>}`. Canonical use: forward the requesting end
  user's bearer JWT so the agent's outbound calls back to the host
  backend authenticate as that user without the LLM ever seeing the
  credential. Encrypted at rest on the server; write-only (the server
  never returns the values back). Requires the server to be running
  with `TAVORA_SECRET_KEK`; otherwise `CreateAgentSession` returns
  503. Keys must match `[A-Za-z0-9_]+`; total payload ≤ 8 KiB.

- **`FetchPolicies` on `CreateAgentSessionInput`** + new `FetchPolicy`
  type. Declares which outbound origins the sandbox's fetch egress
  shim injects headers into. Companion to `SessionVars` — vars carry
  credentials, policies declare where they go. A header template
  like `"Bearer ${session.jwt}"` resolves at egress against the
  session's vars. A policy match also acts as explicit allowlist
  approval for the origin (so the agent's fetch() to that origin
  doesn't trigger a per-URL prompt). Write-only; cap of 16 entries;
  each origin must include a scheme.

## [0.4.0] — 2026-05-17

The code-first pivot. Source-sync (`PUT /api/sdk/source-sync`,
driven by `tavora dev` from a local `tavora/` folder) is now the
single writer for agents, skills, eval cases, and MCP server
bindings. The platform's matching mutator endpoints came off, and
this release prunes the SDK client to match.

### Added

- **Code-first source APIs.** `SourceSync`, `SourceValidate`,
  `SourceDeploy`, `SourceRename`, `SourceDelete`, `SourceDiff`,
  `SourceExport`. These are what the `tavora` CLI uses internally
  to author agents from a local folder; application code rarely
  calls them directly.
- **Code-first markers on `AgentConfig`** — `CodeFirstProject`
  and `CodeFirstLocalID` (the project + local-id pair from
  `tavora.jsonc` / `agent.jsonc`) are now exposed so callers can
  resolve a server agent UUID by local id without source-syncing
  first.
- **`AgentID` + `Target` on `CreateAgentSessionInput`** — drives
  the "run against staged draft" path used by `tavora run --draft`.

### Removed (BREAKING)

- **Draft + publish flow.** `UpdateAgentDraft`,
  `DiscardAgentDraft`, `PublishAgent`, `RevertAgent`, and the
  `DraftConfig` type are gone. Promotion is `SourceDeploy`-only.
- **Agent-version writes.** `CreateAgentConfig`,
  `UpdateAgentConfig`, `SetActiveAgentVersion`,
  `CreateAgentVersion` are gone. Agents are scaffolded via
  `tavora init` and modified via the local folder.
- **MCP server registry.** `mcp_servers.go` deleted entirely.
  `ListMCPServers`, `CreateMCPServer`, `UpdateMCPServer`,
  `DeleteMCPServer`, `TestMCPServer` all removed. MCP servers are
  now declared inline in `agent.jsonc → mcp` and flow through
  `SourceSync`.
- **Skill CRUD.** `CreateSkill`, `DeleteSkill` are gone; skills
  are authored as `.js` / `.md` files under
  `tavora/agents/<id>/skills/`.
- **Eval-case writes.** `CreateEvalCase`, `UpdateEvalCase`,
  `DeleteEvalCase`, `RunEval` are gone. Cases live as JSON files
  under `tavora/agents/<id>/evals/`. Trigger advisory runs per
  agent via `RunAgentEval`.
- **Eval-suite writes.** `CreateSuite`, `DeleteSuite`,
  `NewSuiteVersion` are gone. Suites are read-only via SDK; the
  Phase-12 promotion gate was dismantled.
- **Tool policies + approvals.** Removed alongside the platform
  Phase-14 deletion on 2026-05-13: `UpsertToolPolicy`,
  `DeleteToolPolicy`, `ListToolPolicies`,
  `ListPendingApprovals`, `ApproveApprovalRequest`,
  `RejectApprovalRequest`.
- **`SeedApp`** — there's no in-server starter agent seed
  anymore; agents land via `SourceSync` when the operator runs
  `tavora init` + `tavora dev`.
- **`run_eval_on_publish` + `draft_config` fields** on
  `AgentConfig`. The browser no longer publishes; run-on-deploy
  is a `tavora deploy --run-evals` flag.

### Changed

- **`AgentConfig` shape narrowed.** Live config (persona, skills,
  stores, provider, model) lives on the agent row directly;
  `AgentVersion` rows are append-only history snapshots written
  by the code-first publish path.
- **`UpdateAgentSettings`** is the only `agent_configs` mutator
  left and only patches the `EvalSuiteID` pin.

### Examples

- `examples/research-assistant`, `examples/support-bot` removed
  (consolidated into `tavora-cli/cmd/tavora-tui` and the
  `tasklist` tutorial).
- `examples/tasklist` rewritten around code-first authoring — the
  agent is declared under `tavora/agents/tasklist/` and shipped
  via `tavora deploy`; the example binary just hosts the MCP
  server and proxies a chat UI.
- `examples/eval-ci` README updated to reflect the new
  `tavora evals run <agent> --gate` shape.

### Migration guide

```go
// before (v0.3.x)
draft := tavora.DraftConfig{PersonaMD: "...", ...}
_, _ = client.UpdateAgentDraft(ctx, agentID, draft)
_, _ = client.PublishAgent(ctx, agentID)

// after (v0.4.0)
// 1. Edit tavora/agents/<id>/persona.md + agent.jsonc locally.
// 2. tavora dev  (auto-syncs as you save)
// 3. tavora deploy  (publishes an immutable version)
// The SDK only consumes the deployed agent at runtime via
// CreateAgentSession{AgentID: ...}.
```

```go
// before (v0.3.x)
mcp, _ := client.CreateMCPServer(ctx, tavora.CreateMCPServerInput{
    Name: "tasklist-example",
    URL:  "https://example.com/mcp",
    ...
})

// after (v0.4.0)
// Declare inline in tavora/agents/<id>/agent.jsonc:
//   "mcp": [{
//     "name": "tasklist-example",
//     "url": "${APP_PUBLIC_URL}/mcp",
//     "transport": "streamable_http",
//     "auth": {"type":"bearer","tokenRef":"TASKLIST_BEARER"}
//   }]
// Then tavora deploy.
```

## [0.3.0] — 2026-05-15

### Removed (BREAKING)

- **Memory stores API** — `MemoryStore`, `MemoryEntry`,
  `CreateMemoryStoreInput`, `UpdateMemoryStoreInput`, and all
  client methods (`memory_stores.go` deleted). The
  `remember()` / `recall()` / `memories()` sandbox primitives
  were also removed from agent JS — cross-session memory now
  lives in the customer's database, injected as context at
  session-create.
- **Tenant facade** — `Tenant`, `ProvisionTenantInput`,
  `UpdateTenantInput`, and all client methods (`tenants.go`
  deleted). The platform no longer models per-end-customer
  state behind an opaque `tenant_ref`; end-user isolation
  moves to the customer's app layer.
- **Session-pinned credentials vault** — `MemoryStoreID`,
  `SecretVaultID`, and `TenantRef` fields removed from both
  `AgentSession` and `CreateAgentSessionInput`. Sessions
  inherit credentials from the app's designated vault
  (see "Changed" below).
- **`secret(name)` sandbox primitive** — gone from the agent's
  JS vocabulary. Tools (LLM dispatcher, Brave search) read
  credentials internally from the app vault; the agent never
  touches plaintext.

### Changed

- **One credentials vault per app.** The app designates one
  `secret_vault` via `PUT /api/sdk/app/vault` (was
  `/app/llm-vault`). The runtime LLM resolver reads provider
  keys (`openai_api_key`, `anthropic_api_key`,
  `gemini_api_key`, `openrouter_api_key`, `edenai_api_key`,
  `ollama_base_url`) and the Brave search pack reads
  `brave_api_key` from the same vault on every dispatch.
- The `TenantAuditEntry` type name is retained for schema
  continuity (the underlying `tenant_audit_log` table is
  unchanged) but the documentation no longer describes it
  as tenant-scoped.

### Migration guide

If your code creates agent sessions with primitive pins:

```go
// before (v0.2.x)
sess, err := client.CreateAgentSession(ctx, tavora.CreateAgentSessionInput{
    MemoryStoreID: "store-uuid",
    SecretVaultID: "vault-uuid",
    TenantRef:     "acme-corp",
})

// after (v0.3.0)
sess, err := client.CreateAgentSession(ctx, tavora.CreateAgentSessionInput{
    IndexIDs: []string{"index-uuid"}, // search() scoping survives
})
```

For per-end-user state, manage it in your own database and inject
relevant facts via `SystemPrompt` or the initial user message at
session-create time.

To replace `agent.secret("openai_api_key")` from inside skill
code: store the key in the app's vault, designate the vault via
`PUT /api/sdk/app/vault`, and the LLM resolver picks it up
automatically.

## [0.2.1] — earlier releases

See git history.
