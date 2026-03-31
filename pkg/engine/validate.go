package engine

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const (
	MaxTreeDepth        = 20
	MaxCombinatorDepth  = 3
	MaxConditionsPerGroup = 10
)

var validOperators = map[string]bool{
	"eq": true, "neq": true,
	"gt": true, "gte": true,
	"lt": true, "lte": true,
	"in": true, "nin": true,
	"regex": true, "pct": true,
}

var validRuleTypes = map[string]bool{
	"feature_flag":  true,
	"decision_tree": true,
	"kill_switch":   true,
}

var validStatuses = map[string]bool{
	"active":   true,
	"draft":    true,
	"disabled": true,
}

// ValidateRule validates a complete rule for creation or update.
func ValidateRule(r *Rule) error {
	if strings.TrimSpace(r.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if !validRuleTypes[r.Type] {
		return fmt.Errorf("type must be one of: feature_flag, decision_tree, kill_switch")
	}
	if r.Status != "" && !validStatuses[r.Status] {
		return fmt.Errorf("status must be one of: active, draft, disabled")
	}
	if len(r.Tree) == 0 {
		return fmt.Errorf("tree is required")
	}

	var root Node
	if err := json.Unmarshal(r.Tree, &root); err != nil {
		return fmt.Errorf("invalid tree JSON: %w", err)
	}

	return validateNode(&root, 0)
}

// validateNode recursively validates a tree node.
func validateNode(n *Node, depth int) error {
	if depth > MaxTreeDepth {
		return fmt.Errorf("tree exceeds maximum depth of %d", MaxTreeDepth)
	}

	// Leaf node is always valid
	if n.IsLeaf() {
		return nil
	}

	// Branch node must have a condition
	if n.Condition != nil {
		if err := validateCondition(n.Condition, 0); err != nil {
			return err
		}
	}

	if n.Then != nil {
		if err := validateNode(n.Then, depth+1); err != nil {
			return err
		}
	}
	if n.Else != nil {
		if err := validateNode(n.Else, depth+1); err != nil {
			return err
		}
	}

	return nil
}

// validateCondition validates a condition (single or combinator).
func validateCondition(c *Condition, depth int) error {
	if c.IsCombinator() {
		return validateCombinator(c, depth)
	}
	return validateSingleCondition(c)
}

// validateSingleCondition validates a field comparison condition.
func validateSingleCondition(c *Condition) error {
	if strings.TrimSpace(c.Field) == "" {
		return fmt.Errorf("condition field is required")
	}
	if !validOperators[c.Operator] {
		return fmt.Errorf("invalid operator: %s", c.Operator)
	}

	// Operator-specific validation
	switch c.Operator {
	case "gt", "gte", "lt", "lte":
		if _, ok := toFloat64(c.Value); !ok {
			return fmt.Errorf("operator %s requires a numeric value", c.Operator)
		}
	case "in", "nin":
		if _, ok := c.Value.([]any); !ok {
			return fmt.Errorf("operator %s requires an array value", c.Operator)
		}
	case "regex":
		pattern, ok := c.Value.(string)
		if !ok {
			return fmt.Errorf("operator regex requires a string pattern")
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("invalid regex pattern: %w", err)
		}
	case "pct":
		pctVal, ok := toFloat64(c.Value)
		if !ok {
			return fmt.Errorf("operator pct requires a numeric value")
		}
		if pctVal < 0 || pctVal > 100 {
			return fmt.Errorf("pct value must be between 0 and 100")
		}
	}

	return nil
}

// validateCombinator validates an AND/OR combinator group.
func validateCombinator(c *Condition, depth int) error {
	if depth >= MaxCombinatorDepth {
		return fmt.Errorf("combinator nesting exceeds maximum depth of %d", MaxCombinatorDepth)
	}
	if c.Combinator != "and" && c.Combinator != "or" {
		return fmt.Errorf("combinator must be 'and' or 'or', got: %s", c.Combinator)
	}
	if len(c.Conditions) == 0 {
		return fmt.Errorf("combinator must have at least one condition")
	}
	if len(c.Conditions) > MaxConditionsPerGroup {
		return fmt.Errorf("combinator exceeds maximum of %d conditions per group", MaxConditionsPerGroup)
	}

	for _, sub := range c.Conditions {
		if err := validateCondition(sub, depth+1); err != nil {
			return err
		}
	}

	return nil
}
