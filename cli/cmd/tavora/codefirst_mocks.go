package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/fetchpolicy"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/requestlog"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/routes"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/scenarios"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/server"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/store"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/watch"
)

// mockBasePort is the first port the dev runner hands to a mock
// backend. Subsequent mocks get +1 each. The range stays clear of
// `tavora-go` (:50000) and `tavora-mock` (:50100).
const mockBasePort = 50201

// MockBackend describes one running fake-backend process bound to a
// `<project>/mocks/<name>/` folder.
type MockBackend struct {
	Name   string // folder name
	URL    string // e.g. http://127.0.0.1:50201
	Folder string // absolute path
}

// startMockBackends scans <projectRoot>/mocks/* for subfolders and
// boots one in-process fakeback HTTP listener per folder. Returns
// the list of running mocks (with the URLs to plug into
// `context('backend_url')`) and a shutdown function the caller
// invokes on dev exit.
//
// If <projectRoot>/mocks does not exist, returns (nil, noop, nil) —
// projects without mocks pay nothing.
func startMockBackends(ctx context.Context, projectRoot string, logger *slog.Logger) ([]MockBackend, func(), error) {
	mocksDir := filepath.Join(projectRoot, "mocks")
	entries, err := os.ReadDir(mocksDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, func() {}, nil
		}
		return nil, nil, fmt.Errorf("read %s: %w", mocksDir, err)
	}

	folders := []string{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		folders = append(folders, e.Name())
	}
	sort.Strings(folders) // stable port assignment

	if len(folders) == 0 {
		return nil, func() {}, nil
	}

	var mocks []MockBackend
	var servers []*http.Server
	mockCtx, cancel := context.WithCancel(ctx)

	for i, name := range folders {
		folder := filepath.Join(mocksDir, name)
		port := mockBasePort + i
		addr := fmt.Sprintf("127.0.0.1:%d", port)

		srv, err := buildMockServer(mockCtx, folder, addr, logger.With("mock", name))
		if err != nil {
			cancel()
			for _, s := range servers {
				_ = s.Shutdown(context.Background())
			}
			return nil, nil, fmt.Errorf("mock %s: %w", name, err)
		}

		go func(name string, srv *http.Server) {
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Warn("mock listener exited", "name", name, "err", err)
			}
		}(name, srv)
		servers = append(servers, srv)
		mocks = append(mocks, MockBackend{
			Name:   name,
			URL:    "http://" + addr,
			Folder: folder,
		})
	}

	shutdown := func() {
		cancel()
		for _, srv := range servers {
			sctx, c := context.WithTimeout(context.Background(), 2*time.Second)
			_ = srv.Shutdown(sctx)
			c()
		}
	}
	return mocks, shutdown, nil
}

// buildMockServer assembles one fakeback http.Server bound to addr.
// Reads db.json + routes.yaml + expected-headers.yaml + scenarios/
// from folder; sets up the fsnotify hot-reload loop tied to mockCtx.
func buildMockServer(mockCtx context.Context, folder, addr string, logger *slog.Logger) (*http.Server, error) {
	dbPath := filepath.Join(folder, "db.json")
	st, err := store.Open(store.Config{
		File:       dbPath,
		IDStrategy: "int",
	}, logger)
	if err != nil {
		return nil, fmt.Errorf("store.Open %s: %w", dbPath, err)
	}

	customRoutes, err := routes.LoadFolderRoutes(folder)
	if err != nil {
		return nil, err
	}
	expected, err := fetchpolicy.LoadFile(filepath.Join(folder, "expected-headers.yaml"))
	if err != nil {
		return nil, err
	}
	scenReg, err := scenarios.LoadDir(filepath.Join(folder, "scenarios"))
	if err != nil {
		return nil, err
	}

	// requests.jsonl always-on under `tavora dev` so the skill
	// author (and any AI coding tool watching them) sees what the
	// agent's outbound calls actually look like during iteration.
	// 50MB / 3 backups is generous — a long dev session bursting
	// fetches every second still has ~14 hours of headroom before
	// rotation; sensitive headers redacted by default.
	// Errors here are non-fatal — the mock should still boot if the
	// log file can't be opened.
	reqLogPath := filepath.Join(folder, "requests.jsonl")
	reqLog, logErr := requestlog.Open(requestlog.Options{
		Path:       reqLogPath,
		MaxSizeMB:  50,
		MaxBackups: 3,
	}, logger)
	if logErr != nil {
		logger.Warn("requestlog open failed; mock continues without request log",
			"path", reqLogPath, "err", logErr)
	}

	handler, err := server.New(server.Options{
		Logger:          logger,
		Store:           st,
		Routes:          customRoutes,
		ExpectedHeaders: expected,
		Scenarios:       scenReg,
		RequestLog:      reqLog,
		CORS:            true,
	})
	if err != nil {
		if reqLog != nil {
			_ = reqLog.Close()
		}
		return nil, err
	}
	// Tie the requestlog's lifetime to the dev-loop context so its
	// file handle closes on tavora dev exit (matches the watch loop).
	if reqLog != nil {
		go func() {
			<-mockCtx.Done()
			_ = reqLog.Close()
		}()
	}

	// Hot reload tied to the dev-loop context.
	go func() {
		err := watch.Run(mockCtx, logger, []string{dbPath}, func() error {
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

	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}, nil
}

// printMocks emits a human-readable summary of the running mocks for
// the dev banner. Skill authors copy the URL into
// `context.backend_url` in their tavora.json or session options.
func printMocks(mocks []MockBackend) {
	if len(mocks) == 0 {
		return
	}
	fmt.Println()
	status("mock backends running:")
	for _, m := range mocks {
		fmt.Printf("    %-20s %s\n", m.Name, m.URL)
		fmt.Printf("    %-20s tail -f %s\n", "  ↳ request log", filepath.Join(m.Folder, "requests.jsonl"))
	}
	fmt.Printf("    %-20s %s\n", "(opt-in)", "TAVORA_ALLOW_FAKE=1")
	status("set context.backend_url in tavora.json to the matching URL, or pass it via session_vars at session creation")
	fmt.Println()
}
