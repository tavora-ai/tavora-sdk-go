// llm-judge — minimal demo of LLM-as-judge scoring.
//
// Asks Gemini to score an assistant's answer against a known
// ground-truth value on a 0-10 rubric, returning a JSON verdict with
// score + one-sentence reason. This is the same scoring pattern that
// powers `tavora rag-eval judge` in tavora-tools, but isolated to ~80
// lines so SDK readers can see the LLM-as-judge primitive without the
// surrounding RAG pipeline.
//
// Note: this example uses google.golang.org/genai directly (not the
// Tavora SDK) because the LLM-judge call doesn't go through Tavora —
// it's a separate Gemini call against the same model the rest of your
// stack uses. If your eval needs to score answers against ground truth
// in CI, use `tavora rag-eval judge --gate` instead.
//
// Usage:
//
//	export GEMINI_API_KEY=...
//	go run .                    # uses the canned (question, expected, answer) trio
//	go run . --answer "the total is $2,657.71"
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"

	"google.golang.org/genai"
)

const promptTemplate = `You are evaluating whether an AI assistant's answer correctly reflects a known ground-truth value.

Question asked to the assistant:
%s

Ground truth (expected value):
%v

Assistant's answer:
%s

Scoring guide (0-10):
- 10 = exact match or trivially-equivalent formatting (e.g. "$2,657.71" vs 2657.71)
- 7-9 = correct value with extra prose or minor presentation differences
- 4-6 = partially correct, or the right value appears but contradictory info is also present
- 1-3 = wrong value but recognisable topic
- 0 = unrelated, refusal, or blank

Focus on the VALUE, not the framing. Return ONLY a single JSON object:
{"score": <int 0-10>, "reason": "<1 short sentence>"}`

type Verdict struct {
	Score  int    `json:"score"`
	Reason string `json:"reason"`
}

var jsonBlockRE = regexp.MustCompile(`(?s)\{.*\}`)

func main() {
	question := flag.String("question", "What is the total on this invoice?", "Question that was asked")
	expected := flag.String("expected", "2657.71", "Ground-truth value")
	answer := flag.String("answer", "The total amount due is $2,657.71.", "Assistant's answer to grade")
	model := flag.String("model", "gemini-2.5-flash", "Gemini model used by the judge")
	flag.Parse()

	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		fmt.Fprintln(os.Stderr, "error: set GEMINI_API_KEY")
		os.Exit(1)
	}

	ctx := context.Background()
	gc, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: key, Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	prompt := fmt.Sprintf(promptTemplate, *question, *expected, *answer)
	resp, err := gc.Models.GenerateContent(ctx, *model, genai.Text(prompt), nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "judge call failed: %v\n", err)
		os.Exit(1)
	}

	// The model wraps the JSON object in occasional prose; pluck it out
	// with a non-greedy regex rather than trusting the response to be
	// pure JSON.
	match := jsonBlockRE.FindString(resp.Text())
	if match == "" {
		fmt.Fprintf(os.Stderr, "no JSON in judge response: %s\n", resp.Text())
		os.Exit(1)
	}
	var v Verdict
	if err := json.Unmarshal([]byte(match), &v); err != nil {
		fmt.Fprintf(os.Stderr, "parse verdict: %v (raw: %s)\n", err, match)
		os.Exit(1)
	}

	fmt.Printf("question: %s\nexpected: %s\nanswer:   %s\n\nverdict:\n  score:  %d/10\n  reason: %s\n",
		*question, *expected, *answer, v.Score, v.Reason)
}
