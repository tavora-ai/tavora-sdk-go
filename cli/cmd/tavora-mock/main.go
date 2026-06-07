// Command tavora-mock runs a fake Tavora server suitable for
// onboarding, CLI/SDK contract tests, demos, and screencasts. It does
// NOT validate the agent runtime — see tavora-cli/docs/
// tavora-mock-server-concept.md for the layered-validation framing.
//
// Listening defaults to :50100 — well clear of `tavora-go` on
// :50000. Override with -addr. Every response carries
// `X-Tavora-Mock: true` so a misconfigured SDK refuses to operate
// against it (the SDK check lands in v0.5).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tavora-ai/tavora-sdk-go/cli/internal/mock/server"
)

var (
	flagAddr     = flag.String("addr", "127.0.0.1:50100", "listen address host:port")
	flagLogLevel = flag.String("log-level", "info", "slog level: debug|info|warn|error")
	flagLogText  = flag.Bool("log-text", false, "use text-format slog handler (default: JSON)")
	flagCORS     = flag.Bool("cors", true, "enable permissive CORS for browser callers")
	flagScenario = flag.String("scenario", "", "default SSE scenario name (per-request ?scenario=… overrides)")
	flagSeed     = flag.Int64("seed", 0, "seed for deterministic UUID generation (two runs with the same seed produce the same trace)")
)

func main() {
	flag.Parse()
	os.Exit(run())
}

func run() int {
	logger, err := newLogger(*flagLogLevel, *flagLogText)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tavora-mock: %v\n", err)
		return 2
	}

	handler, err := server.New(server.Options{
		Logger:          logger,
		CORS:            *flagCORS,
		DefaultScenario: *flagScenario,
		Seed:            *flagSeed,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "tavora-mock: %v\n", err)
		return 2
	}

	srv := &http.Server{
		Addr:              *flagAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
		// No WriteTimeout — SSE responses (v0.2+) stay open for the
		// duration of an agent run, which can outlast any sensible
		// per-request write budget.
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("tavora-mock listening",
			"addr", srv.Addr,
			"cors", *flagCORS,
			"scenario_default", *flagScenario,
			"seed", *flagSeed,
		)
		logger.Info("clients must opt in to talk to a mock",
			"env", "TAVORA_ALLOW_MOCK=1",
			"why", "every response carries X-Tavora-Mock: true; the Go SDK refuses without the opt-in",
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-stop:
		logger.Info("shutdown signal", "signal", sig.String())
	case err := <-errCh:
		logger.Error("server error", "err", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown failed", "err", err)
		return 1
	}
	logger.Info("tavora-mock stopped")
	return 0
}

func newLogger(level string, useText bool) (*slog.Logger, error) {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "", "info":
		lvl = slog.LevelInfo
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		return nil, fmt.Errorf("unknown -log-level %q", level)
	}
	opts := &slog.HandlerOptions{Level: lvl}
	if useText {
		return slog.New(slog.NewTextHandler(os.Stdout, opts)), nil
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, opts)), nil
}
