// Package lineargql implements the §10.5 linear_graphql client-side agent tool.
// It forwards raw GraphQL queries from the agent to the Linear API and maps
// responses to the §10.5 success/errors shape, keeping the API key out of logs.
package lineargql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const defaultTimeout = 30 * time.Second

// Result holds the §10.5 output for a linear_graphql tool invocation.
type Result struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Errors  json.RawMessage `json:"errors,omitempty"`
}

// Client forwards raw GraphQL queries to the Linear API on the agent's behalf.
// It is distinct from the tracker adapter (platform/tracker/linear) which is
// dispatch-specific; this client is a general passthrough per SPEC §10.5.
type Client struct {
	endpoint string
	apiKey   string
	http     *http.Client
}

// New returns a Client for the given Linear endpoint and API key.
// The key is transmitted only in the Authorization header and is never logged.
func New(endpoint, apiKey string) *Client {
	return newClient(endpoint, apiKey, &http.Client{Timeout: defaultTimeout})
}

func newClient(endpoint, apiKey string, hc *http.Client) *Client {
	return &Client{endpoint: endpoint, apiKey: apiKey, http: hc}
}

// Execute sends a raw GraphQL query and variables to Linear and returns a §10.5
// Result. Invalid input and auth failures are encoded in Result.Success=false;
// only unexpected encoding errors are returned as a Go error.
func (c *Client) Execute(ctx context.Context, query string, variables json.RawMessage) (*Result, error) {
	if query == "" {
		return errResult("query must not be empty"), nil
	}
	if c.apiKey == "" {
		return errResult("linear API key not configured"), nil
	}
	if len(variables) == 0 {
		variables = json.RawMessage("null")
	}

	payload, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return nil, fmt.Errorf("lineargql: encode payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return errResult("failed to build request"), nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.apiKey) // key never appears in slog output

	resp, err := c.http.Do(req)
	if err != nil {
		return errResult("transport error"), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errResult(fmt.Sprintf("HTTP %d", resp.StatusCode)), nil
	}

	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors json.RawMessage `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return errResult("response decode error"), nil
	}

	if len(envelope.Errors) > 0 && string(envelope.Errors) != "null" {
		return &Result{Success: false, Data: envelope.Data, Errors: envelope.Errors}, nil
	}
	return &Result{Success: true, Data: envelope.Data}, nil
}

func errResult(msg string) *Result {
	errs, _ := json.Marshal([]map[string]string{{"message": msg}})
	return &Result{Success: false, Errors: errs}
}
