package tavora

import (
	"context"
	"fmt"
)

// Asset is the metadata for an agent-generated artifact persisted
// by the sandbox primitive `asset.write()`. Bytes are not inline —
// fetch them with GetAgentAsset.
type Asset struct {
	ID        string `json:"id"`
	SessionID string `json:"session_id"`
	Name      string `json:"name"`
	Mime      string `json:"mime"`
	Size      int64  `json:"size"`
	DiskPath  string `json:"disk_path"`
}

// ListSessionAssets returns every asset attached to a session in
// arrival order. Empty slice (not error) when the session ran
// without writing any assets.
//
// Endpoint: GET /api/sdk/agents/sessions/:id/assets
func (c *Client) ListSessionAssets(ctx context.Context, sessionID string) ([]Asset, error) {
	var resp struct {
		Assets []Asset `json:"assets"`
	}
	if err := c.get(ctx, fmt.Sprintf("/api/sdk/agents/sessions/%s/assets", sessionID), &resp); err != nil {
		return nil, err
	}
	return resp.Assets, nil
}

// GetAgentAsset fetches an asset's bytes plus its content-type. Use
// this when the consumer wants the raw payload (write to disk, push
// to a downstream service, etc.). Pair with ListSessionAssets to
// enumerate first, then fetch.
//
// Endpoint: GET /api/sdk/assets/:id
func (c *Client) GetAgentAsset(ctx context.Context, assetID string) ([]byte, string, error) {
	resp, err := c.resty.R().SetContext(ctx).Get(fmt.Sprintf("/api/sdk/assets/%s", assetID))
	if err != nil {
		return nil, "", fmt.Errorf("tavora: request failed: %w", err)
	}
	if resp.StatusCode() >= 400 {
		return nil, "", parseAPIError(resp.StatusCode(), resp.Body())
	}
	return resp.Body(), resp.Header().Get("Content-Type"), nil
}
