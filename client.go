// Package tavora provides a Go SDK for the Tavora API.
//
// All SDK operations are scoped to a space via the API key used to create the client.
// API keys are created in the admin UI under Workspace Settings.
package tavora

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/go-resty/resty/v2"
)

// Client is a Tavora API client scoped to a single space via API key.
type Client struct {
	resty *resty.Client
}

// Option configures the Client.
type Option func(*resty.Client)

// WithHTTPClient sets a custom underlying HTTP client.
func WithHTTPClient(c *http.Client) Option {
	return func(r *resty.Client) {
		r.SetTransport(c.Transport)
	}
}

// WithDebug enables resty debug logging.
func WithDebug() Option {
	return func(r *resty.Client) {
		r.SetDebug(true)
	}
}

// NewClient creates a new Tavora SDK client.
//
// baseURL is the Tavora API server (e.g. "https://api.tavora.ai").
// apiKey is a space-scoped API key (starts with "tvr_"), created via the admin UI.
func NewClient(baseURL, apiKey string, opts ...Option) *Client {
	r := resty.New().
		SetBaseURL(baseURL).
		SetHeader("X-API-Key", apiKey).
		SetHeader("Content-Type", "application/json")

	for _, opt := range opts {
		opt(r)
	}

	return &Client{resty: r}
}

func (c *Client) get(ctx context.Context, path string, result interface{}) error {
	req := c.resty.R().SetContext(ctx)
	if result != nil {
		req.SetResult(result)
	}
	resp, err := req.Get(path)
	if err != nil {
		return fmt.Errorf("tavora: request failed: %w", err)
	}
	return checkError(resp)
}

func (c *Client) post(ctx context.Context, path string, body, result interface{}) error {
	req := c.resty.R().SetContext(ctx)
	if body != nil {
		req.SetBody(body)
	}
	if result != nil {
		req.SetResult(result)
	}
	resp, err := req.Post(path)
	if err != nil {
		return fmt.Errorf("tavora: request failed: %w", err)
	}
	return checkError(resp)
}

func (c *Client) patch(ctx context.Context, path string, body, result interface{}) error {
	req := c.resty.R().SetContext(ctx)
	if body != nil {
		req.SetBody(body)
	}
	if result != nil {
		req.SetResult(result)
	}
	resp, err := req.Patch(path)
	if err != nil {
		return fmt.Errorf("tavora: request failed: %w", err)
	}
	return checkError(resp)
}

func (c *Client) put(ctx context.Context, path string, body, result interface{}) error {
	req := c.resty.R().SetContext(ctx)
	if body != nil {
		req.SetBody(body)
	}
	if result != nil {
		req.SetResult(result)
	}
	resp, err := req.Put(path)
	if err != nil {
		return fmt.Errorf("tavora: request failed: %w", err)
	}
	return checkError(resp)
}

func (c *Client) delete(ctx context.Context, path string) error {
	resp, err := c.resty.R().
		SetContext(ctx).
		Delete(path)
	if err != nil {
		return fmt.Errorf("tavora: request failed: %w", err)
	}
	return checkError(resp)
}

func (c *Client) upload(ctx context.Context, path, filePath string, fields map[string]string, result interface{}) error {
	req := c.resty.R().
		SetContext(ctx).
		SetFile("file", filePath)
	for k, v := range fields {
		req.SetFormData(map[string]string{k: v})
	}
	if result != nil {
		req.SetResult(result)
	}
	// Remove Content-Type so resty sets multipart boundary automatically
	req.SetHeader("Content-Type", "")
	resp, err := req.Post(path)
	if err != nil {
		return fmt.Errorf("tavora: upload failed: %w", err)
	}
	return checkError(resp)
}

func (c *Client) uploadReader(ctx context.Context, path, filename string, reader io.Reader, fields map[string]string, result interface{}) error {
	req := c.resty.R().
		SetContext(ctx).
		SetMultipartField("file", filename, "application/octet-stream", reader)
	for k, v := range fields {
		req.SetFormData(map[string]string{k: v})
	}
	if result != nil {
		req.SetResult(result)
	}
	req.SetHeader("Content-Type", "")
	resp, err := req.Post(path)
	if err != nil {
		return fmt.Errorf("tavora: upload failed: %w", err)
	}
	return checkError(resp)
}

func checkError(resp *resty.Response) error {
	if resp.StatusCode() >= 400 {
		apiErr := parseAPIError(resp.StatusCode(), resp.Body())
		if apiErr.Message == "" {
			apiErr.Message = resp.Status()
		}
		return apiErr
	}
	return nil
}
