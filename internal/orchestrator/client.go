// Package orchestrator provides an HTTP client for the Python LangGraph service.
package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// AskRequest is the JSON body sent to POST /ask.
type AskRequest struct {
	Question string `json:"question"`
	Depth    int    `json:"depth"`
}

// AskResponse is the JSON body received from POST /ask.
type AskResponse struct {
	Summary         string   `json:"summary"`
	CallPath        []string `json:"call_path"`
	RelevantFiles   []string `json:"relevant_files"`
	EntryPointCount int      `json:"entry_point_count"`
	GraphNodeCount  int      `json:"graph_node_count"`
}

// Client calls the Python orchestrator service over HTTP.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient returns a Client that targets baseURL (e.g. "http://localhost:8000").
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Ping calls GET /healthz. Returns nil when the service is healthy.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/healthz", nil)
	if err != nil {
		return fmt.Errorf("orchestrator: ping: build request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("orchestrator: ping: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("orchestrator: ping: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// Ask sends POST /ask and returns the structured response.
func (c *Client) Ask(ctx context.Context, req AskRequest) (*AskResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: ask: marshal request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/ask", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("orchestrator: ask: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: ask: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("orchestrator: ask: server returned status %d", resp.StatusCode)
	}
	var askResp AskResponse
	if err := json.NewDecoder(resp.Body).Decode(&askResp); err != nil {
		return nil, fmt.Errorf("orchestrator: ask: decode response: %w", err)
	}
	return &askResp, nil
}
