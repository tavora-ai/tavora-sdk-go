// Customer support bot — demonstrates RAG-augmented conversational chat via the Tavora SDK.
//
// This program:
//  1. Optionally uploads support documentation from a directory
//  2. Creates a conversation with a customer support system prompt
//  3. Runs an interactive REPL where you can ask questions
//  4. Answers are augmented with relevant documentation via RAG
//
// Usage:
//
//	export TAVORA_URL=http://localhost:8080
//	export TAVORA_API_KEY=tvr_...
//
//	# Interactive mode with doc upload
//	go run . --docs ./support-docs
//
//	# Interactive mode with existing store
//	go run . --store <store-id>
//
//	# Single question mode
//	go run . --question "How do I reset my password?"
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tavora "github.com/tavora-ai/tavora-sdk-go"
)

const systemPrompt = `You are a helpful customer support assistant. Answer questions based on the provided documentation.

Guidelines:
- Be concise, friendly, and accurate
- Reference specific documentation when possible
- If you don't know the answer, say so and suggest contacting support
- Keep responses focused and actionable`

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

	// Parse flags
	docsDir := ""
	storeID := ""
	question := ""
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
		case "--question":
			if i+1 < len(os.Args) {
				i++
				question = os.Args[i]
			}
		}
	}

	if docsDir == "" && storeID == "" && question == "" {
		fmt.Println("Customer Support Bot — RAG-augmented chat via Tavora SDK")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  go run . --docs ./support-docs           Upload docs and start chatting")
		fmt.Println("  go run . --store <id>                    Chat with existing store")
		fmt.Println("  go run . --question \"How do I...\"        Ask a single question")
		fmt.Println()
		fmt.Println("Flags can be combined: --docs ./docs --question \"How do I reset my password?\"")
		return nil
	}

	client := tavora.NewClient(url, key)
	ctx := context.Background()

	// Show workspace
	ws, err := client.GetWorkspace(ctx)
	if err != nil {
		return fmt.Errorf("connecting to Tavora: %w", err)
	}
	fmt.Printf("Connected to workspace: %s\n", ws.Name)

	// Upload docs if needed
	if docsDir != "" {
		id, err := uploadDocs(ctx, client, docsDir)
		if err != nil {
			return err
		}
		storeID = id
	}

	// Create conversation
	conv, err := client.CreateConversation(ctx, tavora.CreateConversationInput{
		Title:        "Support Chat " + time.Now().Format("2006-01-02 15:04"),
		SystemPrompt: systemPrompt,
	})
	if err != nil {
		return fmt.Errorf("creating conversation: %w", err)
	}
	fmt.Printf("Conversation: %s\n\n", conv.ID)

	// Single question mode
	if question != "" {
		return askQuestion(ctx, client, conv.ID, storeID, question)
	}

	// Interactive REPL
	return repl(ctx, client, conv.ID, storeID)
}

func askQuestion(ctx context.Context, client *tavora.Client, convID, storeID, question string) error {
	input := tavora.SendMessageInput{
		Content: question,
		UseRAG:  true,
	}
	if storeID != "" {
		input.IndexID = storeID
	}

	result, err := client.SendMessage(ctx, convID, input)
	if err != nil {
		return fmt.Errorf("sending message: %w", err)
	}

	fmt.Println(result.Message.Content)
	fmt.Fprintf(os.Stderr, "\n[%d tokens]\n", result.TokenUsage.TotalTokens)
	return nil
}

func repl(ctx context.Context, client *tavora.Client, convID, storeID string) error {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Type your questions below. Press Ctrl+C to exit.")
	fmt.Println()

	for {
		fmt.Print("You: ")
		if !scanner.Scan() {
			break
		}
		question := strings.TrimSpace(scanner.Text())
		if question == "" {
			continue
		}
		if question == "/quit" || question == "/exit" {
			break
		}

		input := tavora.SendMessageInput{
			Content: question,
			UseRAG:  true,
		}
		if storeID != "" {
			input.IndexID = storeID
		}

		result, err := client.SendMessage(ctx, convID, input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			continue
		}

		fmt.Printf("\nBot: %s\n", result.Message.Content)
		fmt.Fprintf(os.Stderr, "[%d tokens]\n\n", result.TokenUsage.TotalTokens)
	}

	return nil
}

func uploadDocs(ctx context.Context, client *tavora.Client, dir string) (string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
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
		return "", fmt.Errorf("no uploadable files in %s", dir)
	}

	colName := "support-docs-" + time.Now().Format("20060102-1504")
	col, err := client.CreateIndex(ctx, tavora.CreateIndexInput{
		Name:        colName,
		Description: "Support documentation",
	})
	if err != nil {
		return "", fmt.Errorf("creating store: %w", err)
	}

	fmt.Printf("Uploading %d files to store %s...\n", len(files), col.ID)
	var docIDs []string
	for _, f := range files {
		rel, _ := filepath.Rel(dir, f)
		fmt.Printf("  %s", rel)
		doc, err := client.UploadDocument(ctx, tavora.UploadDocumentInput{
			FilePath:     f,
			IndexID: col.ID,
		})
		if err != nil {
			fmt.Printf(" FAILED: %v\n", err)
			continue
		}
		fmt.Printf(" OK (%s)\n", doc.ID)
		docIDs = append(docIDs, doc.ID)
	}

	// Wait for processing
	if len(docIDs) > 0 {
		fmt.Print("Processing")
		for i := 0; i < 60; i++ {
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
	}

	return col.ID, nil
}
