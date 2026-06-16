package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// Tier 3 rollback over the project-environment pin model. Every cut /
// promote records an immutable release snapshot; `tavora releases` lists
// an environment's history and `tavora rollback --to <#>` re-applies a
// prior snapshot (recording a fresh rollback release).

var (
	releasesEnv     string
	releasesProject string

	rollbackEnv     string
	rollbackTo      string
	rollbackProject string
)

// resolveEnvSlug resolves a project's environment of the given kind to its
// slug, erroring clearly when that environment doesn't exist yet.
func resolveEnvSlug(project, kind string) (string, error) {
	deps, err := client.ListDeployments(globalCtx(), project)
	if err != nil {
		return "", err
	}
	slug := findEnvSlug(deps, kind)
	if slug == "" {
		return "", fmt.Errorf("no %s environment in project %q yet", kind, project)
	}
	return slug, nil
}

var releasesCmd = &cobra.Command{
	Use:   "releases",
	Short: "List an environment's release history (newest first)",
	Long: `tavora releases prints the append-only release log for an
environment — each row is an immutable {agent → version} snapshot you can
roll back to.

  tavora releases --env prod      # production's history (default)
  tavora releases --env staging   # staging's history

Use the # column with "tavora rollback --to <#>".`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if client == nil {
			return fmt.Errorf("no API key configured — run `tavora login` first")
		}
		kind := strings.TrimSpace(releasesEnv)
		if kind != "staging" && kind != "prod" {
			return fmt.Errorf("--env must be \"staging\" or \"prod\" (got %q)", releasesEnv)
		}
		project, err := resolveProjectName(releasesProject)
		if err != nil {
			return err
		}
		slug, err := resolveEnvSlug(project, kind)
		if err != nil {
			return err
		}
		releases, err := client.ListReleases(globalCtx(), project, slug)
		if err != nil {
			return err
		}
		if isJSON() {
			return printJSON(releases)
		}
		if len(releases) == 0 {
			fmt.Fprintf(os.Stderr, "No releases in %s yet — run `tavora deploy --env %s`.\n", kind, kind)
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "#\tACTION\tAGENTS\tWHEN\tID")
		for i, r := range releases {
			marker := ""
			if i == 0 {
				marker = " (current)"
			}
			fmt.Fprintf(w, "%d\t%s%s\t%d\t%s\t%s\n",
				r.Seq, r.Action, marker, r.AgentCount,
				r.CreatedAt.Format("2006-01-02 15:04"), r.ID)
		}
		return w.Flush()
	},
}

var rollbackCmd = &cobra.Command{
	Use:   "rollback --to <#>",
	Short: "Roll an environment back to a prior release",
	Long: `tavora rollback re-applies a prior release's pinned versions to an
environment, recording a fresh rollback release. It's a pure re-pin — no
new versions are cut, so the environment runs exactly what that release
shipped.

  tavora rollback --to 3              # roll prod back to release #3
  tavora rollback --env staging --to 5

Find the release # with "tavora releases". --to also accepts a release id
(UUID) directly.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if client == nil {
			return fmt.Errorf("no API key configured — run `tavora login` first")
		}
		kind := strings.TrimSpace(rollbackEnv)
		if kind != "staging" && kind != "prod" {
			return fmt.Errorf("--env must be \"staging\" or \"prod\" (got %q)", rollbackEnv)
		}
		target := strings.TrimSpace(rollbackTo)
		if target == "" {
			return fmt.Errorf("--to is required (a release # from `tavora releases`, or a release id)")
		}
		project, err := resolveProjectName(rollbackProject)
		if err != nil {
			return err
		}
		slug, err := resolveEnvSlug(project, kind)
		if err != nil {
			return err
		}

		releaseID, err := resolveReleaseID(project, slug, target)
		if err != nil {
			return err
		}
		rel, err := client.Rollback(globalCtx(), project, slug, releaseID)
		if err != nil {
			return err
		}
		if isJSON() {
			return printJSON(rel)
		}
		status("rolled back %s (%s) to release #%v: %d agent(s) pinned",
			rel.Deployment.Kind, rel.Deployment.Slug, rel.Seq, len(rel.Pins))
		return nil
	},
}

// resolveReleaseID turns the --to value into a release id: a bare number
// is looked up against the environment's history by seq; anything else is
// treated as a release id already.
func resolveReleaseID(project, slug, target string) (string, error) {
	seq, err := strconv.ParseInt(target, 10, 64)
	if err != nil {
		return target, nil // not a number — assume it's already an id
	}
	releases, err := client.ListReleases(globalCtx(), project, slug)
	if err != nil {
		return "", err
	}
	for _, r := range releases {
		if r.Seq == seq {
			return r.ID, nil
		}
	}
	return "", fmt.Errorf("no release #%d in %s — run `tavora releases --env %s`", seq, slug, strings.TrimSpace(rollbackEnv))
}

func init() {
	releasesCmd.Flags().StringVar(&releasesEnv, "env", "prod", "environment to list: staging or prod")
	releasesCmd.Flags().StringVar(&releasesProject, "project", "", "project name (defaults to the local tavora/ manifest)")
	rootCmd.AddCommand(releasesCmd)

	rollbackCmd.Flags().StringVar(&rollbackEnv, "env", "prod", "environment to roll back: staging or prod")
	rollbackCmd.Flags().StringVar(&rollbackTo, "to", "", "release # (from `tavora releases`) or release id (required)")
	rollbackCmd.Flags().StringVar(&rollbackProject, "project", "", "project name (defaults to the local tavora/ manifest)")
	_ = rollbackCmd.MarkFlagRequired("to")
	rootCmd.AddCommand(rollbackCmd)
}
