package tavora

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestListPromptTemplates(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/prompt-templates", 200, map[string]interface{}{
		"templates": []PromptTemplate{
			{ID: "pt_1", Name: "support-bot", Content: "You are a helpful support agent."},
		},
	})

	templates, err := ts.client().ListPromptTemplates(context.Background())
	assertNoError(t, err)
	assertEqual(t, "count", len(templates), 1)
	assertEqual(t, "name", templates[0].Name, "support-bot")
}

func TestGetPromptTemplate(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/prompt-templates/pt_1", 200, PromptTemplate{
		ID:      "pt_1",
		Name:    "support-bot",
		Content: "You are a helpful support agent.",
	})

	tmpl, err := ts.client().GetPromptTemplate(context.Background(), "pt_1")
	assertNoError(t, err)
	assertEqual(t, "id", tmpl.ID, "pt_1")
	assertEqual(t, "content", tmpl.Content, "You are a helpful support agent.")
}

func TestCreatePromptTemplate(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/prompt-templates", 201, PromptTemplate{
		ID:   "pt_new",
		Name: "sales-bot",
	})

	tmpl, err := ts.client().CreatePromptTemplate(context.Background(), CreatePromptTemplateInput{
		Name:      "sales-bot",
		Content:   "You help with sales.",
		Variables: []string{"app_name"},
	})
	assertNoError(t, err)
	assertEqual(t, "id", tmpl.ID, "pt_new")

	req := ts.lastRequest(t)
	var body CreatePromptTemplateInput
	json.Unmarshal([]byte(req.Body), &body)
	assertEqual(t, "name", body.Name, "sales-bot")
	assertEqual(t, "vars count", len(body.Variables), 1)
}

func TestUpdatePromptTemplate(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPatch, "/api/sdk/prompt-templates/pt_1", 200, PromptTemplate{
		ID:      "pt_1",
		Name:    "updated-bot",
		Content: "Updated content.",
	})

	tmpl, err := ts.client().UpdatePromptTemplate(context.Background(), "pt_1", UpdatePromptTemplateInput{
		Name:    "updated-bot",
		Content: "Updated content.",
	})
	assertNoError(t, err)
	assertEqual(t, "name", tmpl.Name, "updated-bot")

	req := ts.lastRequest(t)
	assertEqual(t, "method", req.Method, http.MethodPatch)
}

func TestDeletePromptTemplate(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodDelete, "/api/sdk/prompt-templates/pt_1", 204, nil)

	err := ts.client().DeletePromptTemplate(context.Background(), "pt_1")
	assertNoError(t, err)
}
