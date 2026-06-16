package tavora

import "context"

// Capability is one row of the server's capability catalog — the
// catalog of JS-surface primitives every agent can opt into via
// agent.jsonc's `capabilities` array. The CLI uses this list to
// power its "unknown capability" linter so the check tracks the
// server's live registry instead of a hand-kept constant.
type Capability struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// listCapabilitiesResponse mirrors the server's wire shape.
type listCapabilitiesResponse struct {
	Capabilities []Capability `json:"capabilities"`
}

// ListCapabilities returns the static catalog of agent capabilities
// (JS primitives) registered on the server. Safe to call on every
// startup — cheap, no auth except the project-scoped API key.
func (c *Client) ListCapabilities(ctx context.Context) ([]Capability, error) {
	var resp listCapabilitiesResponse
	if err := c.get(ctx, "/api/sdk/capabilities", &resp); err != nil {
		return nil, err
	}
	return resp.Capabilities, nil
}
