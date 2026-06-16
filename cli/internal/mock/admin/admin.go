// Package admin holds the /_admin/* endpoints. Reset restores the
// seed snapshot the store captured at startup; Snapshot dumps current
// state as JSON. Both are useful for autonomous test loops that want
// to run several scenarios back-to-back without restarting the binary.
package admin

import (
	"encoding/json"
	"net/http"

	"github.com/tavora-ai/tavora-sdk-go/cli/internal/mock/store"
)

// Handler bundles the admin endpoints. The mock keeps these unauth'd
// — the binary is dev-only and lives on a localhost-bound port.
type Handler struct {
	Store *store.Store
}

func New(s *store.Store) *Handler { return &Handler{Store: s} }

// Reset restores the store's seed snapshot. Always returns 200.
func (h *Handler) Reset(w http.ResponseWriter, _ *http.Request) {
	if err := h.Store.Reset(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "status": 500})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "action": "reset"})
}

// Snapshot dumps the current store state as JSON.
func (h *Handler) Snapshot(w http.ResponseWriter, _ *http.Request) {
	raw, err := h.Store.Snapshot()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "status": 500})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
