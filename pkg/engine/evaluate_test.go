package engine

import (
	"encoding/json"
	"testing"
)

func TestEvaluateSimpleLeaf(t *testing.T) {
	tree := `{"value": true}`
	result := Evaluate(json.RawMessage(tree), map[string]any{}, "test", nil)
	if result.Value != true {
		t.Errorf("expected true, got %v", result.Value)
	}
	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
}

func TestEvaluateFalseLeaf(t *testing.T) {
	tree := `{"value": false}`
	result := Evaluate(json.RawMessage(tree), map[string]any{}, "test", nil)
	if result.Value != false {
		t.Errorf("expected false, got %v", result.Value)
	}
}

func TestEvaluateZeroLeaf(t *testing.T) {
	tree := `{"value": 0}`
	result := Evaluate(json.RawMessage(tree), map[string]any{}, "test", nil)
	val, ok := toFloat64(result.Value)
	if !ok || val != 0 {
		t.Errorf("expected 0, got %v", result.Value)
	}
}

func TestEvaluateEmptyStringLeaf(t *testing.T) {
	tree := `{"value": ""}`
	result := Evaluate(json.RawMessage(tree), map[string]any{}, "test", nil)
	if result.Value != "" {
		t.Errorf("expected empty string, got %v", result.Value)
	}
}

func TestEvaluateSimpleBranch(t *testing.T) {
	tree := `{
		"condition": {"field": "user.age", "op": "gte", "value": 18},
		"then": {"value": "adult"},
		"else": {"value": "minor"}
	}`

	// Adult
	result := Evaluate(json.RawMessage(tree), map[string]any{
		"user": map[string]any{"age": float64(25)},
	}, "test", nil)
	if result.Value != "adult" {
		t.Errorf("expected 'adult', got %v", result.Value)
	}

	// Minor
	result = Evaluate(json.RawMessage(tree), map[string]any{
		"user": map[string]any{"age": float64(15)},
	}, "test", nil)
	if result.Value != "minor" {
		t.Errorf("expected 'minor', got %v", result.Value)
	}
}

func TestEvaluateNestedTree(t *testing.T) {
	tree := `{
		"condition": {"field": "org.employees", "op": "gte", "value": 100},
		"then": {"value": "enterprise"},
		"else": {
			"condition": {"field": "org.employees", "op": "gte", "value": 10},
			"then": {"value": "team"},
			"else": {"value": "starter"}
		}
	}`

	tests := []struct {
		employees float64
		expected  string
	}{
		{200, "enterprise"},
		{50, "team"},
		{5, "starter"},
	}

	for _, tt := range tests {
		ctx := map[string]any{"org": map[string]any{"employees": tt.employees}}
		result := Evaluate(json.RawMessage(tree), ctx, "test", nil)
		if result.Value != tt.expected {
			t.Errorf("employees=%v: expected %q, got %v", tt.employees, tt.expected, result.Value)
		}
	}
}

func TestEvaluateANDCombinator(t *testing.T) {
	tree := `{
		"condition": {
			"combinator": "and",
			"conditions": [
				{"field": "user.age", "op": "gte", "value": 18},
				{"field": "user.country", "op": "eq", "value": "US"}
			]
		},
		"then": {"value": true},
		"else": {"value": false}
	}`

	// Both true
	result := Evaluate(json.RawMessage(tree), map[string]any{
		"user": map[string]any{"age": float64(25), "country": "US"},
	}, "test", nil)
	if result.Value != true {
		t.Errorf("expected true, got %v", result.Value)
	}

	// One false (short-circuit)
	result = Evaluate(json.RawMessage(tree), map[string]any{
		"user": map[string]any{"age": float64(15), "country": "US"},
	}, "test", nil)
	if result.Value != false {
		t.Errorf("expected false, got %v", result.Value)
	}
}

func TestEvaluateORCombinator(t *testing.T) {
	tree := `{
		"condition": {
			"combinator": "or",
			"conditions": [
				{"field": "user.age", "op": "gte", "value": 18},
				{"field": "user.country", "op": "eq", "value": "US"}
			]
		},
		"then": {"value": true},
		"else": {"value": false}
	}`

	// One true is enough
	result := Evaluate(json.RawMessage(tree), map[string]any{
		"user": map[string]any{"age": float64(15), "country": "US"},
	}, "test", nil)
	if result.Value != true {
		t.Errorf("expected true, got %v", result.Value)
	}

	// Both false
	result = Evaluate(json.RawMessage(tree), map[string]any{
		"user": map[string]any{"age": float64(15), "country": "IN"},
	}, "test", nil)
	if result.Value != false {
		t.Errorf("expected false, got %v", result.Value)
	}
}

func TestEvaluateEmptyContext(t *testing.T) {
	tree := `{
		"condition": {"field": "user.name", "op": "eq", "value": "alice"},
		"then": {"value": true},
		"else": {"value": false}
	}`
	result := Evaluate(json.RawMessage(tree), map[string]any{}, "test", nil)
	if result.Value != false {
		t.Errorf("expected false with empty context, got %v", result.Value)
	}
}

