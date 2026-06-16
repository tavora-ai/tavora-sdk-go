package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	tavora "github.com/tavora-ai/tavora-sdk-go"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/codefirst/scaffold"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/codefirst/source"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/codefirst/validate"
)

// Code-first verbs. The implementation notes in
// tavora-go/docs/code-first-agents-concept.md drive the contract;
// see the "CLI UX" section there.

// --- tavora init ---

var (
	initProjectSlug string // --project — matches the cloud-side Project slug
	initAPIURL      string
	initForce       bool
	initDir         string
	initDryRun      bool
	initNoFooter    bool // hidden: suppress "Next steps:" — used by create-tavora wrapper
)

var codefirstInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold a tavora/ project folder (manifest, agent, persona, skill, eval, .gitignore)",
	Long: `tavora init creates a tavora/ folder under the current directory
containing a working starter project: tavora.jsonc, one agent
(agents/support/) with persona, skills, and an eval case, plus a
.gitignore that hides .runs/.

This is the entry point for the code-first authoring path. Edit the
files, then run tavora dev to sync a dev draft and tavora deploy to
cut an immutable published version.

Existing files are preserved unless --force is set.`,
	Example: `  tavora init
  tavora init --project acme-support
  tavora init --dir ./vendor/tavora --force`,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := runInitFlow(initDir, initProjectSlug, initForce, initDryRun)
		if err != nil {
			return err
		}
		if initDryRun || root == "" || initNoFooter {
			return nil
		}
		fmt.Println()
		fmt.Println("Next steps:")
		fmt.Printf("  cd %s\n", root)
		fmt.Println("  tavora dev          # validate, watch, sync a dev draft")
		fmt.Println("  tavora deploy       # promote the draft to a published version")
		return nil
	},
}

// runInitFlow is the shared scaffold+bind path for both `tavora init`
// and `tavora dev`'s offer-to-init fallback. Returns the absolute
// root of the scaffolded folder, or "" when --dry-run printed the
// plan and didn't write anything. Caller decides whether to print
// the "Next steps:" footer.
func runInitFlow(rootHint, appSlug string, force, dryRun bool) (string, error) {
	root := rootHint
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		root = filepath.Join(cwd, "tavora")
	}
	project := appSlug
	if project == "" {
		// If the user is already logged in (api key configured), the
		// truthful default is the slug of the project that key is bound
		// to — that's where source-sync will actually route. Falling
		// back to the directory name was a pre-deployments choice
		// and confuses users who init inside `<repo>/backend-go/`.
		project = tryGetProjectSlug()
	}
	if project == "" {
		project = filepath.Base(filepath.Dir(root))
		if project == "." || project == "/" || project == "" {
			project = "tavora-project"
		}
	}
	opt := scaffold.Options{
		Root:        root,
		ProjectName: project,
		APIURL:      initAPIURL,
		Force:       force,
	}
	if dryRun {
		for _, f := range scaffold.Plan(opt) {
			status("would write %s", filepath.Join(root, f.RelPath))
		}
		return "", nil
	}
	written, err := scaffold.Write(opt)
	if err != nil {
		return "", err
	}
	if len(written) == 0 {
		status("no files written (already present — pass --force to overwrite)")
		return root, nil
	}
	for _, p := range written {
		status("wrote %s", p)
	}

	// Best-effort cloud bind: mint (or reuse) the user's dev
	// deployment and write its slug to <root>/.env.local. Failures
	// aren't fatal — `tavora dev` will fall back to the server's
	// resolver auto-create on first sync.
	if err := bindCloudDeployment(root); err != nil {
		status("cloud bind skipped: %v", err)
		status("run `tavora login` then `tavora dev` to bind on first sync.")
	} else {
		status("wrote %s", filepath.Join(root, ".env.local"))
	}
	return root, nil
}

// hasEnvLocal reports whether the project root already carries a
// deployment binding (tavora/.env.local). It's a cheap stat — we
// treat any read error as "missing" since this gates an interactive
// prompt, not security.
func hasEnvLocal(root string) bool {
	if root == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(root, ".env.local")); err == nil {
		return true
	}
	return false
}

// --- tavora bind ---

var bindDir string

