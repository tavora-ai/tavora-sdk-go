// Package canned returns the default response shapes the mock emits
// for each SDK route. v0.1 returns hand-written stubs; v0.3 will
// regenerate the response types from `tavora-go/api/swagger.json`.
//
// Every shape here mirrors the JSON the real server emits — field
// names match the SDK's struct tags so a real SDK call against the
// mock parses without `unknown field` errors. Stay disciplined about
// field names; that's the whole point of the mock as a contract pin.
package canned

import (
	"time"
)

// FixedNow is the timestamp used wherever the response shape carries
// a CreatedAt/UpdatedAt. The mock isn't bothered with wall-clock
// determinism in v0.1 — `--seed` (v0.4) will make this configurable.
var FixedNow = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)

// Project mirrors tavora-sdk-go.Project (GET /api/sdk/project).
func Project() map[string]any {
	return map[string]any{
		"id":          "00000000-0000-0000-0000-000000000001",
		"team_id":     "00000000-0000-0000-0000-000000000002",
		"name":        "Mock Project",
		"slug":        "mock-project",
		"description": "A pretend project served by tavora-mock.",
		"created_at":  FixedNow,
		"updated_at":  FixedNow,
	}
}

// Capabilities mirrors tavora-sdk-go.listCapabilitiesResponse. The
// list is the static set the CLI's "unknown capability" linter checks
// against; keep it loosely in sync with the real registry.
func Capabilities() map[string]any {
	return map[string]any{
		"capabilities": []map[string]any{
			{"name": "context", "description": "Read per-session context map."},
			{"name": "search", "description": "Search bound retrieval indexes."},
			{"name": "fetch", "description": "Outbound HTTP via the fetch egress shim."},
			{"name": "execute_js", "description": "Run a JS block in the sandbox."},
			{"name": "asset_write", "description": "Persist a binary asset."},
		},
	}
}

// SourceSyncResult mirrors tavora-sdk-go.SourceSyncResult. The shape
// matches what tavora dev expects so the CLI's debounced sync round-trip
// resolves cleanly. Agent IDs are derived from the local IDs in the
// incoming manifest so the CLI's local→server mapping table populates.
func SourceSyncResult(project string, manifestHash string, localAgentIDs []string) map[string]any {
	agents := make([]map[string]any, 0, len(localAgentIDs))
	for _, localID := range localAgentIDs {
		agents = append(agents, map[string]any{
			"localId":    localID,
			"agentId":    AgentIDFor(localID),
			"draftId":    "draft-" + localID,
			"sourceHash": manifestHash,
		})
	}
	return map[string]any{
		"draftHash": manifestHash,
		"agents":    agents,
		"syncedAt":  FixedNow,
	}
}

// SourceValidateResult mirrors the wrapper SourceValidate returns
// (`{ "issues": [...] }`).
func SourceValidateResult() map[string]any {
	return map[string]any{
		"issues": []any{},
	}
}

// SourceDeployResult mirrors tavora-sdk-go.SourceDeployResult.
func SourceDeployResult(project string, localAgentIDs []string) map[string]any {
	agents := make([]map[string]any, 0, len(localAgentIDs))
	for _, localID := range localAgentIDs {
		agents = append(agents, map[string]any{
			"localId":   localID,
			"agentId":   AgentIDFor(localID),
			"versionId": "v-" + localID,
			"semver":    "1.0.0",
		})
	}
	return map[string]any{
		"version":    "1.0.0",
		"agents":     agents,
		"deployedAt": FixedNow,
	}
}

// AgentSession mirrors tavora-sdk-go.AgentSession.
func AgentSession(id, title, model string, indexIDs []string) map[string]any {
	if model == "" {
		model = "mock-model"
	}
	if title == "" {
		title = "Mock Session"
	}
	return map[string]any{
		"id":                id,
		"project_id":        "00000000-0000-0000-0000-000000000001",
		"title":             title,
		"system_prompt":     "You are a mock agent.",
		"model":             model,
		"tools_config":      map[string]any{},
		"metadata":          map[string]any{},
		"status":            "active",
		"index_ids":         indexIDs,
		"created_at":        FixedNow,
		"updated_at":        FixedNow,
		"prompt_tokens":     0,
		"completion_tokens": 0,
		"step_count":        0,
		"duration_ms":       0,
	}
}

// AgentIDFor maps a local agent id to a stable mock-side UUID by
// concatenating a known prefix. Stable across calls so the CLI's
// local→server mapping table stays consistent.
func AgentIDFor(localID string) string {
	// Deterministic, human-recognizable. Pad to UUID-ish length so a
	// downstream type that demands 36 chars doesn't choke; v0.4
	// (`--seed`) will replace with real UUID derivation.
	const padding = "0000-0000-0000-000000000000"
	id := "mock-" + localID + "-" + padding
	if len(id) > 36 {
		id = id[:36]
	}
	return id
}
