// rag-eval-judge validates Tavora's RAG pipeline end-to-end with an
// LLM-as-judge grader.
//
// Pipeline under test:
//
//  1. Upload invoice fixtures into a Tavora store.
//  2. Ask the chat endpoint RAG-grounded questions about each invoice
//     ("What is the total on invoice INV-2026-0042?").
//  3. Score each answer with Gemini as a judge, comparing against the
//     ground-truth values in cases.json.
//
// Unlike rag-eval-formats (which does filename-based retrieval round-trips),
// this example tests *answer quality* — does the pipeline let an LLM
// produce correct, well-grounded answers from the retrieved context?
//
// Requires three environment variables:
//
//	TAVORA_URL=http://localhost:8080
//	TAVORA_API_KEY=tvr_...
//	GEMINI_API_KEY=...
//
// Usage:
//
//	# default testdata: ../../../tavora-testdata/extraction
//	go run .
//
//	# explicit testdata + limit to first 3 invoices
//	go run . --testdata /path/to/tavora-testdata/extraction --limit 3 --verbose
//
// Scores per field (0–10) are averaged; pass threshold is configurable.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	tavora "github.com/tavora-ai/tavora-sdk-go"
	"google.golang.org/genai"
)

// fieldQuery pairs a cases.json field with a natural-language question.
// The question is what we actually ask the RAG; the field tells us which
// ground-truth value to judge the answer against.
type fieldQuery struct {
	Field    string
	Question string
}

var fieldQueries = []fieldQuery{
	{"vendor", "What is the vendor or company name that issued this invoice?"},
	{"invoice_number", "What is the invoice number on this invoice?"},
	{"total", "What is the total amount due on this invoice?"},
	{"date", "What is the invoice date?"},
	{"currency", "What currency is used on this invoice?"},
}

type evalCase struct {
	File     string                 `json:"file"`
	Expected map[string]interface{} `json:"expected"`
}

type judgment struct {
	Score  int    `json:"score"`
	Reason string `json:"reason"`
}

type caseResult struct {
	File        string
	Field       string
	Question    string
	Expected    interface{}
	Answer      string
	Score       int
	JudgeReason string
	Err         string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	testdata := flag.String("testdata", "", "path to tavora-testdata/extraction (default: auto-detect sibling)")
	limit := flag.Int("limit", 0, "limit to first N cases (0 = all)")
	threshold := flag.Int("pass-threshold", 7, "min judge score (0–10) to count as a pass")
	judgeModel := flag.String("judge-model", "gemini-2.5-flash", "Gemini model for the judge")
	verbose := flag.Bool("verbose", false, "print per-case details")
	cleanup := flag.Bool("cleanup", true, "delete the tavora store after eval")
	flag.Parse()

	url := os.Getenv("TAVORA_URL")
	key := os.Getenv("TAVORA_API_KEY")
	geminiKey := os.Getenv("GEMINI_API_KEY")
	if url == "" || key == "" || geminiKey == "" {
		return fmt.Errorf("set TAVORA_URL, TAVORA_API_KEY, and GEMINI_API_KEY")
	}

	testdataPath, err := resolveTestdata(*testdata)
	if err != nil {
		return err
	}
	casesPath := filepath.Join(testdataPath, "cases.json")
	cases, err := loadCases(casesPath)
	if err != nil {
		return fmt.Errorf("loading cases: %w", err)
	}
	if *limit > 0 && *limit < len(cases) {
		cases = cases[:*limit]
	}
	fmt.Printf("Testdata: %s\n", testdataPath)
	fmt.Printf("Cases: %d × fields: %d = %d judgments\n\n", len(cases), len(fieldQueries), len(cases)*len(fieldQueries))

	ctx := context.Background()

	// --- Tavora setup ---
	client := tavora.NewClient(url, key)
	ws, err := client.GetWorkspace(ctx)
	if err != nil {
		return fmt.Errorf("connecting to tavora: %w", err)
	}
	fmt.Printf("Connected to workspace: %s\n", ws.Name)

	store, err := client.CreateStore(ctx, tavora.CreateStoreInput{
		Name:        "rag-eval-judge-" + time.Now().Format("20060102-150405"),
		Description: "LLM-as-judge RAG eval",
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

	// --- Upload invoices ---
	docByFile, err := uploadInvoices(ctx, client, store.ID, testdataPath, cases, *verbose)
	if err != nil {
		return err
	}

	// --- Gemini judge ---
	gc, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  geminiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return fmt.Errorf("creating genai client: %w", err)
	}

	// --- Eval loop ---
	fmt.Println("Running eval...")
	var results []caseResult
	for _, c := range cases {
		docID := docByFile[c.File]
		if docID == "" {
			fmt.Printf("  [skip] %s — not processed\n", filepath.Base(c.File))
			continue
		}
		for _, fq := range fieldQueries {
			expected, ok := c.Expected[fq.Field]
			if !ok {
				continue
			}
			res := caseResult{
				File:     filepath.Base(c.File),
				Field:    fq.Field,
				Question: fq.Question,
				Expected: expected,
			}
			answer, err := askRAG(ctx, client, store.ID, fq.Question)
			if err != nil {
				res.Err = "rag: " + err.Error()
				results = append(results, res)
				continue
			}
			res.Answer = answer
			verdict, err := judge(ctx, gc, *judgeModel, fq.Question, expected, answer)
			if err != nil {
				res.Err = "judge: " + err.Error()
				results = append(results, res)
				continue
			}
			res.Score = verdict.Score
			res.JudgeReason = verdict.Reason
			results = append(results, res)
			if *verbose {
				fmt.Printf("  [%d/10] %-18s %-18s expected=%v  answer=%q\n",
					res.Score, res.File, res.Field, res.Expected, truncate(res.Answer, 60))
			}
		}
	}

	printReport(results, *threshold, *verbose)
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
	cwd, _ := os.Getwd()
	candidates := []string{
		filepath.Join(cwd, "..", "..", "..", "tavora-testdata", "extraction"),
		filepath.Join(cwd, "..", "..", "tavora-testdata", "extraction"),
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs, nil
		}
	}
	return "", fmt.Errorf("could not locate tavora-testdata/extraction; pass --testdata <path>")
}

