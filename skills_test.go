package tavora

import (
	"context"
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

// CreateSkill / DeleteSkill tests were removed 2026-05-17 when the
// write surface collapsed into the code-first source-sync path.
// The SDK no longer exposes mutating skill calls; coverage for
// source-sync skill upserts lives in tavora-go's source_test.go.
