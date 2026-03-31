package engine

import (
	"strings"
)

// GetField extracts a value from a nested context map using dot-notation.
// Example: GetField(ctx, "user.age") traverses ctx["user"]["age"].
// Returns nil if any segment is missing or the intermediate value is not a map.
func GetField(ctx map[string]any, field string) any {
	parts := strings.Split(field, ".")
	var current any = ctx

	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current, ok = m[part]
		if !ok {
			return nil
		}
	}

	return current
}