var codefirstBindCmd = &cobra.Command{
	Use:   "bind",
	Short: "Bind an existing tavora/ folder to a dev deployment (writes .env.local)",
	Long: `tavora bind mints (or reuses) the API key's dev deployment for
this project and writes the slug to <root>/.env.local so subsequent
CLI invocations (sync, env put, deploy) attach the right
X-Tavora-Deployment header.

Use this when you cloned a repo that already ships a tavora/ folder —
init would re-scaffold agent files, bind only writes .env.local.

Requires an API key (run ` + "`tavora login`" + ` first).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := loadProjectOrFail(bindDir)
		if err != nil {
			return err
		}
		if hasEnvLocal(p.Root) {
			status(".env.local already exists in %s — nothing to do", p.Root)
			return nil
		}
		if client == nil {
			return fmt.Errorf("no API key configured — run `tavora login` first")
		}
		if err := bindCloudDeployment(p.Root); err != nil {
			return fmt.Errorf("bind failed: %w", err)
		}
		status("wrote %s", filepath.Join(p.Root, ".env.local"))
		return nil
	},
}

// --- tavora dev ---

var (
	devDir     string
	devOnce    bool
	devNoSync  bool
	devVerbose bool
	devNoInit  bool
)

var codefirstDevCmd = &cobra.Command{
	Use:   "dev",
	Short: "Watch tavora/, validate on every change, and sync a dev draft",
	Long: `tavora dev is the inner-loop command. It watches the tavora/
folder, debounces file changes, validates every revision, and syncs
a mutable dev draft to your account so playground invocations and
SDK calls targeting the draft pick up the new behavior.

Pass --once to do a single validate + sync (useful for CI). Pass
--no-sync to validate locally without touching the server.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := loadProjectOrFail(devDir)
		if err != nil {
			// If the failure is "no tavora.jsonc found anywhere", offer
			// to scaffold one in-process so the user doesn't have to
			// quit, run init, and re-run dev. Skipped in CI / non-TTY /
			// when --no-init is set, so scripts get the clean failure.
			if !isMissingTavoraFolderErr(err) || devNoInit || !isInteractive() {
				return err
			}
			fmt.Fprintf(os.Stderr, "No tavora/ folder found at or above this directory.\n")
			if !promptYesNo("Scaffold one here with `tavora init`?") {
				return fmt.Errorf("aborted; run `tavora init` first (or pass --no-init to skip this prompt)")
			}
			root, initErr := runInitFlow("", "", false, false)
			if initErr != nil {
				return initErr
			}
			if devDir == "" {
				devDir = root
			}
			fmt.Println()
			p, err = loadProjectOrFail(devDir)
			if err != nil {
				return err
			}
		}
		// Stamp the project root as the sync source for this
		// process. The transport reads CurrentSyncSource on every
		// outgoing request and sets X-Tavora-Source, letting the
		// server detect "two tavora/ folders authoring the same
		// agent" and warn back.
		CurrentSyncSource = p.Root

		// Folder exists but unbound? Offer to mint a dev deployment
		// and write .env.local. We deliberately call bindCloudDeployment
		// directly here — never runInitFlow — because the scaffold
		// step would touch files in an existing project. Skipped in
		// CI / non-TTY / when --no-init is set / when no API key is
		// configured (bindCloudDeployment would just fail). On bind
		// failure we surface a warning and continue; the server-side
		// resolver still auto-creates a deployment for sync.
		if !devNoSync && client != nil && isInteractive() && !devNoInit && !hasEnvLocal(p.Root) {
			fmt.Fprintf(os.Stderr, "This tavora/ folder isn't bound to a deployment (no .env.local).\n")
			if promptYesNo("Bind it to a dev deployment now?") {
				if err := bindCloudDeployment(p.Root); err != nil {
					status("bind skipped: %v", err)
				} else {
					status("wrote %s", filepath.Join(p.Root, ".env.local"))
					// Refresh the SDK client so its transport picks up
					// the freshly-written X-Tavora-Deployment header on
					// every request from here on. httpClientForDeployment
					// reads loadDeploymentSlug() at construction time, so
					// re-build it.
					if url, key := resolveAPIConfig(); key != "" {
						client = tavora.NewClient(url, key, tavora.WithHTTPClient(httpClientForDeployment()))
					}
				}
				fmt.Println()
			}
		}

		// Warn loudly when the user invoked `tavora dev` (no --no-sync)
		// but no API key is configured — otherwise the "sync skipped"
		// status looks identical to --no-sync and they wonder why the
		// server never sees their agents.
		if !devNoSync && client == nil {
			status("warning: no API key configured — running in local-only mode (run `tavora login` or set TAVORA_API_KEY to enable sync)")
		}
		effectiveNoSync := devNoSync || client == nil

		// Tier 3: ensure the project's dev environment exists so synced
		// drafts have a home to resolve against. Best-effort — a failure
		// (offline, legacy backend without the route) shouldn't block the
		// local watch loop; the sync resolver still handles drafts.
		if !effectiveNoSync {
			if _, err := client.EnsureDevDeployment(globalCtx(), p.Manifest.Project); err != nil && devVerbose {
				status("dev environment ensure skipped: %v", err)
			}
		}

		// Mock backends — auto-launch one fakeback HTTP listener per
		// <project>/mocks/<name>/ folder. Skill authors point
		// context.backend_url at the printed URLs to iterate against
		// the fake instead of a real backend. Noop for projects
		// without a mocks/ folder. --once skips this — the mocks would
		// only live for the single validate pass anyway.
		var stopMocks func()
		if !devOnce {
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()
			mocks, shutdown, mockErr := startMockBackends(ctx, p.Root, devSlog())
			if mockErr != nil {
				status("mock backends not started: %v", mockErr)
			} else {
				printMocks(mocks)
				stopMocks = shutdown
				defer stopMocks()
			}
		}

		// In watch mode we deliberately swallow the validate error
		// from the first pass so the user can keep editing toward
		// green. --once exits with a non-zero code so CI / scripts
		// still see the failure.
		firstErr := runValidateAndSync(p, effectiveNoSync, devVerbose)
		if devOnce {
			return firstErr
		}
		if firstErr != nil {
			status("%v — keep editing; the watcher will retry on every save", firstErr)
		}
		return watchAndSync(p, effectiveNoSync, devVerbose)
	},
}

