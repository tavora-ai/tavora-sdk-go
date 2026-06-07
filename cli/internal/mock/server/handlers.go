package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	mockauth "github.com/tavora-ai/tavora-sdk-go/cli/internal/mock/auth"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/mock/canned"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/mock/ids"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/mock/scenarios"
	"github.com/tavora-ai/tavora-sdk-go/cli/internal/mock/store"
)

type handlers struct {
	store           *store.Store
	auth            *mockauth.Manager
	scenarios       *scenarios.Registry
	ids             *ids.Source
	defaultScenario string
}

func newHandlers(s *store.Store, a *mockauth.Manager, sc *scenarios.Registry, id *ids.Source, defScenario string) *handlers {
	if defScenario == "" {
		defScenario = scenarios.DefaultName
	}
	return &handlers{store: s, auth: a, scenarios: sc, ids: id, defaultScenario: defScenario}
}

// ---------- liveness / version ----------

func (h *handlers) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (h *handlers) Version(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"mock":    true,
		"version": "tavora-mock/0.1",
	})
}

// ---------- auth ----------

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Username == "" {
		req.Username = "mock-user"
	}
	tok, exp, err := h.auth.Issue(req.Username, []string{"user"})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token":      tok,
		"token_type": "Bearer",
		"expires_at": exp.UTC().Format(time.RFC3339),
		"username":   req.Username,
		"roles":      []string{"user"},
	})
}

// ---------- project + capabilities ----------

func (h *handlers) GetProject(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, canned.Project())
}

func (h *handlers) SeedProject(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"already_seeded": false,
		"agent_id":       canned.AgentIDFor("default"),
		"agent_name":     "default",
	})
}

func (h *handlers) ListCapabilities(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, canned.Capabilities())
}

// ---------- source-* ----------

// sourceSyncManifest is a partial decode of the body — just enough to
// pull the project name + per-agent local ids out so the response can
// echo the local→server mapping.
type sourceSyncManifest struct {
	Project    string `json:"project"`
	SourceHash string `json:"sourceHash"`
	Agents     []struct {
		ID         string `json:"id"`
		SourceHash string `json:"sourceHash"`
	} `json:"agents"`
}

func (h *handlers) SourceSync(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var m sourceSyncManifest
	if err := json.Unmarshal(body, &m); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body: "+err.Error())
		return
	}
	localIDs := localIDsFrom(m)
	hash := m.SourceHash
	if hash == "" {
		hash = "sha256:" + sha256Hex(body)
	}
	writeJSON(w, http.StatusOK, canned.SourceSyncResult(m.Project, hash, localIDs))
}

func (h *handlers) SourceValidate(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, canned.SourceValidateResult())
}

type sourceDeployInput struct {
	Project      string `json:"project"`
	LocalAgentID string `json:"localAgentId"`
}

func (h *handlers) SourceDeploy(w http.ResponseWriter, r *http.Request) {
	var in sourceDeployInput
	_ = json.NewDecoder(r.Body).Decode(&in)
	locals := []string{"default"}
	if in.LocalAgentID != "" {
		locals = []string{in.LocalAgentID}
	}
	writeJSON(w, http.StatusOK, canned.SourceDeployResult(in.Project, locals))
}

func (h *handlers) SourceExport(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	if project == "" {
		project = "mock-project"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"project": project,
		"agents":  []any{},
	})
}

type renameInput struct {
	Project    string `json:"project"`
	OldLocalID string `json:"oldLocalId"`
	NewLocalID string `json:"newLocalId"`
}

func (h *handlers) SourceRename(w http.ResponseWriter, r *http.Request) {
	var in renameInput
	_ = json.NewDecoder(r.Body).Decode(&in)
	writeJSON(w, http.StatusOK, map[string]any{
		"agentId":    canned.AgentIDFor(in.NewLocalID),
		"oldLocalId": in.OldLocalID,
		"newLocalId": in.NewLocalID,
	})
}

type deleteInput struct {
	Project string `json:"project"`
	LocalID string `json:"localId"`
	Force   bool   `json:"force"`
}

func (h *handlers) SourceDelete(w http.ResponseWriter, r *http.Request) {
	var in deleteInput
	_ = json.NewDecoder(r.Body).Decode(&in)
	if !in.Force {
		writeStructuredError(w, http.StatusBadRequest, "force_required", "set force=true to delete an agent")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"agentId": canned.AgentIDFor(in.LocalID),
		"localId": in.LocalID,
		"deleted": true,
	})
}

func (h *handlers) SourceDiff(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"inSync": true,
		"paths":  []any{},
	})
}

// ---------- deployment env ----------

