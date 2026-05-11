package tavora

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ChatMessage represents a message in a conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionInput holds the parameters for a chat completion.
type ChatCompletionInput struct {
	// Model is the AI model to use (default: "gemini-2.5-flash").
	Model string `json:"model,omitempty"`
	// Messages is the conversation history. The last message must be from the user.
	Messages []ChatMessage `json:"messages"`
	// UseRAG enables retrieval-augmented generation from space documents.
	UseRAG bool `json:"use_rag,omitempty"`
	// IndexID limits RAG search to a specific store.
	IndexID string `json:"index_id,omitempty"`
}

// ChatCompletionChoice represents a single completion choice.
type ChatCompletionChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// ChatCompletionUsage holds token usage information.
type ChatCompletionUsage struct {
	PromptTokens     int32 `json:"prompt_tokens"`
	CompletionTokens int32 `json:"completion_tokens"`
	TotalTokens      int32 `json:"total_tokens"`
}

// ChatCompletionResult is the response from a chat completion request.
type ChatCompletionResult struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   ChatCompletionUsage    `json:"usage"`
}

// ChatCompletion sends a stateless chat completion request.
// The client sends the full message history each time — no server-side state.
func (c *Client) ChatCompletion(ctx context.Context, input ChatCompletionInput) (*ChatCompletionResult, error) {
	var result ChatCompletionResult
	if err := c.post(ctx, "/api/sdk/chat/completions", input, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// --- Server-side Conversations (optional persistence) ---

// Conversation represents a server-side conversation container.
type Conversation struct {
	ID           string          `json:"id"`
	AppID      string          `json:"app_id"`
	Title        string          `json:"title"`
	SystemPrompt string          `json:"system_prompt"`
	Model        string          `json:"model"`
	Metadata     json.RawMessage `json:"metadata"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// CreateConversationInput holds parameters for creating a conversation.
type CreateConversationInput struct {
	Title        string          `json:"title,omitempty"`
	SystemPrompt string          `json:"system_prompt,omitempty"`
	Model        string          `json:"model,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
}

// ConversationMessage represents a stored message in a conversation.
type ConversationMessage struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
}

// ConversationDetail includes conversation, messages, and token usage.
type ConversationDetail struct {
	Conversation Conversation          `json:"conversation"`
	Messages     []ConversationMessage `json:"messages"`
	TokenUsage   *ChatCompletionUsage  `json:"token_usage"`
}

// SendMessageInput holds parameters for sending a message.
type SendMessageInput struct {
	Content string `json:"content"`
	UseRAG  bool   `json:"use_rag,omitempty"`
	IndexID string `json:"index_id,omitempty"`
}

// SendMessageResult holds the response from sending a message.
type SendMessageResult struct {
	UserMessage ConversationMessage `json:"user_message"`
	Message     ConversationMessage `json:"message"`
	TokenUsage  ChatCompletionUsage `json:"token_usage"`
}

// CreateConversation creates a server-side conversation.
func (c *Client) CreateConversation(ctx context.Context, input CreateConversationInput) (*Conversation, error) {
	var conv Conversation
	if err := c.post(ctx, "/api/sdk/conversations", input, &conv); err != nil {
		return nil, err
	}
	return &conv, nil
}

// ListConversations returns conversations in the space.
func (c *Client) ListConversations(ctx context.Context, limit, offset int) ([]Conversation, error) {
	if limit <= 0 {
		limit = 50
	}
	var resp struct {
		Conversations []Conversation `json:"conversations"`
	}
	path := fmt.Sprintf("/api/sdk/conversations?limit=%d&offset=%d", limit, offset)
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, err
	}
	return resp.Conversations, nil
}

// GetConversation returns a conversation with its messages and token usage.
func (c *Client) GetConversation(ctx context.Context, id string) (*ConversationDetail, error) {
	var detail ConversationDetail
	if err := c.get(ctx, fmt.Sprintf("/api/sdk/conversations/%s", id), &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

// DeleteConversation deletes a conversation and all its messages.
func (c *Client) DeleteConversation(ctx context.Context, id string) error {
	return c.delete(ctx, fmt.Sprintf("/api/sdk/conversations/%s", id))
}

// SendMessage sends a message in a conversation and returns the AI response.
func (c *Client) SendMessage(ctx context.Context, conversationID string, input SendMessageInput) (*SendMessageResult, error) {
	var result SendMessageResult
	if err := c.post(ctx, fmt.Sprintf("/api/sdk/conversations/%s/messages", conversationID), input, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
