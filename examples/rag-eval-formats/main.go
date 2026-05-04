// rag-eval-formats measures how the Tavora RAG pipeline handles a
// multi-format document corpus.
//
// It uploads a sampler from each supported format in the shared
// tavora-testdata/extraction/kreuzberg corpus, waits for processing,
// then runs a retrieval round-trip against each successfully processed
// document. The output is a per-format coverage table showing where
// the pipeline accepts, processes, and makes content searchable.
//
// Usage:
//
//	export TAVORA_URL=http://localhost:8080
//	export TAVORA_API_KEY=tvr_...
//
//	# default: ../../../tavora-testdata/extraction, 2 files per format
//	go run .
//
//	# custom testdata checkout and sample size
//	go run . --testdata /path/to/tavora-testdata/extraction --per-format 3
//
//	# keep the store for inspection
//	go run . --no-cleanup --verbose
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tavora "github.com/tavora-ai/tavora-sdk-go"
)

// formatSpec maps a human label to the kreuzberg subdirectory and the
// file extensions we pull from it. The five formats listed here are the
// ones the tavora-go RAG extractor supports today (internal/rag/extractor.go).
var formatSpecs = []struct {
	Label string
	Dir   string
	Exts  []string
}{
	{"pdf", "pdf", []string{".pdf"}},
	{"markdown", "markdown", []string{".md"}},
	{"text", "text", []string{".txt"}},
	{"csv", "csv", []string{".csv"}},
	{"html", "html", []string{".html", ".htm"}},
}

type docRecord struct {
	Format   string
	Filename string
	DocID    string
	Status   string
	Err      string
}

type formatReport struct {
	Label      string
	Attempted  int
	Uploaded   int
	Processed  int
	Searchable int
	Errors     []string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	testdata := flag.String("testdata", "", "path to tavora-testdata/extraction (default: auto-detect sibling)")
	perFormat := flag.Int("per-format", 2, "files to sample per format")
	cleanup := flag.Bool("cleanup", true, "delete store after eval")
	verbose := flag.Bool("verbose", false, "show per-file details")
	flag.Parse()

	url := os.Getenv("TAVORA_URL")
	key := os.Getenv("TAVORA_API_KEY")
	if url == "" || key == "" {
		return fmt.Errorf("set TAVORA_URL and TAVORA_API_KEY")
	}

	resolved, err := resolveTestdata(*testdata)
	if err != nil {
		return err
	}
	kreuzbergDir := filepath.Join(resolved, "kreuzberg")
	if _, err := os.Stat(kreuzbergDir); err != nil {
		return fmt.Errorf("kreuzberg dir missing at %s: %w", kreuzbergDir, err)
	}
	fmt.Printf("Using testdata: %s\n", resolved)
	fmt.Printf("Sampling up to %d files per format across %d formats\n\n", *perFormat, len(formatSpecs))

	client := tavora.NewClient(url, key)
	ctx := context.Background()

	ws, err := client.GetWorkspace(ctx)
	if err != nil {
		return fmt.Errorf("connecting: %w", err)
	}
	fmt.Printf("Connected to workspace: %s\n", ws.Name)

	store, err := client.CreateStore(ctx, tavora.CreateStoreInput{
		Name:        "rag-eval-formats-" + time.Now().Format("20060102-150405"),
		Description: "Format coverage eval",
	})
	if err != nil {
		return fmt.Errorf("creating store: %w", err)
	}
	fmt.Printf("Store: %s\n\n", store.ID)

	if *cleanup {
		defer func() {
			if err := client.DeleteStore(ctx, store.ID); err != nil {
				fmt.Fprintf(os.Stderr, "warning: delete store: %v\n", err)
			}
		}()
	}

	var records []docRecord
	for _, spec := range formatSpecs {
		files, err := sampleFiles(filepath.Join(kreuzbergDir, spec.Dir), spec.Exts, *perFormat)
		if err != nil {
			fmt.Printf("[%s] scan failed: %v\n", spec.Label, err)
			continue
		}
		for _, f := range files {
			rec := docRecord{Format: spec.Label, Filename: filepath.Base(f)}
			doc, upErr := client.UploadDocument(ctx, tavora.UploadDocumentInput{
				FilePath: f,
				StoreID:  store.ID,
			})
			if upErr != nil {
				rec.Err = upErr.Error()
				if *verbose {
					fmt.Printf("  [%s] %s upload FAILED: %v\n", spec.Label, rec.Filename, upErr)
				}
			} else {
				rec.DocID = doc.ID
				if *verbose {
					fmt.Printf("  [%s] %s uploaded (%s)\n", spec.Label, rec.Filename, doc.ID)
				}
			}
			records = append(records, rec)
		}
	}
	fmt.Printf("\nSubmitted %d files; waiting for processing...\n", len(records))

	records = waitForProcessing(ctx, client, records, 90*time.Second)

	fmt.Println("Running retrieval round-trip...")
	for i, rec := range records {
		if rec.Status != "completed" {
			continue
		}
		if checkSearchable(ctx, client, store.ID, rec) {
			records[i].Status = "searchable"
		}
		if *verbose {
			fmt.Printf("  [%s] %s → %s\n", rec.Format, rec.Filename, records[i].Status)
		}
	}

	reports := aggregate(records)
	printReport(reports, *verbose, records)

	if *cleanup {
		fmt.Printf("\nCleaning up store %s...\n", store.ID)
	} else {
		fmt.Printf("\nStore retained: %s\n", store.ID)
	}
	return nil
}

