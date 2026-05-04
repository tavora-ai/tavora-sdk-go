// Research assistant — demonstrates agent tool use via the Tavora SDK.
//
// The agent uses the search tool to find relevant documents and synthesizes answers.
//
// Usage:
//
//	export TAVORA_URL=http://localhost:8080
//	export TAVORA_API_KEY=tvr_...
//	go run . "What documents do we have about authentication?"
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

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

	if len(os.Args) < 2 {
		fmt.Println("Usage: research-assistant \"your question here\"")
		return nil
	}
	question := strings.Join(os.Args[1:], " ")

	client := tavora.NewClient(url, key)
	ctx := context.Background()

	// Create agent session
	session, err := client.CreateAgentSession(ctx, tavora.CreateAgentSessionInput{
		Title: "Research: " + question[:min(len(question), 50)],
		SystemPrompt: `You are a research assistant. Use the search tool to find relevant documents and synthesize a comprehensive answer.

Guidelines:
- Search for relevant information before answering
- Cite specific documents when referencing information
- If search returns no results, say so clearly
- Provide a well-structured, thorough response`,
		Tools: []string{"search", "list_stores"},
	})
	if err != nil {
		return fmt.Errorf("creating session: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Session: %s\n\n", session.ID)

	// Run the agent
	err = client.RunAgent(ctx, session.ID, question, func(evt tavora.AgentEvent) {
		switch evt.Type {
		case "tool_call":
			argsJSON, _ := json.Marshal(evt.Args)
			fmt.Fprintf(os.Stderr, "[%s] %s\n", evt.Tool, string(argsJSON))
		case "tool_result":
			fmt.Fprintf(os.Stderr, "[%s] done\n", evt.Tool)
		case "response":
			fmt.Println(evt.Content)
		case "error":
			fmt.Fprintf(os.Stderr, "[error] %s\n", evt.Content)
		case "done":
			if evt.Summary != nil {
				fmt.Fprintf(os.Stderr, "\n[%d steps | %d tokens]\n",
					evt.Summary.Steps, evt.Summary.Tokens.Prompt+evt.Summary.Tokens.Completion)
			}
		}
	})
	if err != nil {
		return fmt.Errorf("agent run failed: %w", err)
	}

	return nil
}
