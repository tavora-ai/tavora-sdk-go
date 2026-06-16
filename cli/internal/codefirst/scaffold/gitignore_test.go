package scaffold_test

import (
	"strings"
	"testing"

	"github.com/tavora-ai/tavora-sdk-go/cli/internal/codefirst/scaffold"
)

// TestScaffoldGitignoreContainsFakebackEntries pins that `tavora
// init` ships a .gitignore that hides the fake-backend's
// per-developer artifacts (request log, recorded captures). Without
// these entries, the first `tavora dev` run would land an
// unredacted requests.jsonl in the next git commit — the user's
// bearer-shaped headers + agent traffic in plain text.
func TestScaffoldGitignoreContainsFakebackEntries(t *testing.T) {
	files := scaffold.Plan(scaffold.Options{ProjectName: "test"})
	var gitignore string
	for _, f := range files {
		if strings.HasSuffix(f.RelPath, ".gitignore") {
			gitignore = f.Body
			break
		}
	}
	if gitignore == "" {
		t.Fatal("scaffold.Plan did not produce a .gitignore")
	}
	wantPatterns := []string{
		"mocks/*/requests.jsonl",
		"mocks/*/recorded.json",
	}
	for _, want := range wantPatterns {
		if !strings.Contains(gitignore, want) {
			t.Errorf(".gitignore missing pattern %q:\n%s", want, gitignore)
		}
	}
}