func loadCases(path string) ([]evalCase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cases []evalCase
	if err := json.Unmarshal(data, &cases); err != nil {
		return nil, err
	}
	return cases, nil
}

func uploadInvoices(ctx context.Context, client *tavora.Client, storeID, testdataPath string, cases []evalCase, verbose bool) (map[string]string, error) {
	docByFile := map[string]string{}
	var docIDs []string
	for _, c := range cases {
		abs := filepath.Join(testdataPath, c.File)
		doc, err := client.UploadDocument(ctx, tavora.UploadDocumentInput{
			FilePath: abs,
			StoreID:  storeID,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "  upload %s: %v\n", c.File, err)
			continue
		}
		docByFile[c.File] = doc.ID
		docIDs = append(docIDs, doc.ID)
		if verbose {
			fmt.Printf("  uploaded %s → %s\n", c.File, doc.ID)
		}
	}
	fmt.Printf("Uploaded %d/%d docs; waiting for processing...\n", len(docIDs), len(cases))

	deadline := time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) {
		pending := 0
		for _, id := range docIDs {
			d, err := client.GetDocument(ctx, id)
			if err != nil {
				continue
			}
			if d.Status == "pending" || d.Status == "processing" {
				pending++
			}
		}
		if pending == 0 {
			break
		}
		time.Sleep(2 * time.Second)
	}
	fmt.Println()
	return docByFile, nil
}

// askRAG queries Tavora chat with RAG enabled against a specific store.
func askRAG(ctx context.Context, client *tavora.Client, storeID, question string) (string, error) {
	resp, err := client.ChatCompletion(ctx, tavora.ChatCompletionInput{
		Messages: []tavora.ChatMessage{{Role: "user", Content: question}},
		UseRAG:   true,
		StoreID:  storeID,
	})
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}
	return resp.Choices[0].Message.Content, nil
}

