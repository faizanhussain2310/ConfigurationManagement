package engine

import (
	"encoding/json"
	"fmt"
	"time"
)

// Evaluate walks a decision tree with the given context and returns a result.
// It traces the path taken through the tree for debugging.
func Evaluate(treeJSON json.RawMessage, ctx map[string]any, ruleID string, defaultValue json.RawMessage) EvalResult {
	start := time.Now()

	var root Node
	if err := json.Unmarshal(treeJSON, &root); err != nil {
		return EvalResult{
			Error:   fmt.Sprintf("invalid tree: %s", err),
			Path:    []string{},
			Elapsed: time.Since(start).String(),
		}
	}

	path := []string{}
	value, isDefault := evalNode(&root, ctx, ruleID, &path, 0)

	if isDefault && defaultValue != nil {
		var defVal any
		if err := json.Unmarshal(defaultValue, &defVal); err == nil {
			return EvalResult{
				Value:   defVal,
				Path:    path,
				Default: true,
				Elapsed: time.Since(start).String(),
			}
		}
	}

	return EvalResult{
		Value:   value,
		Path:    path,
		Default: isDefault,
		Elapsed: time.Since(start).String(),
	}
}

// evalNode recursively evaluates a single node. Returns the value and whether
// the result came from hitting the tree bottom (default).
func evalNode(node *Node, ctx map[string]any, ruleID string, path *[]string, depth int) (any, bool) {
	if depth > 20 {
		*path = append(*path, "max depth exceeded")
		return nil, true
	}

	// Leaf node: return the value
	if node.IsLeaf() {
		*path = append(*path, fmt.Sprintf("→ value: %v", node.Value))
		return node.Value, false
	}

	// No condition means this is an incomplete node, return default
	if node.Condition == nil {
		*path = append(*path, "→ no condition (default)")
		return nil, true
	}

	// Evaluate condition
	result := evalCondition(node.Condition, ctx, ruleID, path)

	if result {
		if node.Then != nil {
			return evalNode(node.Then, ctx, ruleID, path, depth+1)
		}
		*path = append(*path, "→ then branch missing (default)")
		return nil, true
	}

	if node.Else != nil {
		return evalNode(node.Else, ctx, ruleID, path, depth+1)
	}
	*path = append(*path, "→ else branch missing (default)")
	return nil, true
}

// evalCondition evaluates a Condition (single or combinator) against a context.
func evalCondition(cond *Condition, ctx map[string]any, ruleID string, path *[]string) bool {
	if cond.IsCombinator() {
		return evalCombinator(cond, ctx, ruleID, path)
	}
	return evalSingleCondition(cond, ctx, ruleID, path)
}

// evalSingleCondition evaluates a single field comparison.
func evalSingleCondition(cond *Condition, ctx map[string]any, ruleID string, path *[]string) bool {
	fieldValue := GetField(ctx, cond.Field)
	result := EvalOperator(cond.Operator, fieldValue, cond.Value, ruleID)

	if result {
		*path = append(*path, fmt.Sprintf("%s %s %v → true", cond.Field, cond.Operator, cond.Value))
	} else {
		*path = append(*path, fmt.Sprintf("%s %s %v → false", cond.Field, cond.Operator, cond.Value))
	}

	return result
}

// evalCombinator evaluates AND/OR combinator groups with short-circuit logic.
func evalCombinator(cond *Condition, ctx map[string]any, ruleID string, path *[]string) bool {
	*path = append(*path, fmt.Sprintf("(%s group)", cond.Combinator))

	switch cond.Combinator {
	case "and":
		for _, sub := range cond.Conditions {
			if !evalCondition(sub, ctx, ruleID, path) {
				return false // short-circuit on first false
			}
		}
		return true
	case "or":
		for _, sub := range cond.Conditions {
			if evalCondition(sub, ctx, ruleID, path) {
				return true // short-circuit on first true
			}
		}
		return false
	default:
		*path = append(*path, fmt.Sprintf("unknown combinator: %s", cond.Combinator))
		return false
	}
}
