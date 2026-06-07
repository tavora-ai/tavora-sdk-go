package scenarios

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Stream writes the scenario's events as a Server-Sent Events response
// to w. The HTTP status (default 200) is written first, then each
// event is delayed and emitted in turn. Returns nil on a clean stream
// or when the context is cancelled (e.g. client disconnect); returns
// an error only if the ResponseWriter itself fails or json encoding
// can't serialise an event payload.
//
// For scenarios with a Status >= 400 and BodyJSON populated, Stream
// writes the single JSON body instead and returns. This is how the
// mock pins error shapes (service-not-configured, etc.) without
// pretending the connection ever became a stream.
func Stream(ctx context.Context, w http.ResponseWriter, s *Scenario) error {
	if s == nil {
		return fmt.Errorf("nil scenario")
	}

	status := s.Status
	if status == 0 {
		status = 200
	}

	if status >= 400 || len(s.Events) == 0 {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(status)
		body := s.BodyJSON
		if body == nil {
			body = map[string]any{"error": "scenario " + s.Name + " has no events"}
		}
		return json.NewEncoder(w).Encode(body)
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("ResponseWriter does not support http.Flusher; SSE requires streaming")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(status)
	flusher.Flush()

	for _, evt := range s.Events {
		if evt.Delay > 0 {
			select {
			case <-time.After(evt.Delay):
			case <-ctx.Done():
				return nil
			}
		}

		payload := evt.Data
		if payload == nil {
			payload = map[string]any{}
		}
		// Mirror the type onto the data payload too. The SDK's
		// parseSSEStream reads `type` off the JSON, so this keeps a
		// scenario file that only sets `type:` at the SSE level still
		// produce events the SDK decodes into AgentEvent.Type. If the
		// scenario already set type in `data:`, leave that alone.
		if _, has := payload["type"]; !has && evt.Type != "" {
			payload["type"] = evt.Type
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode event %q: %w", evt.Type, err)
		}

		if evt.Type != "" {
			if _, err := fmt.Fprintf(w, "event: %s\n", evt.Type); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", raw); err != nil {
			return err
		}
		flusher.Flush()
	}
	return nil
}
