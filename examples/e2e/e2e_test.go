// Package e2e runs end-to-end tests for the Tavora example apps using testscript.
//
// These tests require a running Tavora instance. Set TAVORA_URL and TAVORA_API_KEY
// to a dedicated test space. Tests are skipped if these are not set.
//
// Run:
//
//	TAVORA_URL=http://localhost:8080 TAVORA_API_KEY=tvr_... go test -v -timeout 10m
package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/rogpeppe/go-internal/testscript"
)

var examplesDir string

func TestMain(m *testing.M) {
	// Resolve the examples root relative to this file's location.
	// When running from examples/e2e/, the other examples are siblings.
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	examplesDir = filepath.Dir(wd)

	os.Exit(m.Run())
}

// buildExample compiles an example app and returns the path to the binary.
func buildExample(t *testing.T, name string) string {
	t.Helper()
	srcDir := filepath.Join(examplesDir, name)
	bin := filepath.Join(t.TempDir(), name)
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = srcDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("building %s: %v\n%s", name, err, out)
	}
	return bin
}

func TestKnowledgeBase(t *testing.T) {
	runExample(t, "knowledge-base", 5*time.Minute)
}

func TestSupportBot(t *testing.T) {
	runExample(t, "support-bot", 5*time.Minute)
}

func TestResearchAssistant(t *testing.T) {
	runExample(t, "research-assistant", 5*time.Minute)
}

func runExample(t *testing.T, name string, timeout time.Duration) {
	t.Helper()
	requireEnv(t)
	bin := buildExample(t, name)

	testscript.Run(t, testscript.Params{
		Dir:      filepath.Join("testdata", name),
		TestWork: testing.Verbose(),
		Deadline: time.Now().Add(timeout),
		Setup: func(env *testscript.Env) error {
			binDir := filepath.Dir(bin)
			env.Vars = append(env.Vars,
				"TAVORA_URL="+os.Getenv("TAVORA_URL"),
				"TAVORA_API_KEY="+os.Getenv("TAVORA_API_KEY"),
				"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
			)
			return nil
		},
	})
}

func requireEnv(t *testing.T) {
	t.Helper()
	if os.Getenv("TAVORA_URL") == "" || os.Getenv("TAVORA_API_KEY") == "" {
		t.Skip("TAVORA_URL and TAVORA_API_KEY required for E2E tests")
	}
}
