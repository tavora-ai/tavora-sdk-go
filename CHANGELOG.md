# Changelog

All notable changes to the Tavora Go SDK are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
this project follows pre-v1 [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
where minor bumps may carry breaking changes.

## [Unreleased]

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
