// Command tavora-fake-backend serves a fake of the user's host
// backend so skill authors can iterate on outbound fetch() calls
// (from skills like
// `tavora-demos/tasks/tavora/agents/copilot/skills/board/main.js`)
// without running their real backend.
//
// Routing model — json-server style: top-level keys in db.json become
// resources, served as GET/POST/PUT/PATCH/DELETE under
// /{resource}[/{id}]. v0.2 adds custom routes from routes.yaml;
// v0.3 adds fetchPolicies header verification; v0.4 adds scenarios.
//
// Every response carries `X-Tavora-Fake-Backend: true` so the
// sandbox's fetch shim (v0.6) can refuse to surface responses to
// production sessions without an explicit opt-in.
//
// Defaults to 127.0.0.1:50201 — distinct from `tavora-go` (:50000)
// and `tavora-mock` (:50100). Use distinct ports when running
// multiple fake-backend processes (one per mocked origin) under
// `tavora dev`.
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
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/fetchpolicy"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/recorder"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/requestlog"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/routes"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/scenarios"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/server"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/store"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/watch"
)

// stringSliceFlag collects repeated `-expect-header NAME=TEMPLATE`
// flags. Each entry overrides the same-named header from
// expected-headers.yaml.
type stringSliceFlag []string

func (s *stringSliceFlag) String() string     { return fmt.Sprint([]string(*s)) }
func (s *stringSliceFlag) Set(v string) error { *s = append(*s, v); return nil }

var (
	flagAddr       = flag.String("addr", "127.0.0.1:50201", "listen address host:port")
	flagRoot       = flag.String("root", ".", "per-mock folder containing db.json (and, later, routes.yaml + scenarios/)")
	flagDB         = flag.String("db", "", "override db.json path (default: <root>/db.json)")
	flagRoutes     = flag.String("routes", "", "override routes.yaml path (default: <root>/routes.yaml)")
	flagExpectFile = flag.String("expected-headers", "", "override expected-headers.yaml path (default: <root>/expected-headers.yaml)")
	flagScenarios  = flag.String("scenarios-dir", "", "override scenarios folder (default: <root>/scenarios)")
	flagScenario   = flag.String("scenario", "", "scenario to activate at startup (per-request ?scenario=… overrides)")
	flagWatch      = flag.Bool("watch", true, "hot-reload db.json on disk changes (fsnotify-driven)")
	flagUpstream   = flag.String("upstream", "", "absolute URL of an upstream server; requests the local handlers can't satisfy get proxied here (e.g. https://api.real-backend.com)")
	flagRecordTo   = flag.String("record-to", "", "capture proxied responses to this file (default: <root>/recorded.json when -upstream is set; pass `off` to disable capture)")
	flagLogReq     = flag.String("log-requests", "", "append one JSONL entry per request to this file; an AI coding tool tailing it sees the agent's live traffic. Pass <root>/requests.jsonl, an absolute path, or `auto` to use <root>/requests.jsonl")
	flagLogMaxSize = flag.Int("log-max-size", 50, "request log rotation threshold in MB")
	flagLogMaxKeep = flag.Int("log-max-backups", 3, "request log rotated-file retention count")
	flagLogRedact  = flag.String("log-redact-headers", "", "comma-separated header names whose values are replaced with <redacted> in the log (empty = default safe list: Authorization, Cookie, Set-Cookie, X-Api-Key, Proxy-Authorization; pass `none` to disable redaction entirely)")
	flagPersist    = flag.Bool("persist", false, "write mutations back to db.json (default: in-memory only)")
	flagIDStrat    = flag.String("id-strategy", "int", "id auto-assignment: int|uuid")
	flagLogLevel   = flag.String("log-level", "info", "slog level: debug|info|warn|error")
	flagLogText    = flag.Bool("log-text", false, "use text-format slog handler (default: JSON)")
	flagCORS       = flag.Bool("cors", true, "enable permissive CORS for browser callers")
	flagExpect     stringSliceFlag
)

func init() {
	flag.Var(&flagExpect, "expect-header", "header expectation NAME=TEMPLATE (repeatable, overrides file)")
}

func main() {
	flag.Parse()
	os.Exit(run())
}

