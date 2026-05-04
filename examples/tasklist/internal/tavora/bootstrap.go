// Package tavora registers the example's MCP server with the Tavora
// workspace the example's API key is scoped to. Idempotent: running the
// example twice does not create duplicates.
//
// The agent runtime auto-loads every enabled MCP server in a workspace
// (see internal/agent/mcp.go:30 in tavora-go), so no per-agent binding is
// needed — registration alone makes the tools available to every agent
// in the workspace.
package tavora

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	tavora "github.com/tavora-ai/tavora-sdk-go"
)

// ServerName is the name under which we register the MCP server in the
// workspace. Used as the idempotency key.
const ServerName = "tasklist-example"

// EnsureMCPServer registers (or re-registers) the tasklist MCP server with
// Tavora. The SDK exposes UpdateMCPServer, but we follow the simpler
// delete+create path on drift since config changes are rare and the code
// stays symmetric with how skills would be managed.
func EnsureMCPServer(ctx context.Context, client *tavora.Client, publicURL, sharedSecret string) error {
	endpoint := publicURL + "/mcp"
	authConfig, _ := json.Marshal(map[string]any{
		"type":  "bearer",
		"token": sharedSecret,
	})

	existing, err := client.ListMCPServers(ctx)
	if err != nil {
		return fmt.Errorf("list mcp servers: %w", err)
	}
	for _, s := range existing {
		if s.Name != ServerName {
			continue
		}
		// Always delete+recreate. We can't tell from URL/transport alone
		// whether auth_config is current — tasklist generates a fresh
		// random bearer secret on every startup, so any existing row's
		// secret is stale by definition. Previous "skip if URL matches"
		// shortcut caused 401s on Test MCP because Tavora kept sending
		// the first-run secret forever.
		slog.Info("mcp server exists, recreating to refresh shared secret", "name", s.Name, "prev_url", s.URL)
		if err := client.DeleteMCPServer(ctx, s.ID); err != nil {
			return fmt.Errorf("delete stale mcp server: %w", err)
		}
		break
	}

	created, err := client.CreateMCPServer(ctx, tavora.CreateMCPServerInput{
		Name:       ServerName,
		URL:        endpoint,
		Transport:  "streamable_http",
		AuthConfig: authConfig,
	})
	if err != nil {
		return fmt.Errorf("create mcp server: %w", err)
	}
	slog.Info("mcp server registered", "name", created.Name, "id", created.ID, "url", created.URL)
	return nil
}
