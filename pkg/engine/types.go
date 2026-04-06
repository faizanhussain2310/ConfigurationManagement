package engine

import (
	"encoding/json"
	"fmt"
	"time"
)

// Rule is the top-level entity stored in the database.
type Rule struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Type         string          `json:"type"` // feature_flag, decision_tree, kill_switch, composite
	Version      int             `json:"version"`
	Tree         json.RawMessage `json:"tree"`
	DefaultValue json.RawMessage `json:"default_value,omitempty"`
	Status       string          `json:"status"`      // active, draft, disabled
	Environment  string          `json:"environment"`  // production, staging, development
	ActiveFrom   *time.Time      `json:"active_from,omitempty"`
	ActiveUntil  *time.Time      `json:"active_until,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// IsScheduleActive checks if the rule is within its scheduled activation window.
// Returns true if no schedule is set (always active) or current time is within the window.
func (r *Rule) IsScheduleActive() bool {
	now := time.Now().UTC()
	if r.ActiveFrom != nil && now.Before(*r.ActiveFrom) {
		return false
	}
	if r.ActiveUntil != nil && now.After(*r.ActiveUntil) {
		return false
	}
	return true
}

// RuleVersion is an immutable snapshot of a rule at a specific version.
type RuleVersion struct {
	ID           int             `json:"id"`
	RuleID       string          `json:"rule_id"`
	Version      int             `json:"version"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Type         string          `json:"type"`
	Tree         json.RawMessage `json:"tree"`
	DefaultValue json.RawMessage `json:"default_value,omitempty"`
	Status       string          `json:"status"`
	Environment  string          `json:"environment"`
	ActiveFrom   *time.Time      `json:"active_from,omitempty"`
	ActiveUntil  *time.Time      `json:"active_until,omitempty"`
	ModifiedBy   string          `json:"modified_by"`
	CreatedAt    time.Time       `json:"created_at"`
}

// RuleVersionSummary is the lightweight version returned by GET /versions.
type RuleVersionSummary struct {
	Version    int       `json:"version"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	ModifiedBy string    `json:"modified_by"`
	CreatedAt  time.Time `json:"created_at"`
}

// Condition is either a single field comparison OR a logical group.
// Single: Field + Operator + Value are set, Combinator is empty.
// Group: Combinator ("and" | "or") + Conditions are set, Field is empty.
type Condition struct {
	Field    string `json:"field,omitempty"`
	Operator string `json:"op,omitempty"`
	Value    any    `json:"value,omitempty"`

	Combinator string       `json:"combinator,omitempty"` // "and" | "or"
	Conditions []*Condition `json:"conditions,omitempty"` // max 10 per group, max nesting 3
}

// IsCombinator returns true if this condition is a logical group.
func (c *Condition) IsCombinator() bool {
	return c.Combinator != ""
}

// Node represents a decision tree node. It's either a branch (condition + then/else)
// or a leaf (has a value). Custom UnmarshalJSON handles the Value field correctly
// so that false, 0, and "" are preserved as valid leaf values.
type Node struct {
	Condition *Condition `json:"condition,omitempty"`
	Then      *Node      `json:"then,omitempty"`
	Else      *Node      `json:"else,omitempty"`

	RawValue json.RawMessage `json:"-"`
	HasValue bool            `json:"-"`
	Value    any             `json:"value,omitempty"`
}

// UnmarshalJSON implements custom JSON unmarshaling to detect the presence of
// the "value" key. Go's omitempty drops false/0/"" on marshal, but we need to
// know if value was explicitly set during unmarshal.
func (n *Node) UnmarshalJSON(data []byte) error {
	// Use a raw map to detect if "value" key is present
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("invalid node JSON: %w", err)
	}

	// Parse condition
	if condData, ok := raw["condition"]; ok {
		var cond Condition
		if err := json.Unmarshal(condData, &cond); err != nil {
			return fmt.Errorf("invalid condition: %w", err)
		}
		n.Condition = &cond
	}

	// Parse then/else (recursive)
	if thenData, ok := raw["then"]; ok {
		var then Node
		if err := json.Unmarshal(thenData, &then); err != nil {
			return fmt.Errorf("invalid then node: %w", err)
		}
		n.Then = &then
	}
	if elseData, ok := raw["else"]; ok {
		var els Node
		if err := json.Unmarshal(elseData, &els); err != nil {
			return fmt.Errorf("invalid else node: %w", err)
		}
		n.Else = &els
	}

	// Detect value presence
	if valData, ok := raw["value"]; ok {
		n.RawValue = valData
		n.HasValue = true
		var val any
		if err := json.Unmarshal(valData, &val); err != nil {
			return fmt.Errorf("invalid value: %w", err)
		}
		n.Value = val
	}

	return nil
}

// MarshalJSON implements custom JSON marshaling. If HasValue is true, always
// include the "value" key even if the value is false/0/"".
func (n Node) MarshalJSON() ([]byte, error) {
	type nodeAlias struct {
		Condition *Condition      `json:"condition,omitempty"`
		Then      *Node           `json:"then,omitempty"`
		Else      *Node           `json:"else,omitempty"`
		Value     json.RawMessage `json:"value,omitempty"`
	}

	out := nodeAlias{
		Condition: n.Condition,
		Then:      n.Then,
		Else:      n.Else,
	}

	if n.HasValue {
		if n.RawValue != nil {
			out.Value = n.RawValue
		} else {
			b, err := json.Marshal(n.Value)
			if err != nil {
				return nil, err
			}
			out.Value = b
		}
	}

	return json.Marshal(out)
}

// IsLeaf returns true if this node has a value (is a leaf node).
func (n *Node) IsLeaf() bool {
	return n.HasValue
}

// EvalResult is the output of evaluating a rule against a context.
type EvalResult struct {
	Value   any      `json:"value"`
	Path    []string `json:"path"`
	Default bool     `json:"default"`
	Error   string   `json:"error,omitempty"`
	Elapsed string   `json:"elapsed"`
}

// EvalHistoryEntry is a stored evaluation record.
type EvalHistoryEntry struct {
	ID        int             `json:"id"`
	RuleID    string          `json:"rule_id"`
	Context   json.RawMessage `json:"context"`
	Result    json.RawMessage `json:"result"`
	CreatedAt time.Time       `json:"created_at"`
}