func run() int {
	logger, err := newLogger(*flagLogLevel, *flagLogText)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tavora-fake-backend: %v\n", err)
		return 2
	}

	dbPath := *flagDB
	if dbPath == "" {
		dbPath = filepath.Join(*flagRoot, "db.json")
	}
	routesPath := *flagRoutes
	if routesPath == "" {
		routesPath = filepath.Join(*flagRoot, "routes.yaml")
	}
	expectPath := *flagExpectFile
	if expectPath == "" {
		expectPath = filepath.Join(*flagRoot, "expected-headers.yaml")
	}
	scenariosDir := *flagScenarios
	if scenariosDir == "" {
		scenariosDir = filepath.Join(*flagRoot, "scenarios")
	}

	// LoadFolderRoutes merges routes.yaml + recorded.yaml from
	// <root>. When -routes is set explicitly we honor that file
	// alone; the merge only applies in the default folder mode.
	var customRoutes []routes.Route
	if *flagRoutes == "" {
		customRoutes, err = routes.LoadFolderRoutes(*flagRoot)
	} else {
		customRoutes, err = routes.LoadFile(routesPath)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "tavora-fake-backend: %v\n", err)
		return 1
	}
	expected, err := fetchpolicy.LoadFile(expectPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tavora-fake-backend: %v\n", err)
		return 1
	}
	scenReg, err := scenarios.LoadDir(scenariosDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tavora-fake-backend: %v\n", err)
		return 1
	}
	if *flagScenario != "" {
		if err := scenReg.Activate(*flagScenario); err != nil {
			fmt.Fprintf(os.Stderr, "tavora-fake-backend: %v (available: %v)\n", err, scenReg.Names())
			return 2
		}
	}
	// CLI flags merge over file (last write wins per header name).
	for _, kv := range flagExpect {
		name, tmpl, ok := strings.Cut(kv, "=")
		if !ok || name == "" {
			fmt.Fprintf(os.Stderr, "tavora-fake-backend: -expect-header must be NAME=TEMPLATE, got %q\n", kv)
			return 2
		}
		if expected == nil {
			expected = make(map[string]string)
		}
		expected[name] = tmpl
	}

	st, err := store.Open(store.Config{
		File:           dbPath,
		IDStrategy:     *flagIDStrat,
		Persist:        *flagPersist,
		DebounceWindow: 500 * time.Millisecond,
	}, logger)
	if err != nil {
		logger.Error("store open failed", "err", err)
		return 1
	}
	defer st.Close()

	// Recorder fires only when -upstream is set; -record-to defaults
	// to <root>/recorded.yaml. Pass -record-to "off" (or any sentinel
	// the operator chooses) to disable capture without disabling the
	// proxy itself — we don't special-case that today; an explicit
	// flag of "" (the zero value via -record-to "") would disable.
	var rec *recorder.Recorder
	if *flagUpstream != "" {
		recordPath := *flagRecordTo
		if recordPath == "" {
			recordPath = filepath.Join(*flagRoot, "recorded.json")
		}
		if recordPath != "off" {
			rec = recorder.New(recordPath, logger)
			defer func() { _ = rec.Close() }()
		}
	}

	// Request log — JSONL of every request/response that flows
	// through. Different file from recorded.json (which is the
	// deduplicated mock-build artifact). Rotated by lumberjack so a
	// long-running dev session doesn't fill the disk; sensitive
	// headers redacted by default.
	var reqLog *requestlog.Logger
	if *flagLogReq != "" {
		logPath := *flagLogReq
		if logPath == "auto" {
			logPath = filepath.Join(*flagRoot, "requests.jsonl")
		}
		var redact []string
		switch *flagLogRedact {
		case "":
			redact = nil // → DefaultRedactHeaders
		case "none":
			redact = []string{} // empty slice = no redaction (operator opt-in)
		default:
			redact = strings.Split(*flagLogRedact, ",")
			for i := range redact {
				redact[i] = strings.TrimSpace(redact[i])
			}
		}
		reqLog, err = requestlog.Open(requestlog.Options{
			Path:          logPath,
			MaxSizeMB:     *flagLogMaxSize,
			MaxBackups:    *flagLogMaxKeep,
			RedactHeaders: redact,
		}, logger)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tavora-fake-backend: %v\n", err)
			return 1
		}
		defer func() { _ = reqLog.Close() }()
	}

	serverOpts := server.Options{
		Logger:          logger,
		Store:           st,
		Routes:          customRoutes,
		ExpectedHeaders: expected,
		Scenarios:       scenReg,
		Upstream:        *flagUpstream,
		RequestLog:      reqLog,
		CORS:            *flagCORS,
	}
	if rec != nil {
		serverOpts.Record = rec.Func()
	}
	handler, err := server.New(serverOpts)
	if err != nil {
		logger.Error("server.New failed", "err", err)
		return 1
	}

	srv := &http.Server{
		Addr:              *flagAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	if *flagWatch {
		watchCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			err := watch.Run(watchCtx, logger, []string{dbPath}, func() error {
				if err := st.Reload(); err != nil {
					return err
				}
				logger.Info("db.json reloaded", "resources", st.Resources())
				return nil
			})
			if err != nil {
				logger.Warn("watcher exited", "err", err)
			}
		}()
	}

	errCh := make(chan error, 1)
	go func() {
		recordTo := ""
		if rec != nil {
			recordTo = *flagRecordTo
			if recordTo == "" {
				recordTo = filepath.Join(*flagRoot, "recorded.json")
			}
		}
		logRequests := ""
		if reqLog != nil {
			logRequests = reqLog.Path()
		}
		logger.Info("tavora-fake-backend listening",
			"addr", srv.Addr,
			"root", *flagRoot,
			"db", dbPath,
			"routes", len(customRoutes),
			"expected_headers", len(expected),
			"scenarios", scenReg.Names(),
			"active_scenario", scenReg.ActiveName(),
			"upstream", *flagUpstream,
			"record_to", recordTo,
			"log_requests", logRequests,
			"persist", *flagPersist,
			"cors", *flagCORS,
			"resources", st.Resources(),
		)
		logger.Info("clients must opt in to talk to a fake backend",
			"env", "TAVORA_ALLOW_FAKE=1",
			"why", "every response carries X-Tavora-Fake-Backend: true; the sandbox refuses without the opt-in",
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
	logger.Info("tavora-fake-backend stopped")
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
