package engine

import (
	"encoding/json"
	"fmt"
	"time"
)

// ComposeConfig defines how a composite rule combines child rules.
type ComposeConfig struct {
	Strategy string   `json:"strategy"` // all_true, any_true, first_match, merge_results
	RuleIDs  []string `json:"rule_ids"`
}

// CompositeResult is the output of evaluating a composite rule.
type CompositeResult struct {
	Value   any                `json:"value"`
	Results []CompositeChild   `json:"results"`
	Default bool               `json:"default"`
	Elapsed string             `json:"elapsed"`
}

// CompositeChild is the result of one child rule in a composite evaluation.
type CompositeChild struct {
	RuleID string     `json:"rule_id"`
	Result EvalResult `json:"result"`
}

var validStrategies = map[string]bool{
	"all_true":      true,
	"any_true":      true,
	"first_match":   true,
	"merge_results": true,
}

// ValidateCompositeTree validates the tree field of a composite rule.
func ValidateCompositeTree(tree json.RawMessage) error {
	var config ComposeConfig
	if err := json.Unmarshal(tree, &config); err != nil {
		return fmt.Errorf("invalid composite config: %w", err)
	}
	if !validStrategies[config.Strategy] {
		return fmt.Errorf("strategy must be one of: all_true, any_true, first_match, merge_results")
	}
	if len(config.RuleIDs) == 0 {
		return fmt.Errorf("composite rule must reference at least one child rule")
	}
	if len(config.RuleIDs) > 20 {
		return fmt.Errorf("composite rule cannot reference more than 20 child rules")
	}
	// Check for duplicates
	seen := map[string]bool{}
	for _, id := range config.RuleIDs {
		if id == "" {
			return fmt.Errorf("child rule ID cannot be empty")
		}
		if seen[id] {
			return fmt.Errorf("duplicate child rule ID: %s", id)
		}
		seen[id] = true
	}
	return nil
}

// ParseComposeConfig extracts the composition config from a rule's tree field.
func ParseComposeConfig(tree json.RawMessage) (*ComposeConfig, error) {
	var config ComposeConfig
	if err := json.Unmarshal(tree, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// CombineResults applies a strategy to combine multiple child results.
func CombineResults(strategy string, children []CompositeChild) CompositeResult {
	start := time.Now()

	var value any
	isDefault := false

	switch strategy {
	case "all_true":
		// All child results must be truthy. Returns false if any is falsy.
		allTrue := true
		for _, c := range children {
			if !isTruthy(c.Result.Value) {
				allTrue = false
				break
			}
		}
		value = allTrue

	case "any_true":
		// At least one child result must be truthy.
		anyTrue := false
		for _, c := range children {
			if isTruthy(c.Result.Value) {
				anyTrue = true
				break
			}
		}
		value = anyTrue

	case "first_match":
		// Return the first non-default, non-error result.
		matched := false
		for _, c := range children {
			if c.Result.Error == "" && !c.Result.Default {
				value = c.Result.Value
				matched = true
				break
			}
		}
		if !matched {
			isDefault = true
		}

	case "merge_results":
		// Return a map of rule_id → value.
		merged := map[string]any{}
		for _, c := range children {
			merged[c.RuleID] = c.Result.Value
		}
		value = merged
	}

	return CompositeResult{
		Value:   value,
		Results: children,
		Default: isDefault,
		Elapsed: time.Since(start).String(),
	}
}

// isTruthy checks if a value is truthy (non-nil, non-false, non-zero, non-empty).
func isTruthy(v any) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case float64:
		return val != 0
	case string:
		return val != ""
	default:
		return true
	}
}

// DetectCycles checks if adding the given dependencies would create a cycle.
// allRules maps rule_id → list of child rule_ids (for composite rules).
func DetectCycles(ruleID string, childIDs []string, allComposites map[string][]string) error {
	// Build graph including the proposed new edges
	graph := make(map[string][]string)
	for k, v := range allComposites {
		graph[k] = v
	}
	graph[ruleID] = childIDs

	// DFS cycle detection
	visited := map[string]int{} // 0=unvisited, 1=in-progress, 2=done
	var dfs func(node string) error
	dfs = func(node string) error {
		if visited[node] == 1 {
			return fmt.Errorf("circular reference detected: rule %q creates a cycle", node)
		}
		if visited[node] == 2 {
			return nil
		}
		visited[node] = 1
		for _, dep := range graph[node] {
			if err := dfs(dep); err != nil {
				return err
			}
		}
		visited[node] = 2
		return nil
	}

	return dfs(ruleID)
}