func TestEvaluateDefaultValue(t *testing.T) {
	tree := `{
		"condition": {"field": "x", "op": "eq", "value": "y"}
	}`
	defVal := json.RawMessage(`"fallback"`)
	result := Evaluate(json.RawMessage(tree), map[string]any{}, "test", defVal)
	if result.Value != "fallback" {
		t.Errorf("expected 'fallback', got %v", result.Value)
	}
	if !result.Default {
		t.Error("expected Default=true")
	}
}

func TestEvaluateDecisionPath(t *testing.T) {
	tree := `{
		"condition": {"field": "x", "op": "eq", "value": 1},
		"then": {"value": "yes"},
		"else": {"value": "no"}
	}`
	result := Evaluate(json.RawMessage(tree), map[string]any{"x": float64(1)}, "test", nil)
	if len(result.Path) == 0 {
		t.Error("expected non-empty path")
	}
}

func TestOperatorIn(t *testing.T) {
	if !EvalOperator("in", "US", []any{"US", "CA", "GB"}, "") {
		t.Error("expected US in [US, CA, GB]")
	}
	if EvalOperator("in", "IN", []any{"US", "CA", "GB"}, "") {
		t.Error("expected IN not in [US, CA, GB]")
	}
}

func TestOperatorRegex(t *testing.T) {
	if !EvalOperator("regex", "test@company.com", `@company\.com$`, "") {
		t.Error("expected regex match")
	}
	if EvalOperator("regex", "test@other.com", `@company\.com$`, "") {
		t.Error("expected regex no match")
	}
}

func TestOperatorPctDeterministic(t *testing.T) {
	// Same input should always give same result
	r1 := EvalOperator("pct", "user123", float64(50), "rule_a")
	r2 := EvalOperator("pct", "user123", float64(50), "rule_a")
	if r1 != r2 {
		t.Error("pct should be deterministic")
	}
}

func TestOperatorPctBoundaries(t *testing.T) {
	// pct=0 should be off for everyone
	if EvalOperator("pct", "any_user", float64(0), "test") {
		t.Error("pct=0 should be false")
	}
	// pct=100 should be on for everyone
	if !EvalOperator("pct", "any_user", float64(100), "test") {
		t.Error("pct=100 should be true")
	}
}

func TestOperatorPctDecorrelation(t *testing.T) {
	// Different rule IDs should give different results for some users
	differs := false
	for i := 0; i < 100; i++ {
		userID := string(rune('a' + i%26))
		r1 := EvalOperator("pct", userID, float64(50), "rule_a")
		r2 := EvalOperator("pct", userID, float64(50), "rule_b")
		if r1 != r2 {
			differs = true
			break
		}
	}
	if !differs {
		t.Error("pct with different rule IDs should decorrelate")
	}
}

func TestGetFieldNested(t *testing.T) {
	ctx := map[string]any{
		"user": map[string]any{
			"profile": map[string]any{
				"age": float64(25),
			},
		},
	}
	v := GetField(ctx, "user.profile.age")
	if v != float64(25) {
		t.Errorf("expected 25, got %v", v)
	}
}

func TestGetFieldMissing(t *testing.T) {
	ctx := map[string]any{"user": map[string]any{"name": "alice"}}
	v := GetField(ctx, "user.age")
	if v != nil {
		t.Errorf("expected nil for missing field, got %v", v)
	}
}

func TestValidateRuleValid(t *testing.T) {
	r := &Rule{
		ID:   "test",
		Name: "Test",
		Type: "feature_flag",
		Tree: json.RawMessage(`{"value": true}`),
	}
	if err := ValidateRule(r); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateRuleMissingID(t *testing.T) {
	r := &Rule{Name: "Test", Type: "feature_flag", Tree: json.RawMessage(`{"value": true}`)}
	if err := ValidateRule(r); err == nil {
		t.Error("expected error for missing ID")
	}
}

func TestValidateRuleInvalidType(t *testing.T) {
	r := &Rule{ID: "test", Name: "Test", Type: "invalid", Tree: json.RawMessage(`{"value": true}`)}
	if err := ValidateRule(r); err == nil {
		t.Error("expected error for invalid type")
	}
}

func TestValidateRuleMaxDepth(t *testing.T) {
	// Build a tree that exceeds max depth
	inner := `{"value": true}`
	for i := 0; i < 22; i++ {
		inner = `{"condition":{"field":"x","op":"eq","value":1},"then":` + inner + `,"else":{"value":false}}`
	}
	r := &Rule{ID: "deep", Name: "Deep", Type: "feature_flag", Tree: json.RawMessage(inner)}
	err := ValidateRule(r)
	if err == nil {
		t.Error("expected error for tree exceeding max depth")
	}
}

func TestValidateCombinatorMaxConditions(t *testing.T) {
	conds := make([]any, 11)
	for i := range conds {
		conds[i] = map[string]any{"field": "x", "op": "eq", "value": i}
	}
	tree, _ := json.Marshal(map[string]any{
		"condition": map[string]any{"combinator": "and", "conditions": conds},
		"then":      map[string]any{"value": true},
		"else":      map[string]any{"value": false},
	})
	r := &Rule{ID: "test", Name: "Test", Type: "feature_flag", Tree: tree}
	err := ValidateRule(r)
	if err == nil {
		t.Error("expected error for >10 conditions")
	}
}