func resolveTestdata(flagVal string) (string, error) {
	if flagVal != "" {
		abs, err := filepath.Abs(flagVal)
		if err != nil {
			return "", err
		}
		return abs, nil
	}
	// auto-detect: ../../../tavora-testdata/extraction relative to this example
	exe, _ := os.Getwd()
	candidates := []string{
		filepath.Join(exe, "..", "..", "..", "tavora-testdata", "extraction"),
		filepath.Join(exe, "..", "..", "tavora-testdata", "extraction"),
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs, nil
		}
	}
	return "", fmt.Errorf("could not locate tavora-testdata/extraction; pass --testdata <path>")
}

func sampleFiles(dir string, exts []string, n int) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	wanted := map[string]bool{}
	for _, e := range exts {
		wanted[strings.ToLower(e)] = true
	}
	var picks []string
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		if wanted[strings.ToLower(filepath.Ext(ent.Name()))] {
			picks = append(picks, filepath.Join(dir, ent.Name()))
			if len(picks) == n {
				break
			}
		}
	}
	return picks, nil
}

// waitForProcessing polls document status until every record has
// settled out of pending/processing or the deadline is reached.
func waitForProcessing(ctx context.Context, client *tavora.Client, records []docRecord, maxWait time.Duration) []docRecord {
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		pending := 0
		for i, rec := range records {
			if rec.DocID == "" || isSettled(rec.Status) {
				continue
			}
			doc, err := client.GetDocument(ctx, rec.DocID)
			if err != nil {
				records[i].Err = "status poll: " + err.Error()
				records[i].Status = "failed"
				continue
			}
			records[i].Status = doc.Status
			if doc.ErrorMessage != nil && *doc.ErrorMessage != "" {
				records[i].Err = *doc.ErrorMessage
			}
			if doc.Status == "pending" || doc.Status == "processing" {
				pending++
			}
		}
		if pending == 0 {
			break
		}
		time.Sleep(2 * time.Second)
	}
	return records
}

func isSettled(status string) bool {
	return status == "completed" || status == "searchable" || status == "failed"
}

// checkSearchable verifies the document's content is retrievable via
// search. We use the filename stem as the query and accept a hit if any
// returned chunk is from this document.
func checkSearchable(ctx context.Context, client *tavora.Client, storeID string, rec docRecord) bool {
	query := strings.ReplaceAll(strings.TrimSuffix(rec.Filename, filepath.Ext(rec.Filename)), "_", " ")
	results, err := client.Search(ctx, tavora.SearchInput{
		Query:   query,
		StoreID: storeID,
		TopK:    5,
	})
	if err != nil {
		return false
	}
	for _, r := range results {
		if r.DocumentID == rec.DocID {
			return true
		}
	}
	return false
}

func aggregate(records []docRecord) []formatReport {
	byFormat := map[string]*formatReport{}
	for _, rec := range records {
		rep, ok := byFormat[rec.Format]
		if !ok {
			rep = &formatReport{Label: rec.Format}
			byFormat[rec.Format] = rep
		}
		rep.Attempted++
		if rec.DocID != "" {
			rep.Uploaded++
		}
		if rec.Status == "completed" || rec.Status == "searchable" {
			rep.Processed++
		}
		if rec.Status == "searchable" {
			rep.Searchable++
		}
		if rec.Err != "" {
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s: %s", rec.Filename, truncate(rec.Err, 80)))
		}
	}
	var out []formatReport
	for _, spec := range formatSpecs {
		if rep, ok := byFormat[spec.Label]; ok {
			out = append(out, *rep)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	return out
}

func printReport(reports []formatReport, verbose bool, records []docRecord) {
	fmt.Println()
	fmt.Println(strings.Repeat("─", 70))
	fmt.Printf("%-10s  %-9s  %-9s  %-10s\n", "format", "uploaded", "processed", "searchable")
	fmt.Println(strings.Repeat("─", 70))
	totalAtt, totalUp, totalProc, totalSrch := 0, 0, 0, 0
	for _, r := range reports {
		fmt.Printf("%-10s  %d/%d       %d/%d       %d/%d\n",
			r.Label, r.Uploaded, r.Attempted, r.Processed, r.Attempted, r.Searchable, r.Attempted)
		totalAtt += r.Attempted
		totalUp += r.Uploaded
		totalProc += r.Processed
		totalSrch += r.Searchable
	}
	fmt.Println(strings.Repeat("─", 70))
	fmt.Printf("%-10s  %d/%d       %d/%d       %d/%d\n",
		"TOTAL", totalUp, totalAtt, totalProc, totalAtt, totalSrch, totalAtt)

	var failing []formatReport
	for _, r := range reports {
		if len(r.Errors) > 0 {
			failing = append(failing, r)
		}
	}
	if len(failing) > 0 {
		fmt.Println("\nErrors:")
		for _, r := range failing {
			fmt.Printf("  [%s]\n", r.Label)
			for _, e := range r.Errors {
				fmt.Printf("    - %s\n", e)
			}
		}
	}
	if verbose {
		fmt.Println("\nPer-file detail:")
		for _, rec := range records {
			fmt.Printf("  [%s] %-40s status=%s\n", rec.Format, rec.Filename, rec.Status)
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
