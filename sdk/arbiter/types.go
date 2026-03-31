package arbiter

import (
	"encoding/json"
	"fmt"
	"time"
)

// Rule represents a decision rule in Arbiter.
type Rule struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Type         string          `json:"type"`
	Version      int             `json:"version"`
	Tree         json.RawMessage `json:"tree"`
	DefaultValue json.RawMessage `json:"default_value,omitempty"`
	Status       string          `json:"status"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// EvalResult is the result of evaluating a rule against a context.
type EvalResult struct {
	Value   any      `json:"value"`
	Path    []string `json:"path"`
	Default bool     `json:"default"`
	Error   string   `json:"error,omitempty"`
	Elapsed string   `json:"elapsed"`
}

// VersionSummary is a compact representation of a rule version.
type VersionSummary struct {
	Version   int       `json:"version"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// ListRulesResponse is the response envelope for paginated rule listings.
type ListRulesResponse struct {
	Rules  []Rule `json:"rules"`
	Total  int    `json:"total"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

// BatchEvalRequest is the request body for batch evaluation.
type BatchEvalRequest struct {
	RuleIDs []string       `json:"rule_ids"`
	Context map[string]any `json:"context"`
}

// BatchEvalResponse maps rule IDs to their evaluation results.
type BatchEvalResponse struct {
	Results map[string]EvalResult `json:"results"`
}

// APIError represents an error response from the Arbiter API.
type APIError struct {
	StatusCode int    `json:"-"`
	Message    string `json:"error"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("arbiter: HTTP %d: %s", e.StatusCode, e.Message)
}
