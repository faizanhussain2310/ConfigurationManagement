package engine

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"regexp"
	"sync"
)

// regexCache stores compiled regexes to avoid recompiling on every evaluation.
// Protected by a sync.RWMutex for concurrent read access.
var (
	regexMu    sync.RWMutex
	regexCache = make(map[string]*regexp.Regexp)
)

// ClearRegexCache removes all cached regexes. Call on rule delete.
func ClearRegexCache() {
	regexMu.Lock()
	regexCache = make(map[string]*regexp.Regexp)
	regexMu.Unlock()
}

// getCompiledRegex returns a compiled regex from cache or compiles and caches it.
func getCompiledRegex(pattern string) (*regexp.Regexp, error) {
	regexMu.RLock()
	if re, ok := regexCache[pattern]; ok {
		regexMu.RUnlock()
		return re, nil
	}
	regexMu.RUnlock()

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	regexMu.Lock()
	regexCache[pattern] = re
	regexMu.Unlock()

	return re, nil
}

// toFloat64 coerces a value to float64 for numeric comparison.
// JSON numbers come as float64 from encoding/json. SQLite integers may
// come back as int64 after a round-trip. This handles both.
func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

// EvalOperator evaluates a single operator condition against a context value.
// ruleID is required for the pct operator (decorrelation).
func EvalOperator(op string, fieldValue any, condValue any, ruleID string) bool {
	switch op {
	case "eq":
		return evalEq(fieldValue, condValue)
	case "neq":
		return !evalEq(fieldValue, condValue)
	case "gt":
		return evalNumericCompare(fieldValue, condValue) > 0
	case "gte":
		return evalNumericCompare(fieldValue, condValue) >= 0
	case "lt":
		return evalNumericCompare(fieldValue, condValue) < 0
	case "lte":
		return evalNumericCompare(fieldValue, condValue) <= 0
	case "in":
		return evalIn(fieldValue, condValue)
	case "nin":
		return !evalIn(fieldValue, condValue)
	case "regex":
		return evalRegex(fieldValue, condValue)
	case "pct":
		return evalPct(fieldValue, condValue, ruleID)
	default:
		return false
	}
}

// evalEq compares two values for equality using type-aware comparison.
func evalEq(a, b any) bool {
	fa, aIsNum := toFloat64(a)
	fb, bIsNum := toFloat64(b)
	if aIsNum && bIsNum {
		return fa == fb
	}

	sa, aIsStr := a.(string)
	sb, bIsStr := b.(string)
	if aIsStr && bIsStr {
		return sa == sb
	}

	ba, aIsBool := a.(bool)
	bb, bIsBool := b.(bool)
	if aIsBool && bIsBool {
		return ba == bb
	}

	if a == nil && b == nil {
		return true
	}

	// Cross-type: coerce both to string for comparison
	if (aIsStr || bIsStr) && (aIsNum || bIsNum) {
		return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
	}

	return false
}

// evalNumericCompare returns -1, 0, or 1 for a < b, a == b, a > b.
func evalNumericCompare(a, b any) int {
	fa, aOk := toFloat64(a)
	fb, bOk := toFloat64(b)
	if !aOk || !bOk {
		return 0
	}
	if fa < fb {
		return -1
	}
	if fa > fb {
		return 1
	}
	return 0
}

// evalIn checks if fieldValue is in the condValue array.
func evalIn(fieldValue any, condValue any) bool {
	arr, ok := condValue.([]any)
	if !ok {
		return false
	}
	for _, item := range arr {
		if evalEq(fieldValue, item) {
			return true
		}
	}
	return false
}

// evalRegex matches fieldValue against a regex pattern in condValue.
func evalRegex(fieldValue any, condValue any) bool {
	str, ok := fieldValue.(string)
	if !ok {
		return false
	}
	pattern, ok := condValue.(string)
	if !ok {
		return false
	}
	re, err := getCompiledRegex(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(str)
}

// evalPct implements deterministic percentage rollout.
// FNV-1a(rule_id + ":" + field_value) % 100 < pct_value
func evalPct(fieldValue any, condValue any, ruleID string) bool {
	pctVal, ok := toFloat64(condValue)
	if !ok {
		return false
	}

	fieldStr := fmt.Sprintf("%v", fieldValue)
	key := ruleID + ":" + fieldStr
	h := fnv.New32a()
	h.Write([]byte(key))
	hash := h.Sum32()

	bucket := hash % 100
	return float64(bucket) < pctVal
}
