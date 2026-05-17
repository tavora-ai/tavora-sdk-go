package web

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	tavora "github.com/tavora-ai/tavora-sdk-go"
)

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

	// Bind the session to the deployed tasklist agent — its persona,
	// model, and MCP binding (this server's /mcp endpoint) all come
	// from what `tavora deploy` shipped.
	session, err := s.Tavora.CreateAgentSession(r.Context(), tavora.CreateAgentSessionInput{
		AgentID: s.AgentID,
		Title:   truncate("Tasklist: "+in.Message, 80),
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
