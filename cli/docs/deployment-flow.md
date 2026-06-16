# Deployment versioning (Convex-style)

Tavora follows the **Convex deployment model**: every push from the CLI
creates a personal **dev** version, and a published version can then be
**promoted** to **staging** or **production**. Each environment serves
exactly one version at a time, and promotion is just repointing an
environment at a version — including rolling back to an older one.

```
tavora dev      ── sync source ──▶ draft
tavora deploy   ── cut version ──▶ vN   (your dev environment now runs vN)
tavora promote --to staging      ▶ staging → vN
tavora promote --to prod         ▶ prod    → vN
tavora promote --to prod --version 1   ▶ prod → v1   (rollback)
tavora status   ── show what dev / staging / prod each serve
```

## Model

- **Versions** are immutable snapshots, numbered per agent (v1, v2, …),
  cut from the latest synced draft by `tavora deploy`.
- **Environments** (`dev` / `staging` / `prod`) are deployment targets.
  `dev` is personal (one per developer per agent); `prod` is the single
  production target; `staging` is created on first promote.
- **Promotion** points an environment at a version. It's idempotent and
  reversible — `--version N` pins (or rolls back to) a specific version;
  omitting it promotes the latest.

Deployments are **per-agent**. The CLI verbs are project-level: they
iterate every agent in the project (use `--agent <localId>` to scope to
one), and the backend resolves each agent's own dev/staging/prod target.

## Commands

| Command | What it does |
|---|---|
| `tavora dev [--once]` | Watch/validate the local `tavora/` folder and sync a dev **draft**. |
| `tavora deploy [--agent <id>]` | Cut a published **version** from the latest draft; point your dev at it. |
| `tavora promote --to staging\|prod [--agent <id>] [--version N]` | Repoint an environment at a version. |
| `tavora status [--project <name>]` | Show the version each environment currently serves. |
| `tavora env\|secret <get\|put\|list\|delete> --agent <id> --deployment <slug> …` | Per-deployment config/secrets (`${env.X}` / `${secret.X}`). |

`status` (and `promote`) print the server **agent id** and deployment
**slug** you need for the `env`/`secret` commands.

## Authentication

The CLI authenticates with an `X-API-Key`. Against a fresh backend:

```bash
export TAVORA_URL=http://localhost:8091

# 1. register (once)
curl -s -X POST "$TAVORA_URL/api/users" -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com","password":"correct-horse-battery-staple"}'

# 2. login → access_token (JWT)
ACCESS=$(curl -s -X POST "$TAVORA_URL/api/auth/login" -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com","password":"correct-horse-battery-staple"}' \
  | python3 -c 'import json,sys;print(json.load(sys.stdin)["tokens"]["access_token"])')

# 3. mint an X-API-Key from the JWT
export TAVORA_API_KEY=$(curl -s -X POST "$TAVORA_URL/api/auth/session-token" \
  -H "Authorization: Bearer $ACCESS" -H 'Content-Type: application/json' -d '{}' \
  | python3 -c 'import json,sys;print(json.load(sys.stdin)["api_key"])')
```

(`tavora login` automates the equivalent and writes `~/.tavora.yaml`.)

## One-shot demo

With `TAVORA_URL` + `TAVORA_API_KEY` set, run the whole flow:

```bash
mise run demo          # or: bash cli/scripts/demo.sh
```

It scaffolds a throwaway project and walks init → dev → deploy →
promote staging → promote prod → re-deploy → rollback, printing
`tavora status` at each step.

## REST endpoints (new backend)

All under `/api/sdk/*`, `X-API-Key` (or `Bearer` JWT) auth:

| Method | Path | Purpose |
|---|---|---|
| `PUT` | `/api/sdk/source-sync` | Upsert a dev draft from the manifest. |
| `POST` | `/api/sdk/source-deploy` | Cut a version; pin dev to it. |
| `POST` | `/api/sdk/source-promote` | Promote a version to staging/prod (`{to, versionNumber?}`). |
| `GET` | `/api/sdk/source-status?project=` | Per-agent latest + dev/staging/prod versions. |
| `GET` | `/api/sdk/agents/{agent_id}/deployments` | List an agent's deployments. |
| `GET/PUT/DELETE` | `/api/sdk/agents/{agent_id}/deployments/{slug}/env/{key}` | Per-deployment env/secret KV. |
