// Eval CI gate — runs eval suites and exits non-zero on failure.
//
// Use this in CI pipelines to gate deployments on agent quality.
//
// Usage:
//
//	export TAVORA_URL=http://localhost:8080
//	export TAVORA_API_KEY=tvr_...
//	go run . [flags]
//
// Flags:
//
//	--seed      Create sample eval cases if none exist
//	--set       Filter by eval set name (default: run all)
//	--timeout   Max wait for eval completion (default: 5m)
//
// GitHub Actions example:
//
//	# .github/workflows/eval.yml
//	# jobs:
//	#   eval-gate:
//	#     runs-on: ubuntu-latest
//	#     steps:
//	#       - uses: actions/checkout@v4
//	#       - uses: actions/setup-go@v5
//	#         with:
//	#           go-version: '1.25'
//	#       - run: cd examples/eval-ci && go run . --timeout 10m
//	#         env:
//	#           TAVORA_URL: ${{ secrets.TAVORA_URL }}
//	#           TAVORA_API_KEY: ${{ secrets.TAVORA_API_KEY }}
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	tavora "github.com/tavora-ai/tavora-sdk-go"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	seed := flag.Bool("seed", false, "create sample eval cases if none exist")
	setFilter := flag.String("set", "", "filter by eval set name")
	timeout := flag.Duration("timeout", 5*time.Minute, "max wait for eval completion")
	flag.Parse()

	url := os.Getenv("TAVORA_URL")
	key := os.Getenv("TAVORA_API_KEY")
	if url == "" || key == "" {
		return fmt.Errorf("set TAVORA_URL and TAVORA_API_KEY environment variables")
	}

	client := tavora.NewClient(url, key)
	ctx := context.Background()

	// Optionally seed sample eval cases
	if *seed {
		if err := seedCases(ctx, client); err != nil {
			return fmt.Errorf("seeding eval cases: %w", err)
		}
	}

	// Trigger eval run
	fmt.Fprintf(os.Stderr, "Starting eval run")
	if *setFilter != "" {
		fmt.Fprintf(os.Stderr, " (set: %s)", *setFilter)
	}
	fmt.Fprintln(os.Stderr)

	evalRun, err := client.RunEval(ctx, tavora.RunEvalInput{
		SetFilter: *setFilter,
	})
	if err != nil {
		return fmt.Errorf("triggering eval run: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Run ID: %s\n", evalRun.ID)

	// Poll for completion
	detail, err := pollForCompletion(ctx, client, evalRun.ID, *timeout)
	if err != nil {
		return err
	}

	// Print results
	printResults(detail)

	// Gate: fail if any case failed
	if detail.Run.Failed > 0 {
		return fmt.Errorf("eval failed: %d/%d cases did not pass", detail.Run.Failed, detail.Run.TotalCases)
	}

	fmt.Fprintf(os.Stderr, "\nAll %d cases passed.\n", detail.Run.TotalCases)
	return nil
}

func seedCases(ctx context.Context, client *tavora.Client) error {
	cases, err := client.ListEvalCases(ctx)
	if err != nil {
		return err
	}

	if len(cases) > 0 {
		fmt.Fprintf(os.Stderr, "Found %d existing eval cases, skipping seed.\n", len(cases))
		return nil
	}

	fmt.Fprintln(os.Stderr, "Seeding sample eval cases...")

	samples := []tavora.CreateEvalCaseInput{
		{
			Name:     "basic-search",
			SetName:  "ci",
			Prompt:   "Search for documents in the knowledge base and summarize what you find.",
			Criteria: "Must use the search tool at least once and provide a coherent summary of results. If no documents exist, should clearly state that.",
			Tools:    []string{"search", "list_stores"},
		},
		{
			Name:     "memory-usage",
			SetName:  "ci",
			Prompt:   "Remember that the project deadline is next Friday, then recall it.",
			Criteria: "Must use the remember tool to store information and the recall tool to retrieve it. The recalled information should match what was stored.",
			Tools:    []string{"remember", "recall", "memories"},
		},
	}

	for _, s := range samples {
		if _, err := client.CreateEvalCase(ctx, s); err != nil {
			return fmt.Errorf("creating case %q: %w", s.Name, err)
		}
		fmt.Fprintf(os.Stderr, "  Created: %s\n", s.Name)
	}

	return nil
}

func pollForCompletion(ctx context.Context, client *tavora.Client, runID string, timeout time.Duration) (*tavora.EvalRunDetail, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		detail, err := client.GetEvalRun(ctx, runID)
		if err != nil {
			return nil, fmt.Errorf("polling eval run: %w", err)
		}

		switch detail.Run.Status {
		case "completed":
			return detail, nil
		case "failed":
			return detail, fmt.Errorf("eval run failed")
		}

		fmt.Fprint(os.Stderr, ".")

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timed out waiting for eval run to complete")
		case <-ticker.C:
		}
	}
}

func printResults(detail *tavora.EvalRunDetail) {
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "Run: %s | Status: %s\n\n", detail.Run.ID, detail.Run.Status)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "CASE\tSCORE\tPASS\tDURATION")
	fmt.Fprintln(w, "----\t-----\t----\t--------")

	for _, r := range detail.Results {
		pass := "FAIL"
		if r.Pass {
			pass = "PASS"
		}
		fmt.Fprintf(w, "%s\t%d/10\t%s\t%dms\n", r.CaseName, r.Score, pass, r.DurationMs)
	}
	w.Flush()

	fmt.Fprintf(os.Stderr, "\nPassed: %d/%d | Average Score: %.1f/10\n",
		detail.Run.Passed, detail.Run.TotalCases, detail.Run.AverageScore)
}
