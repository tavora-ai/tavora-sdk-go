package web

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	tavora "github.com/tavora-ai/tavora-sdk-go"
)

const chatSystemPrompt = `You are a task-list assistant. The user runs a small macOS-Reminders-style app.

You can manage task lists and tasks by calling the MCP tools registered with this workspace (create_task_list, list_task_lists, delete_task_list, add_task, list_tasks, complete_task). The tools' schemas tell you their arguments.

Guidelines:
- When the user asks you to create a list of items (e.g. "all large German cities"), first call create_task_list to get a list_id, then call add_task repeatedly for each item.
- If the user refers to an existing list by name, call list_task_lists first to find its id.
- Keep final replies short and concrete ("Created list 'German Cities' with 6 tasks.") — the UI already shows the details.
- Do not invent IDs. Only use IDs returned by tools.`

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if s.Tavora == nil {
		writeError(w, http.StatusServiceUnavailable, "Tavora client not configured")
		return
	}
	var in struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Message == "" {
		writeError(w, http.StatusBadRequest, "missing message")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	sendEvent := func(evt any) {
		b, _ := json.Marshal(evt)
		fmt.Fprintf(w, "data: %s\n\n", b)
		flusher.Flush()
	}

	// No Tools field — the agent auto-loads every enabled MCP server in the
	// workspace (see internal/agent/mcp.go:30 in tavora-go), which includes
	// the tasklist MCP server the example registered on startup.
	session, err := s.Tavora.CreateAgentSession(r.Context(), tavora.CreateAgentSessionInput{
		Title:        truncate("Tasklist: "+in.Message, 80),
		SystemPrompt: chatSystemPrompt,
	})
	if err != nil {
		sendEvent(map[string]string{"type": "error", "content": "create session: " + err.Error()})
		return
	}
	slog.Info("agent session created", "session_id", session.ID)

	err = s.Tavora.RunAgent(r.Context(), session.ID, in.Message, func(evt tavora.AgentEvent) {
		sendEvent(evt)
	})
	if err != nil {
		sendEvent(map[string]string{"type": "error", "content": "run: " + err.Error()})
		return
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
