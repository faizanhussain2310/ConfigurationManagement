package arbiter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// Client is the Arbiter API client.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	AuthToken  string
}

// New creates a new Client pointing at the given base URL.
func New(baseURL string) *Client {
	return &Client{
		BaseURL:    baseURL,
		HTTPClient: http.DefaultClient,
	}
}

// NewWithOptions creates a new Client with a custom http.Client.
func NewWithOptions(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		BaseURL:    baseURL,
		HTTPClient: httpClient,
	}
}

// WithAuth sets the JWT auth token and returns the client for chaining.
func (c *Client) WithAuth(token string) *Client {
	c.AuthToken = token
	return c
}

// do executes an HTTP request and decodes the JSON response into dest.
// If dest is nil the response body is discarded after checking for errors.
func (c *Client) do(ctx context.Context, method, path string, body any, dest any) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("arbiter: marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("arbiter: build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("arbiter: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("arbiter: read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		apiErr := &APIError{StatusCode: resp.StatusCode}
		if err := json.Unmarshal(respBody, apiErr); err != nil {
			apiErr.Message = string(respBody)
		}
		return apiErr
	}

	if dest != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, dest); err != nil {
			return fmt.Errorf("arbiter: decode response: %w", err)
		}
	}

	return nil
}

// Health checks the Arbiter server health.
func (c *Client) Health(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/api/health", nil, nil)
}

// CreateRule creates a new rule.
func (c *Client) CreateRule(ctx context.Context, rule Rule) (*Rule, error) {
	var out Rule
	if err := c.do(ctx, http.MethodPost, "/api/rules", rule, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListRules returns a paginated list of rules.
func (c *Client) ListRules(ctx context.Context, limit, offset int) (*ListRulesResponse, error) {
	path := "/api/rules?limit=" + strconv.Itoa(limit) + "&offset=" + strconv.Itoa(offset)
	var out ListRulesResponse
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetRule retrieves a single rule by ID.
func (c *Client) GetRule(ctx context.Context, id string) (*Rule, error) {
	var out Rule
	if err := c.do(ctx, http.MethodGet, "/api/rules/"+id, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateRule updates an existing rule by ID.
func (c *Client) UpdateRule(ctx context.Context, id string, rule Rule) (*Rule, error) {
	var out Rule
	if err := c.do(ctx, http.MethodPut, "/api/rules/"+id, rule, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteRule deletes a rule by ID.
func (c *Client) DeleteRule(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/rules/"+id, nil, nil)
}

// Evaluate evaluates a single rule against the given context.
func (c *Client) Evaluate(ctx context.Context, id string, evalCtx map[string]any) (*EvalResult, error) {
	var out EvalResult
	if err := c.do(ctx, http.MethodPost, "/api/rules/"+id+"/evaluate", evalCtx, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// BatchEvaluate evaluates multiple rules against the given context.
func (c *Client) BatchEvaluate(ctx context.Context, ruleIDs []string, evalCtx map[string]any) (*BatchEvalResponse, error) {
	req := BatchEvalRequest{
		RuleIDs: ruleIDs,
		Context: evalCtx,
	}
	var out BatchEvalResponse
	if err := c.do(ctx, http.MethodPost, "/api/evaluate", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListVersions returns all versions for a rule.
func (c *Client) ListVersions(ctx context.Context, id string) ([]VersionSummary, error) {
	var out []VersionSummary
	if err := c.do(ctx, http.MethodGet, "/api/rules/"+id+"/versions", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Rollback rolls a rule back to a specific version.
func (c *Client) Rollback(ctx context.Context, id string, version int) (*Rule, error) {
	var out Rule
	if err := c.do(ctx, http.MethodPost, "/api/rules/"+id+"/rollback/"+strconv.Itoa(version), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Duplicate duplicates a rule.
func (c *Client) Duplicate(ctx context.Context, id string) (*Rule, error) {
	var out Rule
	if err := c.do(ctx, http.MethodPost, "/api/rules/"+id+"/duplicate", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Export exports a rule as JSON.
func (c *Client) Export(ctx context.Context, id string) (*Rule, error) {
	var out Rule
	if err := c.do(ctx, http.MethodGet, "/api/rules/"+id+"/export", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Import imports a rule.
func (c *Client) Import(ctx context.Context, rule Rule) (*Rule, error) {
	var out Rule
	if err := c.do(ctx, http.MethodPost, "/api/rules/import", rule, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ImportForce imports a rule with force=true, overwriting any existing rule with the same ID.
func (c *Client) ImportForce(ctx context.Context, rule Rule) (*Rule, error) {
	var out Rule
	if err := c.do(ctx, http.MethodPost, "/api/rules/import?force=true", rule, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