func (h *handlers) ListDeploymentEnv(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	keys := h.store.ListEnv(slug)
	out := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		out = append(out, map[string]any{
			"deployment_id": "mock-deployment-" + slug,
			"key":           k.Key,
			"is_secret":     k.IsSecret,
			"kek_id":        "mock-kek",
			"created_at":    canned.FixedNow,
			"updated_at":    canned.FixedNow,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"env": out})
}

func (h *handlers) GetDeploymentEnv(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	key := chi.URLParam(r, "key")
	value, isSecret, ok := h.store.GetEnv(slug, key)
	if !ok {
		writeStructuredError(w, http.StatusNotFound, "env_not_found", "no env entry for "+slug+"/"+key)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"key":       key,
		"value":     value,
		"is_secret": isSecret,
	})
}

type putEnvInput struct {
	Value    string `json:"value"`
	IsSecret bool   `json:"is_secret"`
}

func (h *handlers) PutDeploymentEnv(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	key := chi.URLParam(r, "key")
	var in putEnvInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body: "+err.Error())
		return
	}
	h.store.PutEnv(slug, key, in.Value, in.IsSecret)
	writeJSON(w, http.StatusOK, map[string]any{
		"deployment_id": "mock-deployment-" + slug,
		"key":           key,
		"is_secret":     in.IsSecret,
		"kek_id":        "mock-kek",
		"created_at":    canned.FixedNow,
		"updated_at":    canned.FixedNow,
	})
}

func (h *handlers) DeleteDeploymentEnv(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	key := chi.URLParam(r, "key")
	h.store.DeleteEnv(slug, key) // idempotent; missing key still 204
	w.WriteHeader(http.StatusNoContent)
}

// ---------- agent sessions ----------

func (h *handlers) CreateAgentSession(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	title, _ := body["title"].(string)
	model, _ := body["model"].(string)
	var indexIDs []string
	if raw, ok := body["index_ids"].([]any); ok {
		for _, v := range raw {
			if s, ok := v.(string); ok {
				indexIDs = append(indexIDs, s)
			}
		}
	}
	id := h.ids.SessionID()
	session := canned.AgentSession(id, title, model, indexIDs)
	h.store.PutSession(id, session)
	writeJSON(w, http.StatusCreated, session)
}

func (h *handlers) ListAgentSessions(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"sessions": []any{}})
}

func (h *handlers) GetAgentSession(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	s := h.store.GetSession(id)
	if s == nil {
		// Synthesize a default so an unrecognised id still parses on the
		// SDK side. Real server returns 404 in this case; the mock
		// leans toward "never fail in canned-mode" for now.
		s = canned.AgentSession(id, "", "", nil)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"session": s,
		"steps":   []any{},
	})
}

func (h *handlers) DeleteAgentSession(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// RunAgent streams the active scenario's canned event sequence as SSE.
// Scenario selection precedence: `?scenario=<name>` query param >
// per-process default (set via -scenario flag) > scenarios.DefaultName.
// Unknown names return 400 with a structured `unknown_scenario` body
// so the caller sees the typo immediately.
func (h *handlers) RunAgent(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("scenario")
	if name == "" {
		name = h.defaultScenario
	}
	s := h.scenarios.Get(name)
	if s == nil {
		writeStructuredError(w, http.StatusBadRequest, "unknown_scenario",
			fmt.Sprintf("scenario %q not registered (try one of %v)", name, h.scenarios.Names()))
		return
	}
	if err := scenarios.Stream(r.Context(), w, s); err != nil {
		// Stream already wrote the status; logging here is the best we
		// can do — surface it so a `task mock:run` operator sees the
		// failure.
		_ = err
	}
}

func (h *handlers) RespondAgentInput(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *handlers) GetAgentSystemPrompt(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"prompt": "You are a mock agent. tavora-mock is serving this prompt.",
	})
}

// ---------- not-found ----------

func (h *handlers) NotFound(w http.ResponseWriter, r *http.Request) {
	writeStructuredError(w, http.StatusNotImplemented, "endpoint_not_mocked",
		fmt.Sprintf("%s %s is not implemented by tavora-mock", r.Method, r.URL.Path))
}

// ListScenarios returns the registered scenario names. Useful for
// `task mock:run -- -help` debugging and for the SDK contract tests
// that want to enumerate available scenarios.
func (h *handlers) ListScenarios(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"default":   h.defaultScenario,
		"scenarios": h.scenarios.Names(),
	})
}

// ---------- helpers ----------

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg, "status": status})
}

func writeStructuredError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{
		"error":   msg,
		"code":    code,
		"status":  status,
		"message": msg,
	})
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func localIDsFrom(m sourceSyncManifest) []string {
	if len(m.Agents) == 0 {
		return []string{"default"}
	}
	out := make([]string, 0, len(m.Agents))
	for _, a := range m.Agents {
		if a.ID != "" {
			out = append(out, a.ID)
		}
	}
	if len(out) == 0 {
		return []string{"default"}
	}
	return out
}

