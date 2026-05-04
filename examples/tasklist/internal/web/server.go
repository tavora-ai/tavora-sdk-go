// Package web wires the HTTP surface of the example: UI, JSON API,
// chat SSE passthrough, and the MCP server that exposes task-list tools
// to the agent.
package web

import (
	"embed"
	"html/template"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tavora-ai/tavora-sdk-go/examples/tasklist/internal/store"
	tavora "github.com/tavora-ai/tavora-sdk-go"
)

//go:embed templates/*.html
var templatesFS embed.FS

// Server holds the dependencies shared across handlers.
type Server struct {
	Store        *store.Store
	Tavora       *tavora.Client
	SharedSecret string // Bearer token required on /mcp
	indexTmpl    *template.Template
	mcpServer    *mcp.Server
}

// New builds a Server and parses templates.
func New(s *store.Store, t *tavora.Client, secret string) (*Server, error) {
	tmpl, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Server{
		Store:        s,
		Tavora:       t,
		SharedSecret: secret,
		indexTmpl:    tmpl,
		mcpServer:    buildMCPServer(s),
	}, nil
}

// Routes returns the chi router with all endpoints wired.
func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", s.handleIndex)
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })

	r.Route("/api", func(r chi.Router) {
		r.Get("/lists", s.handleListLists)
		r.Post("/lists", s.handleCreateList)
		r.Delete("/lists/{id}", s.handleDeleteList)
		r.Get("/lists/{id}/tasks", s.handleListTasks)
		r.Post("/lists/{id}/tasks", s.handleAddTask)
		r.Post("/tasks/{id}/complete", s.handleCompleteTask)
		r.Delete("/tasks/{id}", s.handleDeleteTask)
		r.Post("/chat", s.handleChat)
	})

	// MCP endpoint — auth-gated by shared secret matching the auth_config
	// registered with Tavora via CreateMCPServer.
	mcpHandler := mcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *mcp.Server { return s.mcpServer },
		nil,
	)
	r.Handle("/mcp", mcpAuthHandler(s.SharedSecret, mcpHandler))
	r.Handle("/mcp/*", mcpAuthHandler(s.SharedSecret, mcpHandler))

	return r
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.indexTmpl.ExecuteTemplate(w, "index.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