const judgePromptTemplate = `You are evaluating whether an AI assistant's answer correctly reflects a known ground-truth value.

Question asked to the assistant:
%s

Ground truth (expected value):
%v

Assistant's answer:
%s

Scoring guide (0–10):
- 10 = exact match or trivially-equivalent formatting (e.g. "$2,657.71" vs 2657.71)
- 7–9 = correct value with extra prose or minor presentation differences
- 4–6 = partially correct, or the right value appears but contradictory info is also present
- 1–3 = wrong value but recognisable topic
- 0 = unrelated, refusal, or blank

Focus on the VALUE, not the framing. Return ONLY a single JSON object:
{"score": <int 0-10>, "reason": "<1 short sentence>"}`

var jsonBlockRE = regexp.MustCompile(`(?s)\{.*\}`)

func judge(ctx context.Context, gc *genai.Client, model, question string, expected interface{}, answer string) (*judgment, error) {
	prompt := fmt.Sprintf(judgePromptTemplate, question, expected, answer)
	resp, err := gc.Models.GenerateContent(ctx, model, genai.Text(prompt), nil)
	if err != nil {
		return nil, err
	}
	text := resp.Text()
	match := jsonBlockRE.FindString(text)
	if match == "" {
		return nil, fmt.Errorf("no JSON in judge response: %s", truncate(text, 200))
	}
	var v judgment
	if err := json.Unmarshal([]byte(match), &v); err != nil {
		return nil, fmt.Errorf("parse judgment: %w (raw: %s)", err, truncate(match, 200))
	}
	if v.Score < 0 {
		v.Score = 0
	}
	if v.Score > 10 {
		v.Score = 10
	}
	return &v, nil
}

func printReport(results []caseResult, threshold int, verbose bool) {
	fmt.Println()
	fmt.Println(strings.Repeat("─", 80))

	// Aggregate by field
	byField := map[string]*fieldAgg{}
	for _, r := range results {
		a, ok := byField[r.Field]
		if !ok {
			a = &fieldAgg{}
			byField[r.Field] = a
		}
		a.Total++
		if r.Err != "" {
			a.Errors++
			continue
		}
		a.SumScore += r.Score
		if r.Score >= threshold {
			a.Passed++
		}
	}

	fmt.Printf("%-18s  %-8s  %-8s  %-10s  %-8s\n", "field", "passed", "errors", "avg score", "graded")
	fmt.Println(strings.Repeat("─", 80))

	fields := make([]string, 0, len(byField))
	for f := range byField {
		fields = append(fields, f)
	}
	sort.Strings(fields)
	var totalPassed, totalTotal, totalErrors, totalGraded int
	var totalScore int
	for _, f := range fields {
		a := byField[f]
		graded := a.Total - a.Errors
		avg := 0.0
		if graded > 0 {
			avg = float64(a.SumScore) / float64(graded)
		}
		fmt.Printf("%-18s  %d/%d       %d         %.2f        %d\n",
			f, a.Passed, a.Total, a.Errors, avg, graded)
		totalPassed += a.Passed
		totalTotal += a.Total
		totalErrors += a.Errors
		totalGraded += graded
		totalScore += a.SumScore
	}
	fmt.Println(strings.Repeat("─", 80))
	totalAvg := 0.0
	if totalGraded > 0 {
		totalAvg = float64(totalScore) / float64(totalGraded)
	}
	fmt.Printf("%-18s  %d/%d       %d         %.2f        %d\n",
		"TOTAL", totalPassed, totalTotal, totalErrors, totalAvg, totalGraded)

	// Show failures (low-score and errors)
	var weak []caseResult
	for _, r := range results {
		if r.Err != "" || r.Score < threshold {
			weak = append(weak, r)
		}
	}
	if len(weak) > 0 {
		fmt.Println("\nFailures / low scores:")
		for _, r := range weak {
			if r.Err != "" {
				fmt.Printf("  [ERR] %s %s — %s\n", r.File, r.Field, truncate(r.Err, 100))
				continue
			}
			fmt.Printf("  [%d/10] %s %s\n", r.Score, r.File, r.Field)
			fmt.Printf("      expected=%v  answer=%q\n", r.Expected, truncate(r.Answer, 80))
			fmt.Printf("      judge: %s\n", r.JudgeReason)
		}
	}
}

type fieldAgg struct {
	Total    int
	Passed   int
	Errors   int
	SumScore int
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
