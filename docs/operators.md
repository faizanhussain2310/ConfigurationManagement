# Operator Reference

Arbiter supports 10 operators for condition evaluation. Each operator compares a **field value** (extracted from the evaluation context) against a **condition value** (defined in the rule tree).

## Equality

### `eq` (equals)

Compares two values with type-aware logic.

```json
{"field": "user.country", "op": "eq", "value": "US"}
```

**Type coercion rules:**
- Two numbers: compared as float64 (`42 == 42.0` is true)
- Two strings: exact match
- Two booleans: exact match
- `nil == nil` is true
- Cross-type (string vs number): both coerced to string via `fmt.Sprintf("%v", ...)`

### `neq` (not equals)

Inverse of `eq`. Returns `true` when `eq` would return `false`.

```json
{"field": "user.status", "op": "neq", "value": "banned"}
```

## Numeric Comparison

### `gt` (greater than), `gte` (greater than or equal)
### `lt` (less than), `lte` (less than or equal)

Compare two numeric values. Both sides are coerced to float64.

```json
{"field": "user.age", "op": "gte", "value": 18}
{"field": "cart.total", "op": "lt", "value": 100.50}
```

**Edge cases:**
- If either side can't be coerced to a number, the comparison returns `false` (not an error)
- JSON numbers arrive as `float64` from `encoding/json`. SQLite integers may arrive as `int64` after a round-trip. Both are handled.
- Supports `float64`, `float32`, `int`, `int64`, and `json.Number`

## Membership

### `in` (value in list)

Checks if the field value appears in an array. Each element is compared using the same `eq` logic (type-aware).

```json
{"field": "user.country", "op": "in", "value": ["US", "CA", "GB"]}
```

**Edge cases:**
- If `value` is not an array, returns `false`
- Empty array `[]` always returns `false`
- Mixed-type arrays work: `[1, "2", true]` will match field value `1`, `"2"`, or `true`

### `nin` (value not in list)

Inverse of `in`.

```json
{"field": "user.country", "op": "nin", "value": ["CN", "RU"]}
```

## Pattern Matching

### `regex` (regular expression match)

Matches the field value against an RE2 regular expression pattern.

```json
{"field": "user.email", "op": "regex", "value": "@company\\.com$"}
```

**Details:**
- Uses Go's `regexp` package (RE2 syntax, guaranteed linear time)
- Compiled regexes are cached in memory with `sync.RWMutex` for concurrent access
- Cache is cleared when a rule is deleted (`ClearRegexCache()`)
- If the field value is not a string, returns `false`
- If the pattern is invalid, returns `false` (validated at rule save time by `ValidateRule`)

**Common patterns:**
| Pattern | Matches |
|---------|---------|
| `@company\.com$` | Emails ending in @company.com |
| `^v[0-9]+\.` | Strings starting with v1., v2., etc. |
| `(?i)premium` | Case-insensitive "premium" anywhere |

## Percentage Rollout

### `pct` (percentage)

Deterministic percentage-based rollout. Returns `true` for a consistent subset of field values.

```json
{"field": "user.id", "op": "pct", "value": 25}
```

This means: 25% of users (by their `user.id`) will get `true`.

**How it works:**

```
FNV-1a(rule_id + ":" + field_value) % 100 < pct_value
```

1. Concatenate the rule ID and the field value with a colon separator
2. Hash using FNV-1a (32-bit, from Go's `hash/fnv` stdlib)
3. Take modulo 100 to get a bucket (0-99)
4. Compare against the percentage threshold

**Properties:**

- **Deterministic:** Same user + same rule = same result. Every time. No randomness.
- **Decorrelated:** Rule ID is part of the hash key. A user in the 25% group for `dark_mode_rollout` is NOT necessarily in the 25% group for `new_pricing`. Different rules get different user distributions.
- **Uniform:** FNV-1a has good distribution. 25% means roughly 25% of your user population.

**Edge cases:**
- `pct: 0` is always `false` (no users)
- `pct: 100` is always `true` (all users)
- The field value is coerced to string via `fmt.Sprintf("%v", ...)` before hashing. Integer `42` and string `"42"` produce the same hash.
- `pct` values outside 0-100 are rejected by `ValidateRule`

**Example: gradual rollout**

Start at 5%, monitor, increase:

```json
{"field": "user.id", "op": "pct", "value": 5}
```

The users in the 5% bucket are a strict subset of the users in the 10% bucket, which are a strict subset of the 25% bucket, and so on. This is because the bucket threshold just moves up. Users already in don't get removed as you increase the percentage.

## Validation Rules

`ValidateRule` in `pkg/engine/validate.go` enforces these constraints at save time:

| Operator | Constraint |
|----------|-----------|
| `gt`, `gte`, `lt`, `lte` | Condition value must be numeric |
| `in`, `nin` | Condition value must be an array |
| `regex` | Condition value must be a valid RE2 pattern (compiled at validation time) |
| `pct` | Condition value must be a number in range [0, 100] |
| `eq`, `neq` | Any value type accepted |

Additional tree limits:
- Max tree depth: 20
- Max combinator nesting: 3 levels
- Max conditions per combinator group: 10
