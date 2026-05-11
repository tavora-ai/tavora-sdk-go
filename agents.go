package tavora

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// AgentSession represents a server-side agent session. Pin fields
// (IndexIDs, MemoryStoreID, SecretVaultID, TenantRef) capture the
// primitive bindings resolved at session-create time; the runtime
// uses these to scope every tool call. See `CreateAgentSessionInput`
// for the input-side shape.
type AgentSession struct {
	ID               string          `json:"id"`
	AppID            string          `json:"app_id"`
	Title            string          `json:"title"`
	SystemPrompt     string          `json:"system_prompt"`
	Model            string          `json:"model"`
	ToolsConfig      json.RawMessage `json:"tools_config"`
	Metadata         json.RawMessage `json:"metadata"`
	Status           string          `json:"status"`
	IndexIDs         []string        `json:"index_ids,omitempty"`
	MemoryStoreID    *string         `json:"memory_store_id,omitempty"`
	SecretVaultID    *string         `json:"secret_vault_id,omitempty"`
	TenantRef        *string         `json:"tenant_ref,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	PromptTokens     int64           `json:"prompt_tokens"`
	CompletionTokens int64           `json:"completion_tokens"`
	StepCount        int32           `json:"step_count"`
	DurationMs       int32           `json:"duration_ms"`
	FinishedAt       *time.Time      `json:"finished_at,omitempty"`
}

// AgentStep represents a step in the agent's reasoning.
type AgentStep struct {
	ID               string          `json:"id"`
	SessionID        string          `json:"session_id"`
	StepIndex        int32           `json:"step_index"`
	StepType         string          `json:"step_type"`
	Content          string          `json:"content"`
	ToolName         string          `json:"tool_name"`
	ToolArgs         json.RawMessage `json:"tool_args"`
	DurationMs       int32           `json:"duration_ms"`
	CreatedAt        time.Time       `json:"created_at"`
	PromptTokens     int32           `json:"prompt_tokens"`
	CompletionTokens int32           `json:"completion_tokens"`
}

// AgentSessionDetail includes session and its steps.
type AgentSessionDetail struct {
	Session AgentSession `json:"session"`
	Steps   []AgentStep  `json:"steps"`
}

// CreateAgentSessionInput holds parameters for creating an agent session.
//
// AgentVersionID, when set, pins the session to an immutable agent version
// — the runtime then resolves persona, skills (filtered by the version's
// skills_json), and stores from that version, ignoring any inline Title /
// SystemPrompt / Tools you also pass. Omit it for an ad-hoc session that
// uses the inline fields directly.
//
// Primitive pinning (composable-primitives plan Stage 4 + Stage 5):
//   • IndexIDs / MemoryStoreID / SecretVaultID — explicit refs; the
//     sandbox scopes each tool to the pinned set. Nil/empty = legacy
//     "no pin" (sandbox `secret()` panics with no vault pinned;
//     `remember()` uses ephemeral per-session memory; `search()` sees
//     every index in the app).
//   • TenantRef — the one-line facade. The platform lazy-resolves
//     per-tenant primitives behind the ref (auto-creates memory_store +
//     secret_vault on first touch, records a tenant_pins row).
//     Explicit refs in the same request override the facade per-field —
//     caller can mix shared app-level indexes with per-tenant memory.
type CreateAgentSessionInput struct {
	AgentVersionID string          `json:"agent_version_id,omitempty"`
	Title          string          `json:"title,omitempty"`
	SystemPrompt   string          `json:"system_prompt,omitempty"`
	Model          string          `json:"model,omitempty"`
	Tools          []string        `json:"tools,omitempty"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`

	IndexIDs      []string `json:"index_ids,omitempty"`
	MemoryStoreID string   `json:"memory_store_id,omitempty"`
	SecretVaultID string   `json:"secret_vault_id,omitempty"`
	TenantRef     string   `json:"tenant_ref,omitempty"`
}

// AgentEvent represents a step event during agent execution. The `Type`
// field is the discriminator — see EventType* constants below for the
// complete set. Most events carry context in `Content` / `Tool` /
// `Args`; terminal events populate `Summary`; input requests stash
// their typed fields in `Args` (use AsInputRequest to extract).
type AgentEvent struct {
	Type    string         `json:"type"`
	Content string         `json:"content,omitempty"`
	Tool    string         `json:"tool,omitempty"`
	Args    map[string]any `json:"args,omitempty"`
	Result  any            `json:"result,omitempty"`
	Summary *RunSummary    `json:"summary,omitempty"`
	// Tokens reports per-step LLM token usage for events emitted by an
	// LLM call (think / respond). Nil for events that don't make an LLM
	// call (tool_call, tool_result, error, input_request, done).
	Tokens *CallTokens `json:"tokens,omitempty"`
}

// CallTokens is per-LLM-call token usage. Mirrors the server's
// internal/agent.CallTokens.
type CallTokens struct {
	Prompt     int32 `json:"prompt"`
	Completion int32 `json:"completion"`
}

// Event-type discriminator values that appear in AgentEvent.Type.
//
// Tavora's reasoning model is "agent writes JavaScript, sandbox runs
// it" rather than "agent calls discrete named tools" — these event
// types reflect that. New types are additive; a consumer that doesn't
// recognize a future type should treat it as an opaque step rather
// than crashing.
const (
	// EventTypeSandboxEvent — partial LLM output (text emitted before
	// a JS block, side-channel logging from the sandbox). Carries
	// Content; Tokens populated when the chunk closes an LLM call.
	EventTypeSandboxEvent = "sandbox_event"
	// EventTypeExecuteJS — the agent emitted a complete JS block; the
	// sandbox is about to run it. Content holds the source.
	EventTypeExecuteJS = "execute_js"
	// EventTypeExecuteJSResult — the sandbox finished running a block.
	// Content holds the formatted result; Result holds the raw value.
	EventTypeExecuteJSResult = "execute_js_result"
	// EventTypeDataUpdate — a sandbox primitive mutated agent-session
	// data; Args holds the changed keys. Useful for live UI sync.
	EventTypeDataUpdate = "data_update"
	// EventTypeResponse — the agent's final natural-language answer.
	// Content holds the response text. Token counts in Tokens.
	EventTypeResponse = "response"
	// EventTypeInputRequest — the agent has paused for user input.
	// Use AsInputRequest to extract the typed request, then call
	// RespondToAgentInput to resume.
	EventTypeInputRequest = "input_request"
	// EventTypeDone — terminal. Summary holds run aggregates.
	EventTypeDone = "done"
	// EventTypeError — terminal. Content holds the error message.
	EventTypeError = "error"
)

// IsTerminal reports whether this event ends the SSE stream — the
// callback won't fire again for this RunAgent / ReplayFromStep call.
func (e AgentEvent) IsTerminal() bool {
	return e.Type == EventTypeDone || e.Type == EventTypeError
}

// InputRequest is the typed extraction of an `input_request` event.
// The agent has paused and is waiting for a response via
// RespondToAgentInput(ctx, sessionID, RequestID, value). Block until
// the user/system supplies a value, then call RespondToAgentInput;
// the SSE stream resumes.
type InputRequest struct {
	RequestID   string
	InputType   string // "confirm" | "choice" | "text"
	Message     string
	Options     []string // populated when InputType == "choice"
	Placeholder string
}

// AsInputRequest extracts the typed InputRequest from an event of
// `Type == EventTypeInputRequest`. Returns nil for any other event so
// callers can write `if req := evt.AsInputRequest(); req != nil { … }`.
func (e AgentEvent) AsInputRequest() *InputRequest {
	if e.Type != EventTypeInputRequest {
		return nil
	}
	req := &InputRequest{Message: e.Content}
	if v, ok := e.Args["request_id"].(string); ok {
		req.RequestID = v
	}
	if v, ok := e.Args["input_type"].(string); ok {
		req.InputType = v
	}
	if v, ok := e.Args["message"].(string); ok && v != "" {
		req.Message = v
	}
	if v, ok := e.Args["placeholder"].(string); ok {
		req.Placeholder = v
	}
	if raw, ok := e.Args["options"].([]any); ok {
		for _, o := range raw {
			if s, ok := o.(string); ok {
				req.Options = append(req.Options, s)
			}
		}
	}
	return req
}

// RunSummary holds aggregate metrics for an agent run.
type RunSummary struct {
	SessionID string `json:"session_id"`
	Steps     int    `json:"steps"`
	Tokens    struct {
		Prompt     int32 `json:"prompt"`
		Completion int32 `json:"completion"`
	} `json:"tokens"`
}

// GetAgentSystemPrompt returns the full system prompt the agent sees for this space.
func (c *Client) GetAgentSystemPrompt(ctx context.Context) (string, error) {
	var resp struct {
		Prompt string `json:"prompt"`
	}
	if err := c.get(ctx, "/api/sdk/agents/system-prompt", &resp); err != nil {
		return "", err
	}
	return resp.Prompt, nil
}

// CreateAgentSession creates a new agent session.
func (c *Client) CreateAgentSession(ctx context.Context, input CreateAgentSessionInput) (*AgentSession, error) {
	var session AgentSession
	if err := c.post(ctx, "/api/sdk/agents", input, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// ListAgentSessions returns agent sessions in the space.
func (c *Client) ListAgentSessions(ctx context.Context, limit, offset int) ([]AgentSession, error) {
	if limit <= 0 {
		limit = 50
	}
	var resp struct {
		Sessions []AgentSession `json:"sessions"`
	}
	if err := c.get(ctx, fmt.Sprintf("/api/sdk/agents?limit=%d&offset=%d", limit, offset), &resp); err != nil {
		return nil, err
	}
	return resp.Sessions, nil
}

// GetAgentSession returns an agent session with its steps.
func (c *Client) GetAgentSession(ctx context.Context, id string) (*AgentSessionDetail, error) {
	var detail AgentSessionDetail
	if err := c.get(ctx, fmt.Sprintf("/api/sdk/agents/%s", id), &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

// DeleteAgentSession deletes an agent session.
func (c *Client) DeleteAgentSession(ctx context.Context, id string) error {
	return c.delete(ctx, fmt.Sprintf("/api/sdk/agents/%s", id))
}

// RunAgent sends a message to an agent session and streams execution events.
func (c *Client) RunAgent(ctx context.Context, sessionID, message string, onEvent func(AgentEvent)) error {
	resp, err := c.resty.R().
		SetContext(ctx).
		SetHeader("Accept", "text/event-stream").
		SetBody(map[string]string{"message": message}).
		SetDoNotParseResponse(true).
		Post(fmt.Sprintf("/api/sdk/agents/%s/run", sessionID))
	if err != nil {
		return fmt.Errorf("tavora: agent run request failed: %w", err)
	}
	defer resp.RawBody().Close()

	if resp.StatusCode() >= 400 {
		body, _ := io.ReadAll(resp.RawBody())
		apiErr := parseAPIError(resp.StatusCode(), body)
		if apiErr.Message == "" {
			apiErr.Message = resp.Status()
		}
		return apiErr
	}

	return parseSSEStream(resp.RawBody(), onEvent)
}

// RespondToAgentInput resolves an input_request event so the paused
// SSE stream can continue. `value` is encoded per InputType:
//   - "confirm" → bool
//   - "choice"  → string (one of the offered Options)
//   - "text"    → string
//
// The server matches the response by RequestID; calling this for an
// already-resolved or unknown RequestID returns 404 / 400.
func (c *Client) RespondToAgentInput(ctx context.Context, sessionID, requestID string, value any) error {
	body := map[string]any{"request_id": requestID, "value": value}
	return c.post(ctx, fmt.Sprintf("/api/sdk/agents/%s/input", sessionID), body, nil)
}

func parseSSEStream(reader io.Reader, onEvent func(AgentEvent)) error {
	scanner := bufio.NewScanner(reader)
	var currentEvent string

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			var evt AgentEvent
			if err := json.Unmarshal([]byte(data), &evt); err != nil {
				continue
			}

			onEvent(evt)

			if currentEvent == "done" || currentEvent == "error" {
				return nil
			}
			currentEvent = ""
		}
	}

	return scanner.Err()
}
