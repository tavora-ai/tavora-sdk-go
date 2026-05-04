// Agentic chat — demonstrates a multi-turn agent conversation via the
// Tavora SDK.
//
// Unlike examples/support-bot (which uses the stateless Conversation +
// SendMessage API), this example drives the full agent reasoning loop:
// every user turn goes through CreateAgentSession → RunAgent, so the
// model can execute JavaScript in the Goja sandbox, call MCP tools,
// fetch URLs, and compose multi-step work per turn. The REPL reuses
// one AgentSession across all turns so the agent remembers prior
// exchanges.
//
// Usage:
//
//	export TAVORA_URL=http://localhost:8080
//	export TAVORA_API_KEY=tvr_...
//	go run .
//
//	# Optional flags:
//	go run . --title "Trip planning" \
//	         --system-prompt "You plan trips. Use search for live data." \
//	         --tools search,list_stores
//
// REPL commands: /exit, /quit, /reset (starts a fresh session), /help.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	tavora "github.com/tavora-ai/tavora-sdk-go"
)

const defaultSystemPrompt = `You are a helpful assistant running inside Tavora's code-reasoning sandbox.

When a task needs computation, live data, or multi-step work, use execute_js to write a single JavaScript program that solves it. For simple factual questions, just answer directly.`

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		title        = flag.String("title", "", "session title (default: auto-generated)")
		systemPrompt = flag.String("system-prompt", defaultSystemPrompt, "agent system prompt")
		toolsCSV     = flag.String("tools", "", "comma-separated tool names, e.g. 'search,list_stores' (default: empty = sandbox primitives only)")
		model        = flag.String("model", "", "override model (default: workspace default)")
	)
	flag.Parse()

	url := os.Getenv("TAVORA_URL")
	key := os.Getenv("TAVORA_API_KEY")
	if url == "" || key == "" {
		return fmt.Errorf("set TAVORA_URL and TAVORA_API_KEY environment variables")
	}

	client := tavora.NewClient(url, key)
	ctx := context.Background()

	ws, err := client.GetWorkspace(ctx)
	if err != nil {
		return fmt.Errorf("connecting to Tavora: %w", err)
	}
	fmt.Printf("Connected to workspace: %s\n", ws.Name)

	sessionTitle := *title
	if sessionTitle == "" {
		sessionTitle = "Chat"
	}
	tools := parseTools(*toolsCSV)

	session, err := createSession(ctx, client, sessionTitle, *systemPrompt, *model, tools)
	if err != nil {
		return err
	}
	fmt.Printf("Session:   %s\n", session.ID)
	if len(tools) > 0 {
		fmt.Printf("Tools:     %s\n", strings.Join(tools, ", "))
	}
	fmt.Println()
	fmt.Println("Type a message. /help for commands, /exit to quit.")
	fmt.Println()

	return repl(ctx, client, session, sessionTitle, *systemPrompt, *model, tools)
}

func repl(ctx context.Context, client *tavora.Client, session *tavora.AgentSession, title, systemPrompt, model string, tools []string) error {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	current := session
	for {
		fmt.Print("You: ")
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("stdin: %w", err)
			}
			fmt.Println()
			return nil
		}
		msg := strings.TrimSpace(scanner.Text())
		if msg == "" {
			continue
		}

		switch msg {
		case "/exit", "/quit":
			return nil
		case "/help":
			printHelp()
			continue
		case "/reset":
			next, err := createSession(ctx, client, title, systemPrompt, model, tools)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[reset failed: %v]\n", err)
				continue
			}
			current = next
			fmt.Printf("[new session: %s]\n\n", current.ID)
			continue
		}

		if err := runTurn(ctx, client, current.ID, msg); err != nil {
			fmt.Fprintf(os.Stderr, "[run failed: %v]\n", err)
		}
		fmt.Println()
	}
}

func runTurn(ctx context.Context, client *tavora.Client, sessionID, message string) error {
	return client.RunAgent(ctx, sessionID, message, func(evt tavora.AgentEvent) {
		switch evt.Type {
		case "tool_call":
			args, _ := json.Marshal(evt.Args)
			fmt.Fprintf(os.Stderr, "  [tool_call] %s %s\n", evt.Tool, string(args))
		case "tool_result":
			fmt.Fprintf(os.Stderr, "  [tool_result] %s\n", evt.Tool)
		case "execute_js":
			fmt.Fprintf(os.Stderr, "  [execute_js] %s\n", truncate(evt.Content, 120))
		case "execute_js_result":
			fmt.Fprintf(os.Stderr, "  [execute_js_result] %s\n", truncate(evt.Content, 120))
		case "sandbox_event":
			kind, _ := evt.Args["kind"].(string)
			summary, _ := evt.Args["summary"].(string)
			if kind == "" {
				kind = "sandbox"
			}
			fmt.Fprintf(os.Stderr, "  [%s] %s\n", kind, truncate(summary, 100))
		case "response":
			fmt.Printf("\nAgent: %s\n", evt.Content)
		case "error":
			fmt.Fprintf(os.Stderr, "  [error] %s\n", evt.Content)
		case "done":
			if evt.Summary != nil {
				fmt.Fprintf(os.Stderr, "\n[%d steps · %d prompt + %d completion tokens]\n",
					evt.Summary.Steps, evt.Summary.Tokens.Prompt, evt.Summary.Tokens.Completion)
			}
		}
	})
}

func createSession(ctx context.Context, client *tavora.Client, title, systemPrompt, model string, tools []string) (*tavora.AgentSession, error) {
	input := tavora.CreateAgentSessionInput{
		Title:        title,
		SystemPrompt: systemPrompt,
		Tools:        tools,
	}
	if model != "" {
		input.Model = model
	}
	session, err := client.CreateAgentSession(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}
	return session, nil
}

func parseTools(csv string) []string {
	if csv == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), "\n", " ⏎ ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func printHelp() {
	fmt.Println("Commands:")
	fmt.Println("  /exit, /quit   Leave the chat")
	fmt.Println("  /reset         Start a fresh session (drops history)")
	fmt.Println("  /help          Show this help")
}
