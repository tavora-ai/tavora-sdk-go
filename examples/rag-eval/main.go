// RAG quality evaluation — validates the full retrieval-augmented generation pipeline.
//
// Uploads support documentation, runs search and chat test cases, and reports
// pass/fail scores for retrieval quality and answer accuracy.
//
// Usage:
//
//	export TAVORA_URL=http://localhost:8080
//	export TAVORA_API_KEY=tvr_...
//
//	go run . --docs ../support-bot/support-docs
//	go run . --store <existing-store-id>
//	go run . --docs ../support-bot/support-docs --verbose
package main

import (
	"context"
	"fmt"
	"os"

	tavora "github.com/tavora-ai/tavora-sdk-go"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	url := os.Getenv("TAVORA_URL")
	key := os.Getenv("TAVORA_API_KEY")
	if url == "" || key == "" {
		return fmt.Errorf("set TAVORA_URL and TAVORA_API_KEY environment variables")
	}

	docsDir := ""
	storeID := ""
	verbose := false
	cleanup := true

	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--docs":
			if i+1 < len(os.Args) {
				i++
				docsDir = os.Args[i]
			}
		case "--store":
			if i+1 < len(os.Args) {
				i++
				storeID = os.Args[i]
			}
		case "--verbose", "-v":
			verbose = true
		case "--no-cleanup":
			cleanup = false
		}
	}

	if docsDir == "" && storeID == "" {
		fmt.Println("RAG Quality Evaluation")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  go run . --docs ../support-bot/support-docs    Upload docs and run eval")
		fmt.Println("  go run . --store <id>                          Use existing store")
		fmt.Println("  go run . --docs ... --verbose                  Show search results and answers")
		fmt.Println("  go run . --docs ... --no-cleanup               Keep store after eval")
		return nil
	}

	client := tavora.NewClient(url, key)
	ctx := context.Background()

	ws, err := client.GetWorkspace(ctx)
	if err != nil {
		return fmt.Errorf("connecting to Tavora: %w", err)
	}
	fmt.Printf("Connected to workspace: %s\n\n", ws.Name)

	// Upload if needed
	createdStore := false
	if docsDir != "" {
		id, err := uploadAndWait(ctx, client, docsDir)
		if err != nil {
			return err
		}
		storeID = id
		createdStore = true
	}

	// Run retrieval tests
	retrievalResults := runRetrievalCases(ctx, client, storeID, retrievalCases, verbose)
	rPassed, rFailed := printResults("Retrieval Quality", retrievalResults)

	// Run e2e tests
	e2eResults := runE2ECases(ctx, client, storeID, e2eCases, verbose)
	ePassed, eFailed := printResults("End-to-End RAG", e2eResults)

	// Summary
	totalPassed := rPassed + ePassed
	totalFailed := rFailed + eFailed
	total := totalPassed + totalFailed

	// Average score
	var totalScore float64
	for _, r := range retrievalResults {
		totalScore += r.Score
	}
	for _, r := range e2eResults {
		totalScore += r.Score
	}
	avgScore := totalScore / float64(total)

	fmt.Printf("Results: %d/%d passed, avg score: %.2f\n", totalPassed, total, avgScore)

	// Cleanup
	if createdStore && cleanup {
		fmt.Printf("Cleaning up store %s...\n", storeID)
		if err := client.DeleteStore(ctx, storeID); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to delete store: %v\n", err)
		}
	}

	if totalFailed > 0 {
		return fmt.Errorf("%d test(s) failed", totalFailed)
	}
	return nil
}
