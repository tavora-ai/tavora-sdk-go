package tavora

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestChatCompletion(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/chat/completions", 200, ChatCompletionResult{
		ID:    "cmpl_1",
		Model: "gemini-2.5-flash",
		Choices: []ChatCompletionChoice{
			{Index: 0, Message: ChatMessage{Role: "assistant", Content: "Hello!"}, FinishReason: "stop"},
		},
		Usage: ChatCompletionUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	})

	result, err := ts.client().ChatCompletion(context.Background(), ChatCompletionInput{
		Messages: []ChatMessage{{Role: "user", Content: "Hi"}},
	})
	assertNoError(t, err)
	assertEqual(t, "id", result.ID, "cmpl_1")
	assertEqual(t, "choices", len(result.Choices), 1)
	assertEqual(t, "content", result.Choices[0].Message.Content, "Hello!")
	assertEqual(t, "total_tokens", result.Usage.TotalTokens, int32(15))

	req := ts.lastRequest(t)
	assertEqual(t, "method", req.Method, http.MethodPost)
	assertEqual(t, "path", req.Path, "/api/sdk/chat/completions")
	var body ChatCompletionInput
	json.Unmarshal([]byte(req.Body), &body)
	assertEqual(t, "message role", body.Messages[0].Role, "user")
	assertEqual(t, "message content", body.Messages[0].Content, "Hi")
}

func TestChatCompletion_WithRAG(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/chat/completions", 200, ChatCompletionResult{
		ID: "cmpl_2",
	})

	_, err := ts.client().ChatCompletion(context.Background(), ChatCompletionInput{
		Messages: []ChatMessage{{Role: "user", Content: "What's in the docs?"}},
		UseRAG:   true,
		IndexID:  "st_1",
	})
	assertNoError(t, err)

	req := ts.lastRequest(t)
	var body ChatCompletionInput
	json.Unmarshal([]byte(req.Body), &body)
	assertEqual(t, "use_rag", body.UseRAG, true)
	assertEqual(t, "index_id", body.IndexID, "st_1")
}

func TestCreateConversation(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/conversations", 201, Conversation{
		ID:    "conv_1",
		Title: "Test Chat",
	})

	conv, err := ts.client().CreateConversation(context.Background(), CreateConversationInput{
		Title: "Test Chat",
	})
	assertNoError(t, err)
	assertEqual(t, "id", conv.ID, "conv_1")
	assertEqual(t, "title", conv.Title, "Test Chat")
}

func TestListConversations(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/conversations", 200, map[string]interface{}{
		"conversations": []Conversation{
			{ID: "conv_1", Title: "Chat 1"},
			{ID: "conv_2", Title: "Chat 2"},
		},
	})

	convs, err := ts.client().ListConversations(context.Background(), 10, 0)
	assertNoError(t, err)
	assertEqual(t, "count", len(convs), 2)

	req := ts.lastRequest(t)
	assertEqual(t, "path", req.Path, "/api/sdk/conversations?limit=10&offset=0")
}

func TestListConversations_DefaultLimit(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/conversations", 200, map[string]interface{}{
		"conversations": []Conversation{},
	})

	_, err := ts.client().ListConversations(context.Background(), 0, 0)
	assertNoError(t, err)

	req := ts.lastRequest(t)
	assertEqual(t, "path", req.Path, "/api/sdk/conversations?limit=50&offset=0")
}

func TestGetConversation(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/conversations/conv_1", 200, ConversationDetail{
		Conversation: Conversation{ID: "conv_1", Title: "Test"},
		Messages: []ConversationMessage{
			{ID: "msg_1", Role: "user", Content: "Hello"},
			{ID: "msg_2", Role: "assistant", Content: "Hi there"},
		},
	})

	detail, err := ts.client().GetConversation(context.Background(), "conv_1")
	assertNoError(t, err)
	assertEqual(t, "id", detail.Conversation.ID, "conv_1")
	assertEqual(t, "messages", len(detail.Messages), 2)
}

func TestDeleteConversation(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodDelete, "/api/sdk/conversations/conv_1", 204, nil)

	err := ts.client().DeleteConversation(context.Background(), "conv_1")
	assertNoError(t, err)

	req := ts.lastRequest(t)
	assertEqual(t, "path", req.Path, "/api/sdk/conversations/conv_1")
}

func TestSendMessage(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/conversations/conv_1/messages", 200, SendMessageResult{
		UserMessage: ConversationMessage{ID: "msg_1", Role: "user", Content: "How are you?"},
		Message:     ConversationMessage{ID: "msg_2", Role: "assistant", Content: "I'm well!"},
		TokenUsage:  ChatCompletionUsage{TotalTokens: 20},
	})

	result, err := ts.client().SendMessage(context.Background(), "conv_1", SendMessageInput{
		Content: "How are you?",
	})
	assertNoError(t, err)
	assertEqual(t, "user msg", result.UserMessage.Content, "How are you?")
	assertEqual(t, "ai msg", result.Message.Content, "I'm well!")
	assertEqual(t, "tokens", result.TokenUsage.TotalTokens, int32(20))

	req := ts.lastRequest(t)
	assertEqual(t, "path", req.Path, "/api/sdk/conversations/conv_1/messages")
	var body SendMessageInput
	json.Unmarshal([]byte(req.Body), &body)
	assertEqual(t, "content", body.Content, "How are you?")
}
