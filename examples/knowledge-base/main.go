// Knowledge base example — demonstrates uploading documents and searching via the Tavora SDK.
//
// This program:
//  1. Creates a store
//  2. Uploads all files from a directory
//  3. Waits for processing to complete
//  4. Runs a semantic search query
//
// Usage:
//
//	export TAVORA_URL=http://localhost:8080
//	export TAVORA_API_KEY=tvr_...
//	go run . --dir ./docs --query "how does authentication work?"
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

	// Parse args
	dir := ""
	query := ""
	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--dir":
			if i+1 < len(os.Args) {
				i++
				dir = os.Args[i]
			}
		case "--query":
			if i+1 < len(os.Args) {
				i++
				query = os.Args[i]
			}
		}
	}

	if dir == "" && query == "" {
		fmt.Println("Usage: knowledge-base --dir ./docs --query \"your question\"")
		fmt.Println()
		fmt.Println("  --dir    Directory of files to upload (.pdf, .md, .txt, .csv)")
		fmt.Println("  --query  Search query to run after upload")
		fmt.Println()
		fmt.Println("You can use --dir alone (upload only), --query alone (search only), or both.")
		return nil
	}

	client := tavora.NewClient(url, key)
	ctx := context.Background()

	// Show product info
	ws, err := client.GetProduct(ctx)
	if err != nil {
		return fmt.Errorf("getting product: %w", err)
	}
	fmt.Printf("Product: %s (%s)\n\n", ws.Name, ws.ID)

	var storeID string

	// Upload files if --dir is provided
	if dir != "" {
		storeID, err = uploadDir(ctx, client, dir)
		if err != nil {
			return err
		}
	}

	// Search if --query is provided
	if query != "" {
		return searchDocs(ctx, client, query, storeID)
	}

	return nil
}

func uploadDir(ctx context.Context, client *tavora.Client, dir string) (string, error) {
	// Find uploadable files
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if allowedExts[ext] {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("scanning directory: %w", err)
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no uploadable files found in %s (supported: pdf, md, txt, csv)", dir)
	}

	// Create store named after the directory
	colName := filepath.Base(dir)
	fmt.Printf("Creating store: %s\n", colName)
	col, err := client.CreateIndex(ctx, tavora.CreateIndexInput{
		Name:        colName,
		Description: fmt.Sprintf("Uploaded from %s", dir),
	})
	if err != nil {
		return "", fmt.Errorf("creating store: %w", err)
	}
	fmt.Printf("  Store ID: %s\n\n", col.ID)

	// Upload each file
	fmt.Printf("Uploading %d files...\n", len(files))
	var docIDs []string
	for _, f := range files {
		rel, _ := filepath.Rel(dir, f)
		fmt.Printf("  %s", rel)
		doc, err := client.UploadDocument(ctx, tavora.UploadDocumentInput{
			FilePath:     f,
			IndexID: col.ID,
		})
		if err != nil {
			fmt.Printf(" - FAILED: %v\n", err)
			continue
		}
		fmt.Printf(" - uploaded (%s)\n", doc.ID)
		docIDs = append(docIDs, doc.ID)
	}

	// Wait for processing
	if len(docIDs) > 0 {
		fmt.Printf("\nWaiting for processing...")
		for attempts := 0; attempts < 60; attempts++ {
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
		fmt.Println(" done")

		// Show final status
		fmt.Println()
		for _, id := range docIDs {
			doc, err := client.GetDocument(ctx, id)
			if err != nil {
				continue
			}
			status := doc.Status
			if doc.ErrorMessage != nil {
				status += ": " + *doc.ErrorMessage
			}
			fmt.Printf("  %s — %s (%d chunks)\n", doc.Filename, status, doc.ChunkCount)
		}
	}

	return col.ID, nil
}

func searchDocs(ctx context.Context, client *tavora.Client, query, storeID string) error {
	fmt.Printf("\nSearching: %q\n\n", query)

	results, err := client.Search(ctx, tavora.SearchInput{
		Query:        query,
		IndexID: storeID,
		TopK:         5,
		MinScore:     0.3,
	})
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	for i, r := range results {
		fmt.Printf("--- Result %d (score: %.3f) ---\n", i+1, r.Score)
		fmt.Printf("Document: %s (chunk %d)\n", r.Filename, r.ChunkIndex)
		// Truncate long content for display
		content := r.Content
		if len(content) > 300 {
			content = content[:300] + "..."
		}
		fmt.Printf("%s\n\n", content)
	}

	return nil
}
