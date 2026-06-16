package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	tavora "github.com/tavora-ai/tavora-sdk-go"
)

// Tier 3 promote + status over the project-environment pin model.
// `tavora deploy --env staging` cuts a release into staging; `tavora
// promote --to prod` copies staging's pin set into prod; `tavora status`
// shows the pinned version per agent in each environment.

var (
	promoteTo      string
	promoteProject string
	statusProject  string
)

// resolveProjectName returns the explicit --project flag when set,
// otherwise the project declared in the local tavora/ folder's manifest.
func resolveProjectName(flag string) (string, error) {
	if p := strings.TrimSpace(flag); p != "" {
		return p, nil
	}
	proj, err := loadProjectOrFail("")
	if err != nil {
		return "", fmt.Errorf("%w\n  (or pass --project <name> to run outside a tavora/ folder)", err)
	}
	return proj.Manifest.Project, nil
}

// findEnvSlug returns the slug of the project's environment of the given
// kind, or "" if none exists yet.
func findEnvSlug(deps []tavora.Deployment, kind string) string {
	for _, d := range deps {
		if d.Kind == kind {
			return d.Slug
		}
	}
	return ""
}

var promoteCmd = &cobra.Command{
	Use:   "promote --to staging|prod",
	Short: "Promote a project release into staging or production",
	Long: `tavora promote ships a project's pinned versions between
environments (the Tier 3 pin model — a release is the {agent → version}
snapshot an environment holds).

  tavora promote --to staging   # cut + pin the current drafts into staging
  tavora promote --to prod      # copy the staging pin set into production

--to staging cuts a fresh staging release (same as tavora deploy --env
staging). --to prod copies the existing staging pin set into prod
verbatim — no new versions are cut, so what you tested in staging is
exactly what ships.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if client == nil {
			return fmt.Errorf("no API key configured — run `tavora login` first")
		}
		to := strings.TrimSpace(promoteTo)
		if to != "staging" && to != "prod" {
			return fmt.Errorf("--to must be \"staging\" or \"prod\" (got %q)", promoteTo)
		}
		project, err := resolveProjectName(promoteProject)
		if err != nil {
			return err
		}

		var rel *tavora.Release
		if to == "staging" {
			// Promoting "to staging" cuts a fresh staging release.
			rel, err = client.CutRelease(globalCtx(), project)
		} else {
			// Promoting "to prod" copies staging's pin set into prod.
			deps, derr := client.ListDeployments(globalCtx(), project)
			if derr != nil {
				return derr
			}
			stagingSlug := findEnvSlug(deps, "staging")
			if stagingSlug == "" {
				return fmt.Errorf("no staging environment to promote from — run `tavora deploy --env staging` first")
			}
			rel, err = client.PromoteDeployment(globalCtx(), project, stagingSlug, "prod")
		}
		if err != nil {
			return err
		}
		if isJSON() {
			return printJSON(rel)
		}
		status("promoted → %s (%s): %d agent(s) pinned", rel.Deployment.Kind, rel.Deployment.Slug, len(rel.Pins))
		return nil
	},
}

var shipProject string

var shipCmd = &cobra.Command{
	Use:   "ship",
	Short: "Cut a staging release and promote it straight to production",
	Long: `tavora ship is the one-step path for when you don't need a manual
staging check: it cuts a fresh staging release (diffing each agent by
source hash) and immediately promotes that exact pin set to production.

Equivalent to:
  tavora deploy --env staging
  tavora promote --to prod

Use the two-step form when you want to validate in staging (run evals,
eyeball behavior) before shipping.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if client == nil {
			return fmt.Errorf("no API key configured — run `tavora login` first")
		}
		project, err := resolveProjectName(shipProject)
		if err != nil {
			return err
		}
		staging, err := client.CutRelease(globalCtx(), project)
		if err != nil {
			return err
		}
		status("cut staging release: %d agent(s) pinned in %s", len(staging.Pins), staging.Deployment.Slug)
		prod, err := client.PromoteDeployment(globalCtx(), project, staging.Deployment.Slug, "prod")
		if err != nil {
			return err
		}
		if isJSON() {
			return printJSON(prod)
		}
		status("shipped → prod (%s): %d agent(s) pinned", prod.Deployment.Slug, len(prod.Pins))
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the pinned version per agent in each environment (dev/staging/prod)",
	Long: `tavora status prints each of the project's environments and the
version pinned for every agent in it — the Tier 3 deployment dashboard
in the terminal.

An environment that doesn't exist yet, or has no pin for an agent,
prints "—".`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if client == nil {
			return fmt.Errorf("no API key configured — run `tavora login` first")
		}
		project, err := resolveProjectName(statusProject)
		if err != nil {
			return err
		}
		deps, err := client.ListDeployments(globalCtx(), project)
		if err != nil {
			return err
		}
		if len(deps) == 0 {
			fmt.Fprintf(os.Stderr, "No environments in project %q yet — run `tavora deploy --env staging`.\n", project)
			return nil
		}

		// Gather pins per environment, then render an agent × env grid so
		// version drift across environments is obvious at a glance.
		type envPins struct {
			dep  tavora.Deployment
			pins map[string]int64 // agentID → version
		}
		var envs []envPins
		agentSet := map[string]struct{}{}
		for _, d := range deps {
			pins, perr := client.ListDeploymentPins(globalCtx(), project, d.Slug)
			if perr != nil {
				return perr
			}
			m := make(map[string]int64, len(pins))
			for _, p := range pins {
				m[p.AgentID] = p.VersionNumber
				agentSet[p.AgentID] = struct{}{}
			}
			envs = append(envs, envPins{dep: d, pins: m})
		}

		if isJSON() {
			return printJSON(deps)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		header := "AGENT"
		for _, e := range envs {
			header += "\t" + strings.ToUpper(e.dep.Kind)
		}
		fmt.Fprintln(w, header)
		for agent := range agentSet {
			row := agent
			for _, e := range envs {
				if v, ok := e.pins[agent]; ok {
					row += fmt.Sprintf("\tv%d", v)
				} else {
					row += "\t—"
				}
			}
			fmt.Fprintln(w, row)
		}
		return w.Flush()
	},
}

func init() {
	promoteCmd.Flags().StringVar(&promoteTo, "to", "", "target environment: staging or prod (required)")
	promoteCmd.Flags().StringVar(&promoteProject, "project", "", "project name (defaults to the local tavora/ manifest)")
	_ = promoteCmd.MarkFlagRequired("to")
	rootCmd.AddCommand(promoteCmd)

	statusCmd.Flags().StringVar(&statusProject, "project", "", "project name (defaults to the local tavora/ manifest)")
	rootCmd.AddCommand(statusCmd)

	shipCmd.Flags().StringVar(&shipProject, "project", "", "project name (defaults to the local tavora/ manifest)")
	rootCmd.AddCommand(shipCmd)
}
