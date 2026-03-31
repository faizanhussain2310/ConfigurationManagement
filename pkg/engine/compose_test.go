package engine

import (
	"encoding/json"
	"testing"
)

func TestValidateCompositeTree(t *testing.T) {
	valid := `{"strategy":"all_true","rule_ids":["r1","r2"]}`
	if err := ValidateCompositeTree(json.RawMessage(valid)); err != nil {
		t.Errorf("expected valid, got error: %v", err)
	}

	// Invalid strategy
	bad := `{"strategy":"nope","rule_ids":["r1"]}`
	if err := ValidateCompositeTree(json.RawMessage(bad)); err == nil {
		t.Error("expected error for invalid strategy")
	}

	// No rules
	empty := `{"strategy":"all_true","rule_ids":[]}`
	if err := ValidateCompositeTree(json.RawMessage(empty)); err == nil {
		t.Error("expected error for empty rule_ids")
	}

	// Duplicate
	dup := `{"strategy":"all_true","rule_ids":["r1","r1"]}`
	if err := ValidateCompositeTree(json.RawMessage(dup)); err == nil {
		t.Error("expected error for duplicate rule_ids")
	}

	// Empty ID
	emptyID := `{"strategy":"all_true","rule_ids":["r1",""]}`
	if err := ValidateCompositeTree(json.RawMessage(emptyID)); err == nil {
		t.Error("expected error for empty rule ID")
	}
}

func TestCombineAllTrue(t *testing.T) {
	children := []CompositeChild{
		{RuleID: "r1", Result: EvalResult{Value: true, Path: []string{}}},
		{RuleID: "r2", Result: EvalResult{Value: true, Path: []string{}}},
	}
	result := CombineResults("all_true", children)
	if result.Value != true {
		t.Errorf("expected true, got %v", result.Value)
	}

	children[1].Result.Value = false
	result = CombineResults("all_true", children)
	if result.Value != false {
		t.Errorf("expected false, got %v", result.Value)
	}
}

func TestCombineAnyTrue(t *testing.T) {
	children := []CompositeChild{
		{RuleID: "r1", Result: EvalResult{Value: false, Path: []string{}}},
		{RuleID: "r2", Result: EvalResult{Value: true, Path: []string{}}},
	}
	result := CombineResults("any_true", children)
	if result.Value != true {
		t.Errorf("expected true, got %v", result.Value)
	}

	children[1].Result.Value = false
	result = CombineResults("any_true", children)
	if result.Value != false {
		t.Errorf("expected false, got %v", result.Value)
	}
}

func TestCombineFirstMatch(t *testing.T) {
	children := []CompositeChild{
		{RuleID: "r1", Result: EvalResult{Value: nil, Default: true, Path: []string{}}},
		{RuleID: "r2", Result: EvalResult{Value: "matched", Path: []string{}}},
	}
	result := CombineResults("first_match", children)
	if result.Value != "matched" {
		t.Errorf("expected 'matched', got %v", result.Value)
	}
	if result.Default {
		t.Error("expected non-default result")
	}

	// All defaults
	allDefaults := []CompositeChild{
		{RuleID: "r1", Result: EvalResult{Value: nil, Default: true, Path: []string{}}},
	}
	result = CombineResults("first_match", allDefaults)
	if !result.Default {
		t.Error("expected default when no match")
	}
}

func TestCombineMergeResults(t *testing.T) {
	children := []CompositeChild{
		{RuleID: "r1", Result: EvalResult{Value: true, Path: []string{}}},
		{RuleID: "r2", Result: EvalResult{Value: 42.0, Path: []string{}}},
	}
	result := CombineResults("merge_results", children)
	merged, ok := result.Value.(map[string]any)
	if !ok {
		t.Fatal("expected map result")
	}
	if merged["r1"] != true {
		t.Errorf("expected r1=true, got %v", merged["r1"])
	}
	if merged["r2"] != 42.0 {
		t.Errorf("expected r2=42, got %v", merged["r2"])
	}
}

func TestDetectCycles(t *testing.T) {
	// No cycle
	existing := map[string][]string{
		"a": {"b"},
	}
	if err := DetectCycles("c", []string{"a"}, existing); err != nil {
		t.Errorf("expected no cycle, got: %v", err)
	}

	// Direct cycle
	if err := DetectCycles("a", []string{"a"}, map[string][]string{}); err == nil {
		t.Error("expected cycle error for self-reference")
	}

	// Indirect cycle: a→b→c→a
	existing = map[string][]string{
		"b": {"c"},
		"c": {"a"},
	}
	if err := DetectCycles("a", []string{"b"}, existing); err == nil {
		t.Error("expected cycle error for indirect cycle")
	}
}

func TestIsTruthy(t *testing.T) {
	tests := []struct {
		val  any
		want bool
	}{
		{nil, false},
		{true, true},
		{false, false},
		{1.0, true},
		{0.0, false},
		{"hello", true},
		{"", false},
		{[]int{1}, true},
	}
	for _, tt := range tests {
		got := isTruthy(tt.val)
		if got != tt.want {
			t.Errorf("isTruthy(%v) = %v, want %v", tt.val, got, tt.want)
		}
	}
}