// devSlog returns the slog logger handed to mock backends. Discard
// by default so mock noise doesn't crowd out tavora dev's own status
// lines; flip to stdout when devVerbose is on.
func devSlog() *slog.Logger {
	if devVerbose {
		return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// --- tavora deploy ---

var (
	deployDir    string
	deployDryRun bool
	deployEnv    string
)

var codefirstDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Cut an immutable project release from the current dev drafts",
	Long: `tavora deploy validates the local tavora/ folder, syncs fresh
drafts, then cuts a release.

With --env staging it cuts a Tier 3 project release into staging: each
agent is diffed by source hash, so an unchanged agent keeps its pinned
version and only changed agents get a new one. The staging pin set
({agent → version}) is updated atomically. Production is promote-only —
you reach it by promoting the staging pins, so what ships is exactly what
you validated.

  tavora deploy --env staging   # cut + pin into staging
  tavora promote --to prod      # ship the staging pin set to prod
  tavora ship                   # do both in one step

Without --env it cuts the legacy project-wide release and repoints your
dev environment at it.

--dry-run skips the deploy step; useful as a standalone validate.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := loadProjectOrFail(deployDir)
		if err != nil {
			return err
		}
		issues := validate.Project(p, loadKnownCapabilities())
		printIssues(p, issues)
		if validate.HasFatal(issues) {
			return fmt.Errorf("deploy refused: %d fatal validation issue(s)", validate.CountFatal(issues))
		}
		manifest := buildManifest(p)
		if deployDryRun {
			status("dry-run: would deploy manifest with %d agent(s), source hash %s", len(manifest.Agents), short(manifest.SourceHash))
			return nil
		}
		if client == nil {
			return fmt.Errorf("no API key configured — run 'tavora login' first or use --dry-run")
		}

		// See dev RunE for the rationale; deploy pre-syncs then
		// promotes, so it also wants source-flip detection.
		CurrentSyncSource = p.Root

		sdkManifest := toSDKManifest(manifest, p)
		if _, err := client.SourceSync(globalCtx(), sdkManifest); err != nil {
			// Only suggest "endpoint not exposed" on a real 404 —
			// other statuses already carry an actionable message
			// from the server.
			var apiErr *tavora.APIError
			if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
				return fmt.Errorf("pre-deploy sync failed: %w\n  hint: backend must accept /api/sdk/source-sync before deploy can run", err)
			}
			return fmt.Errorf("pre-deploy sync failed: %w", err)
		}
		// Tier 3 path: `--env staging` cuts a project release into
		// staging, pinning one version per agent (unchanged agents keep
		// their version, changed agents get a new one). Production is
		// promote-only — `--env prod` is rejected with a pointer to
		// `tavora promote --to prod`, so what ships is exactly what was
		// validated in staging.
		if env := strings.TrimSpace(deployEnv); env != "" {
			if env == "prod" {
				return fmt.Errorf("production is promote-only — cut to staging then ship it:\n  tavora deploy --env staging\n  tavora promote --to prod   (or `tavora ship` to do both)")
			}
			if env != "staging" {
				return fmt.Errorf("--env must be \"staging\" (got %q); production is reached via `tavora promote --to prod`", deployEnv)
			}
			rel, err := client.CutRelease(globalCtx(), p.Manifest.Project)
			if err != nil {
				return err
			}
			if isJSON() {
				return printJSON(rel)
			}
			for _, pin := range rel.Pins {
				status("  • %s → v%d", pin.AgentID, pin.VersionNumber)
			}
			status("cut staging release: %d agent(s) pinned in %s", len(rel.Pins), rel.Deployment.Slug)
			status("ship it: `tavora promote --to prod`  (or `tavora ship`)")
			return nil
		}

		input := tavora.SourceDeployInput{
			Project: p.Manifest.Project,
		}
		out, err := client.SourceDeploy(globalCtx(), input)
		if err != nil {
			return err
		}
		if isJSON() {
			return printJSON(out)
		}
		for _, a := range out.Agents {
			status("  • %s → %s (%s)", a.LocalID, a.AgentID, short(a.SourceHash))
		}
		status("cut release %d spanning %d agent(s)", out.ReleaseNumber, len(out.Agents))
		status("your dev environment now runs release %d — `tavora promote --to staging` (or prod) to ship it", out.ReleaseNumber)
		return nil
	},
}

// --- tavora config show ---

var configShowCmd = &cobra.Command{
	Use:   "show [agent]",
	Short: "Print an agent's resolved config (post env-var substitution, post-merge)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := loadProjectOrFail("")
		if err != nil {
			return err
		}
		var target *source.Agent
		if len(args) == 1 {
			for _, a := range p.Agents {
				if a.Config.ID == args[0] {
					target = a
					break
				}
			}
			if target == nil {
				return fmt.Errorf("agent %q not found in project (have: %s)", args[0], agentIDs(p))
			}
			return printResolvedAgent(target)
		}
		// No arg: print every agent
		for _, a := range p.Agents {
			fmt.Printf("# agent: %s (%s)\n", a.Config.ID, a.Config.Name)
			if err := printResolvedAgent(a); err != nil {
				return err
			}
			fmt.Println()
		}
		return nil
	},
}

// configCmd is the parent for `tavora config show` etc. Other
// subcommands (set / get / unset) can land here later.
var codefirstConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Inspect resolved project configuration",
}

// --- shared helpers ---

func loadProjectOrFail(dir string) (*source.Project, error) {
	if dir == "" {
		cwd, _ := os.Getwd()
		dir = cwd
	}
	p, err := source.Load(dir)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// isMissingTavoraFolderErr reports whether the error from source.Load
// is the "walked up to / and never found tavora.jsonc" case — the
// only error worth offering to scaffold around in dev. String-matched
// today because source.Load returns plain fmt.Errorf strings; if the
// source package ever surfaces typed errors, swap in errors.Is.
func isMissingTavoraFolderErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "no tavora.jsonc found")
}

// isInteractive reports whether the CLI can prompt the user. False
// when stdin isn't a TTY (piped input, CI runners) or when CI=true
// is set explicitly so scripts get clean failures instead of hanging
// on a read.
func isInteractive() bool {
	if os.Getenv("CI") != "" {
		return false
	}
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// promptYesNo asks a Y/n question on stdin/stderr and returns true
// for empty input, "y", or "yes" (case-insensitive). Default is Y;
// the prompt prints "[Y/n]" so the user can confirm with Enter.
func promptYesNo(question string) bool {
	fmt.Fprintf(os.Stderr, "%s [Y/n]: ", question)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "" || line == "y" || line == "yes"
}

func runValidateAndSync(p *source.Project, noSync bool, verbose bool) error {
	issues := validate.Project(p, loadKnownCapabilities())
	printIssues(p, issues)
	if validate.HasFatal(issues) {
		return fmt.Errorf("validation failed: %d fatal issue(s)", validate.CountFatal(issues))
	}
	manifest := buildManifest(p)
	if noSync {
		status("validation OK (%d agent(s), source %s, sync skipped)", len(p.Agents), manifest.SourceHash[7:19])
		return nil
	}

	sdkManifest := toSDKManifest(manifest, p)
	result, err := client.SourceSync(globalCtx(), sdkManifest)
	if err != nil {
		// 422 carries the server's validation issues in the response
		// body; surface them inline so the user sees what to fix.
		if issues, ok := extractServerIssues(err); ok {
			for _, i := range issues {
				fmt.Fprintf(os.Stderr, "[%s] %s\n  %s\n", padSeverity(i.Severity), serverIssueLocation(i), i.Message)
				if i.Hint != "" {
					fmt.Fprintf(os.Stderr, "  hint: %s\n", i.Hint)
				}
			}
			return fmt.Errorf("source-sync rejected: %d server validation issue(s)", len(issues))
		}
		// The SDK wraps HTTP errors as *tavora.APIError with the
		// server's `message` field already exposed. Trust that text
		// — `%w` renders it as "tavora: <message> (status <code>)".
		// Pin the legacy "endpoint may not exist yet" hint to 404 so
		// a 500/503 doesn't send the operator chasing a missing
		// route when the real cause is in the server's body.
		var apiErr *tavora.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return fmt.Errorf("source-sync failed: %w\n  hint: backend may not expose /api/sdk/source-sync — use --no-sync to validate locally", err)
		}
		return fmt.Errorf("source-sync failed: %w", err)
	}
	for _, i := range result.ServerIssues {
		fmt.Fprintf(os.Stderr, "[%s] %s\n  %s\n", padSeverity(i.Severity), serverIssueLocation(i), i.Message)
		if i.Hint != "" {
			fmt.Fprintf(os.Stderr, "  hint: %s\n", i.Hint)
		}
	}
	status("synced: draft %s, %d agent(s)", short(result.DraftHash), len(result.Agents))
	if verbose {
		out, _ := source.PrettyJSON(result)
		fmt.Println(string(out))
	}
	return nil
}

func short(hash string) string {
	if len(hash) < 19 {
		return hash
	}
	return hash[7:19]
}

// extractServerIssues pulls the server's `issues` array out of a 422
// response. The SDK exposes the raw body fields via APIError.Details;
// SourceSyncHandler returns {"issues": [...]} on validation failure
// with shape {file,line,column,severity,code,message,hint}. We
// JSON-roundtrip the Details["issues"] value into the SDK's typed
// SourceValidationIssue so the caller can format it the same way as
// successful-sync issues.
func extractServerIssues(err error) ([]tavora.SourceValidationIssue, bool) {
	var apiErr *tavora.APIError
	if !errors.As(err, &apiErr) {
		return nil, false
	}
	raw, ok := apiErr.Details["issues"]
	if !ok {
		return nil, false
	}
	buf, mErr := json.Marshal(raw)
	if mErr != nil {
		return nil, false
	}
	var issues []tavora.SourceValidationIssue
	if uErr := json.Unmarshal(buf, &issues); uErr != nil {
		return nil, false
	}
	return issues, len(issues) > 0
}

func padSeverity(s string) string {
	switch s {
	case "fatal":
		return "fatal"
	case "warn":
		return " warn"
	default:
		return s
	}
}

func serverIssueLocation(i tavora.SourceValidationIssue) string {
	if i.Line > 0 {
		if i.Column > 0 {
			return fmt.Sprintf("%s:%d:%d (server)", i.File, i.Line, i.Column)
		}
		return fmt.Sprintf("%s:%d (server)", i.File, i.Line)
	}
	if i.File != "" {
		return fmt.Sprintf("%s (server)", i.File)
	}
	return "(server) " + i.Code
}

func agentIDs(p *source.Project) string {
	ids := make([]string, 0, len(p.Agents))
	for _, a := range p.Agents {
		ids = append(ids, a.Config.ID)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return "(none)"
	}
	return strings.Join(ids, ", ")
}

func printResolvedAgent(a *source.Agent) error {
	resolved := resolveForDisplay(a)
	out, err := source.PrettyJSON(resolved)
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}

// resolvedAgent is the shape we emit from `tavora config show`. It
// includes the parsed config plus the resolved skill paths and
// kinds so an AI tool can confirm "yes, my new skill is bound".
type resolvedAgent struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Model        source.ModelRef `json:"model"`
	Capabilities []string        `json:"capabilities,omitempty"`
	Persona      string          `json:"persona,omitempty"`
	Skills       []resolvedSkill `json:"skills"`
	Indexes      []string        `json:"indexes,omitempty"`
	Evals        []string        `json:"evals,omitempty"`
}

type resolvedSkill struct {
	Kind    string `json:"kind"`
	Path    string `json:"path"`
	Binding string `json:"binding"`
}

func resolveForDisplay(a *source.Agent) resolvedAgent {
	skills := make([]resolvedSkill, 0, len(a.Skills))
	for _, s := range a.Skills {
		skills = append(skills, resolvedSkill{
			Kind:    string(s.Kind),
			Path:    s.RelPath,
			Binding: s.BindingRaw,
		})
	}
	evals := make([]string, 0, len(a.Evals))
	for _, e := range a.Evals {
		evals = append(evals, e.RelPath)
	}
	out := resolvedAgent{
		ID:           a.Config.ID,
		Name:         a.Config.Name,
		Model:        a.Config.Model,
		Capabilities: a.Config.Capabilities,
		Persona:      a.Persona,
		Skills:       skills,
		Indexes:      a.Config.Indexes,
		Evals:        evals,
	}
	return out
}

// --- manifest ---

// SyncManifest is the payload tavora dev / tavora deploy send to the
// server. It's content-addressed: hash each file, hash each agent,
// hash the whole project. The server can ask for missing blobs once
// the backend grows that capability.
type SyncManifest struct {
	Project     string          `json:"project"`
	Environment string          `json:"environment,omitempty"`
	SourceHash  string          `json:"sourceHash"`
	Agents      []ManifestAgent `json:"agents"`
	GeneratedAt time.Time       `json:"generatedAt"`
}

type ManifestAgent struct {
	ID         string         `json:"id"`
	SourceHash string         `json:"sourceHash"`
	Files      []ManifestFile `json:"files"`
}

type ManifestFile struct {
	Path    string `json:"path"`
	Hash    string `json:"hash"`
	Size    int    `json:"size"`
	Content string `json:"content,omitempty"`
}

// globalCtx returns the rooted context the CLI runs under. We
// rebuild it lazily so the long-running dev loop doesn't share a
// single ctx between cycles.
func globalCtx() context.Context {
	return context.Background()
}

// toSDKManifest converts the CLI's local SyncManifest into the
// shape the SDK sends over the wire. Content bytes are included
// here — when the backend grows content-addressed dedupe it can
// reject duplicates and the CLI can drop the bytes.
func toSDKManifest(local SyncManifest, p *source.Project) tavora.SourceSyncManifest {
	out := tavora.SourceSyncManifest{
		Project:     local.Project,
		SourceHash:  local.SourceHash,
		GeneratedAt: local.GeneratedAt,
	}
	for _, a := range local.Agents {
		// Look up source bytes from the project tree so the manifest
		// is self-contained.
		var bytesByPath map[string][]byte
		for _, pa := range p.Agents {
			if pa.Config.ID == a.ID {
				bytesByPath = pa.SourceBytes
				break
			}
		}
		var files []tavora.SourceFile
		for _, f := range a.Files {
			files = append(files, tavora.SourceFile{
				Path:    f.Path,
				Hash:    f.Hash,
				Size:    f.Size,
				Content: bytesByPath[f.Path],
			})
		}
		out.Agents = append(out.Agents, tavora.SourceAgent{
			ID:         a.ID,
			SourceHash: a.SourceHash,
			Files:      files,
		})
	}
	return out
}

func buildManifest(p *source.Project) SyncManifest {
	m := SyncManifest{
		Project:     p.Manifest.Project,
		GeneratedAt: time.Now().UTC(),
	}
	projectHasher := sha256.New()
	for _, a := range p.Agents {
		agentHasher := sha256.New()
		var files []ManifestFile
		paths := make([]string, 0, len(a.SourceBytes))
		for k := range a.SourceBytes {
			paths = append(paths, k)
		}
		sort.Strings(paths)
		for _, k := range paths {
			b := a.SourceBytes[k]
			h := sha256.Sum256(b)
			hashHex := "sha256:" + hex.EncodeToString(h[:])
			files = append(files, ManifestFile{
				Path: k,
				Hash: hashHex,
				Size: len(b),
				// Content omitted from the manifest by default —
				// the backend resolves on demand once content-addressed
				// upload lands. Local-only printing fills it in below.
			})
			agentHasher.Write([]byte(k))
			agentHasher.Write(b)
		}
		agentHash := "sha256:" + hex.EncodeToString(agentHasher.Sum(nil))
		m.Agents = append(m.Agents, ManifestAgent{
			ID:         a.Config.ID,
			SourceHash: agentHash,
			Files:      files,
		})
		projectHasher.Write([]byte(a.Config.ID))
		projectHasher.Write([]byte(agentHash))
	}
	m.SourceHash = "sha256:" + hex.EncodeToString(projectHasher.Sum(nil))
	return m
}

func init() {
	codefirstInitCmd.Flags().StringVar(&initProjectSlug, "project", "", "Cloud Project slug to bind this folder to (written into tavora.jsonc as the project field)")
	codefirstInitCmd.Flags().StringVar(&initAPIURL, "api-url", "", "API URL written into tavora.jsonc (default: omit; CLI uses ~/.tavora.yaml)")
	codefirstInitCmd.Flags().BoolVar(&initForce, "force", false, "Overwrite existing files")
	codefirstInitCmd.Flags().StringVar(&initDir, "dir", "", "Directory to scaffold (default: ./tavora)")
	codefirstInitCmd.Flags().BoolVar(&initDryRun, "dry-run", false, "Print the file list without writing")
	codefirstInitCmd.Flags().BoolVar(&initNoFooter, "no-footer", false, "Suppress the 'Next steps:' footer (for wrappers like create-tavora)")
	_ = codefirstInitCmd.Flags().MarkHidden("no-footer")

	codefirstDevCmd.Flags().StringVar(&devDir, "dir", "", "Project directory containing tavora.jsonc (default: search up from cwd)")
	codefirstDevCmd.Flags().BoolVar(&devOnce, "once", false, "Validate + sync a single time and exit")
	codefirstDevCmd.Flags().BoolVar(&devNoSync, "no-sync", false, "Validate locally only — do not contact the server")
	codefirstDevCmd.Flags().BoolVarP(&devVerbose, "verbose", "v", false, "Print extra detail on every cycle")
	codefirstDevCmd.Flags().BoolVar(&devNoInit, "no-init", false, "Don't offer to run tavora init when no tavora/ folder is found (default: prompt interactively)")

	codefirstBindCmd.Flags().StringVar(&bindDir, "dir", "", "Project directory containing tavora.jsonc (default: search up from cwd)")

	codefirstDeployCmd.Flags().StringVar(&deployDir, "dir", "", "Project directory containing tavora.jsonc")
	codefirstDeployCmd.Flags().BoolVar(&deployDryRun, "dry-run", false, "Validate and build the manifest, but skip the deploy call")
	codefirstDeployCmd.Flags().StringVar(&deployEnv, "env", "", "Cut a release into staging (Tier 3 pin model). Production is promote-only: `tavora promote --to prod`.")

	codefirstConfigCmd.AddCommand(configShowCmd)

	rootCmd.AddCommand(codefirstInitCmd)
	rootCmd.AddCommand(codefirstBindCmd)
	rootCmd.AddCommand(codefirstDevCmd)
	rootCmd.AddCommand(codefirstDeployCmd)
	rootCmd.AddCommand(codefirstConfigCmd)
}
