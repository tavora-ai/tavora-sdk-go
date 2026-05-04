// Tasklist — an example Tavora SDK consumer that exposes its task-list
// domain to an agent via webhook skills.
//
// Usage:
//
//	export TAVORA_URL=http://localhost:8080
//	export TAVORA_API_KEY=tvr_...
//	export APP_PORT=8090                         # optional, default 8090
//	export APP_PUBLIC_URL=http://localhost:8090  # optional, derived from APP_PORT
//	go run .
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/tavora-ai/tavora-sdk-go/examples/tasklist/internal/store"
	localtavora "github.com/tavora-ai/tavora-sdk-go/examples/tasklist/internal/tavora"
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

	port := envOr("APP_PORT", "8090")
	publicURL := envOr("APP_PUBLIC_URL", "http://localhost:"+port)
	dbPath := envOr("APP_DB", "tasklist.db")

	st, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer st.Close()

	secret, err := randomSecret()
	if err != nil {
		return err
	}

	client := tavora.NewClient(tavoraURL, tavoraKey)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := localtavora.EnsureMCPServer(ctx, client, publicURL, secret); err != nil {
		return fmt.Errorf("register mcp server: %w", err)
	}

	srv, err := web.New(st, client, secret)
	if err != nil {
		return fmt.Errorf("new web server: %w", err)
	}

	slog.Info("tasklist ready",
		"listen", ":"+port,
		"public_url", publicURL,
		"tavora_url", tavoraURL,
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

func randomSecret() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("random: %w", err)
	}
	return hex.EncodeToString(b), nil
}
