// Package middleware holds HTTP middlewares shared between the
// `tavora-mock` (Tavora API stub) and `tavora-fake-backend` (user
// backend stub) tools. Both forks of json-server-go in this repo need
// the same request-id, structured logger, and panic recoverer, and
// both want to stamp a self-identifying response header so a misrouted
// production client refuses to talk to them.
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

type ctxRequestIDKey struct{}

// SelfIDHeader returns a middleware that stamps a fixed header on
// every response. Used by tavora-mock to emit `X-Tavora-Mock: true`
// and by tavora-fake-backend to emit `X-Tavora-Fake-Backend: true`.
// The constants live in each tool's package so the literal value is
// grep-able from there.
func SelfIDHeader(name, value string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(name, value)
			next.ServeHTTP(w, r)
		})
	}
}

// RequestID attaches a request id (or honors an inbound X-Request-Id
// header) and echoes it on the response.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set("X-Request-Id", id)
		ctx := context.WithValue(r.Context(), ctxRequestIDKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFrom returns the id attached by RequestID, or "".
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(ctxRequestIDKey{}).(string)
	return id
}

// Logger logs one structured line per request.
func Logger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &statusWriter{ResponseWriter: w, status: 200}
			next.ServeHTTP(ww, r)
			log.Info("http",
				"id", RequestIDFrom(r.Context()),
				"method", r.Method,
				"path", r.URL.Path,
				"query", r.URL.RawQuery,
				"status", ww.status,
				"bytes", ww.bytes,
				"remote", r.RemoteAddr,
				"ua", r.UserAgent(),
				"dur_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}

// Recoverer catches panics, logs them, and returns 500.
func Recoverer(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic",
						"id", RequestIDFrom(r.Context()),
						"path", r.URL.Path,
						"err", rec,
						"stack", string(debug.Stack()),
					)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"error":"internal server error","status":500}`))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	w.bytes += n
	return n, err
}

// Flush delegates so downstream SSE / streaming middleware can flush
// through us.
func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func newRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "req-unknown"
	}
	return hex.EncodeToString(b)
}
