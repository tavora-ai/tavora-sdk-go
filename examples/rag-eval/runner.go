package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tavora "github.com/tavora-ai/tavora-sdk-go"
)

var allowedExts = map[string]bool{
	".pdf": true, ".md": true, ".txt": true, ".csv": true,
}

// uploadAndWait creates a store, uploads docs, and waits for processing.
func uploadAndWait(ctx context.Context, client *tavora.Client, docsDir string) (string, error) {
	var files []string
	err := filepath.Walk(docsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if allowedExts[strings.ToLower(filepath.Ext(path))] {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("scanning docs: %w", err)
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no uploadable files in %s", docsDir)
	}

	store, err := client.CreateStore(ctx, tavora.CreateStoreInput{
		Name:        "rag-eval-" + time.Now().Format("20060102-150405"),
		Description: "RAG eval test store",
	})
	if err != nil {
		return "", fmt.Errorf("creating store: %w", err)
	}

	fmt.Printf("Uploading %d files to store %s...\n", len(files), store.ID)
	var docIDs []string
	for _, f := range files {
		rel, _ := filepath.Rel(docsDir, f)
		doc, err := client.UploadDocument(ctx, tavora.UploadDocumentInput{
			FilePath: f,
			StoreID:  store.ID,
		})
		if err != nil {
			fmt.Printf("  %s FAILED: %v\n", rel, err)
			continue
		}
		fmt.Printf("  %s OK\n", rel)
		docIDs = append(docIDs, doc.ID)
	}

	fmt.Print("Processing")
	for i := 0; i < 90; i++ {
		time.Sleep(2 * time.Second)
		allDone := true
		for _, id := range docIDs {
			doc, err := client.GetDocument(ctx, id)
			if err != nil {
				continue
			}
			if doc.Status == "pending" || doc.Status == "processing" {
				allDone = false
				break
			}
		}
		if allDone {
			break
		}
		fmt.Print(".")
	}
	fmt.Println(" done\n")

	return store.ID, nil
}

// CaseResult holds the outcome of a single test case.
type CaseResult struct {
	Name        string
	Passed      bool
	Score       float64
	Missing     []string
	Description string
}

// runRetrievalCases runs search-only test cases.
func runRetrievalCases(ctx context.Context, client *tavora.Client, storeID string, cases []TestCase, verbose bool) []CaseResult {
	var results []CaseResult
	for _, tc := range cases {
		searchResults, err := client.Search(ctx, tavora.SearchInput{
			Query:   tc.Query,
			StoreID: storeID,
			TopK:    5,
		})
		if err != nil {
			results = append(results, CaseResult{
				Name:        tc.Name,
				Passed:      false,
				Score:       0,
				Missing:     tc.Expect,
				Description: fmt.Sprintf("search error: %v", err),
			})
			continue
		}

		// Combine all chunk content for keyword matching
		var combined strings.Builder
		for _, r := range searchResults {
			combined.WriteString(r.Content)
			combined.WriteString(" ")
		}
		content := combined.String()

		if verbose {
			fmt.Printf("\n--- %s: %q ---\n", tc.Name, tc.Query)
			for i, r := range searchResults {
				fmt.Printf("  [%d] score=%.3f file=%s\n      %s\n", i, r.Score, r.Filename, truncate(r.Content, 120))
			}
		}

		result := scoreKeywords(tc.Name, tc.Expect, content, tc.Description)
		results = append(results, result)
	}
	return results
}

// runE2ECases runs full RAG pipeline test cases (search + LLM).
func runE2ECases(ctx context.Context, client *tavora.Client, storeID string, cases []TestCase, verbose bool) []CaseResult {
	var results []CaseResult
	for _, tc := range cases {
		resp, err := client.ChatCompletion(ctx, tavora.ChatCompletionInput{
			Messages: []tavora.ChatMessage{
				{Role: "user", Content: tc.Query},
			},
			UseRAG:  true,
			StoreID: storeID,
		})
		if err != nil {
			results = append(results, CaseResult{
				Name:        tc.Name,
				Passed:      false,
				Score:       0,
				Missing:     tc.Expect,
				Description: fmt.Sprintf("chat error: %v", err),
			})
			continue
		}

		answer := ""
		if len(resp.Choices) > 0 {
			answer = resp.Choices[0].Message.Content
		}

		if verbose {
			fmt.Printf("\n--- %s: %q ---\n", tc.Name, tc.Query)
			fmt.Printf("  Answer: %s\n", truncate(answer, 200))
		}

		result := scoreKeywords(tc.Name, tc.Expect, answer, tc.Description)
		results = append(results, result)
	}
	return results
}

// scoreKeywords checks if all expected keywords appear in the content.
func scoreKeywords(name string, expect []string, content, description string) CaseResult {
	lower := strings.ToLower(content)
	found := 0
	var missing []string
	for _, kw := range expect {
		if strings.Contains(lower, strings.ToLower(kw)) {
			found++
		} else {
			missing = append(missing, kw)
		}
	}
	score := float64(found) / float64(len(expect))
	return CaseResult{
		Name:        name,
		Passed:      len(missing) == 0,
		Score:       score,
		Missing:     missing,
		Description: description,
	}
}

// printResults prints a formatted results table and returns pass/fail counts.
func printResults(title string, results []CaseResult) (passed, failed int) {
	fmt.Printf("=== %s ===\n\n", title)
	for _, r := range results {
		status := "PASS"
		if !r.Passed {
			status = "FAIL"
		}
		fmt.Printf("  %-4s  %-25s  %.2f", status, r.Name, r.Score)
		if len(r.Missing) > 0 {
			fmt.Printf("  missing: %s", strings.Join(r.Missing, ", "))
		}
		fmt.Println()
		if r.Passed {
			passed++
		} else {
			failed++
		}
	}
	fmt.Println()
	return
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
