package tavora

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestListSkills(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/skills", 200, map[string]interface{}{
		"skills": []Skill{
			{ID: "sk_1", Name: "summarize", Type: "prompt"},
			{ID: "sk_2", Name: "translate", Type: "webhook"},
		},
	})

	skills, err := ts.client().ListSkills(context.Background())
	assertNoError(t, err)
	assertEqual(t, "count", len(skills), 2)
	assertEqual(t, "first name", skills[0].Name, "summarize")
}

func TestCreateSkill(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodPost, "/api/sdk/skills", 201, Skill{
		ID:   "sk_new",
		Name: "summarize",
		Type: "prompt",
	})

	skill, err := ts.client().CreateSkill(context.Background(), CreateSkillInput{
		Name:        "summarize",
		Description: "Summarize text",
		Type:        "prompt",
		Prompt:      "Summarize the following: {{input}}",
	})
	assertNoError(t, err)
	assertEqual(t, "id", skill.ID, "sk_new")

	req := ts.lastRequest(t)
	var body CreateSkillInput
	json.Unmarshal([]byte(req.Body), &body)
	assertEqual(t, "name", body.Name, "summarize")
	assertEqual(t, "type", body.Type, "prompt")
}

func TestGetSkill(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodGet, "/api/sdk/skills/sk_1", 200, Skill{
		ID:   "sk_1",
		Name: "summarize",
	})

	skill, err := ts.client().GetSkill(context.Background(), "sk_1")
	assertNoError(t, err)
	assertEqual(t, "id", skill.ID, "sk_1")
}

func TestDeleteSkill(t *testing.T) {
	ts := newTestServer(t)
	ts.on(http.MethodDelete, "/api/sdk/skills/sk_1", 204, nil)

	err := ts.client().DeleteSkill(context.Background(), "sk_1")
	assertNoError(t, err)
}
