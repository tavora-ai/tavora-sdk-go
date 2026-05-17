// Tasklist — an example Tavora SDK consumer that exposes its task-list
// domain to an agent via an embedded MCP server.
//
// The agent itself is authored as code under tavora/agents/tasklist/
// in your project; `tavora deploy` ships it. This binary serves the
// /mcp endpoint and a chat UI that drives the deployed agent over the
// SDK.
//
// Usage:
//
//	export TAVORA_URL=http://localhost:8080
//	export TAVORA_API_KEY=tvr_...
//	export TASKLIST_AGENT_ID=agent_...                 # from `tavora deploy`
//	export TASKLIST_BEARER=<random>                    # matches the TASKLIST_BEARER secret in the project's vault
//	export APP_PORT=8090                               # optional, default 8090
//	export APP_PUBLIC_URL=http://localhost:8090        # optional, derived from APP_PORT
//	go run .
package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/tavora-ai/tavora-sdk-go/examples/tasklist/internal/store"
	"github.com/tavora-ai/tavora-sdk-go/examples/tasklist/internal/web"
	tavora "github.com/tavora-ai/tavora-sdk-go"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	tavoraURL := os.Getenv("TAVORA_URL")
	tavoraKey := os.Getenv("TAVORA_API_KEY")
	if tavoraURL == "" || tavoraKey == "" {
		return fmt.Errorf("TAVORA_URL and TAVORA_API_KEY must be set")
	}

	agentID := os.Getenv("TASKLIST_AGENT_ID")
	if agentID == "" {
		return fmt.Errorf("TASKLIST_AGENT_ID must be set to the server-side ID of the deployed tasklist agent (see `tavora deploy` output)")
	}

	secret := os.Getenv("TASKLIST_BEARER")
	if secret == "" {
		return fmt.Errorf("TASKLIST_BEARER must be set to the same value as the TASKLIST_BEARER secret in the Tavora project's vault")
	}

	port := envOr("APP_PORT", "8090")
	publicURL := envOr("APP_PUBLIC_URL", "http://localhost:"+port)
	dbPath := envOr("APP_DB", "tasklist.db")

	st, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer st.Close()

	client := tavora.NewClient(tavoraURL, tavoraKey)

	srv, err := web.New(st, client, agentID, secret)
	if err != nil {
		return fmt.Errorf("new web server: %w", err)
	}

	slog.Info("tasklist ready",
		"listen", ":"+port,
		"public_url", publicURL,
		"tavora_url", tavoraURL,
		"agent_id", agentID,
		"db", dbPath,
	)
	return http.ListenAndServe(":"+port, srv.Routes())
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
