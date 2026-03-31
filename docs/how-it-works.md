# How It Works

Arbiter treats every configuration decision as a **decision tree**. Context goes in, a value comes out. That's the whole model.

A feature flag? It's a tree with one leaf: `true` or `false`.
A pricing calculator? It's a tree with branches that check `org.employees` and return `"starter"`, `"team"`, or `"enterprise"`.
A percentage rollout? It's a tree where the condition uses the `pct` operator to deterministically bucket users.

Same engine. Same API. Same dashboard.

## The Evaluation Loop

When you call `POST /api/rules/:id/evaluate` with a context object:

```json
{"context": {"user": {"age": 25, "country": "US"}}}
```

The engine does this:

1. **Unmarshal the tree** from stored JSON into a `Node` struct
2. **Walk the tree** starting at the root node
3. At each node, check if it's a **leaf** (has a `value` field) or a **branch** (has a `condition`)
4. If it's a leaf, return the value. Done.
5. If it's a branch, **evaluate the condition** against the context
6. If the condition is true, recurse into the `then` branch
7. If the condition is false, recurse into the `else` branch
8. Record every decision in the **path trace**

The whole thing is in `pkg/engine/evaluate.go`. The core function is `evalNode`, about 30 lines of recursive Go.

## Tree Structure

Every node is one of two things:

**Leaf node** (returns a value):
```json
{"value": "enterprise"}
```

**Branch node** (makes a decision):
```json
{
  "condition": {"field": "org.employees", "op": "gte", "value": 100},
  "then": {"value": "enterprise"},
  "else": {"value": "starter"}
}
```

Branches can nest. The `then` or `else` of any branch can be another branch:

```
                    ┌──────────────────┐
                    │ employees >= 100?│
                    └───┬──────────┬───┘
                      true       false
                ┌─────▼─────┐ ┌──▼──────────────┐
                │"enterprise"│ │ employees >= 10? │
                └───────────┘ └───┬──────────┬───┘
                                true       false
                          ┌─────▼─────┐ ┌──▼──────┐
                          │  "team"   │ │"starter" │
                          └───────────┘ └──────────┘
```

This is `examples/pricing_tier.json` in the repo.

## Conditions

A condition is a comparison: take a field from the context, apply an operator, compare against a value.

```json
{"field": "user.age", "op": "gte", "value": 18}
```

**Field resolution** uses dot notation. `"user.age"` means: look up `ctx["user"]`, then `["age"]`. This traversal is in `pkg/engine/context.go`, the `GetField` function. Missing fields return `nil`, which makes the condition false (safe default).

**Operators** are in `pkg/engine/operators.go`. There are 10:

| Operator | What it does |
|----------|-------------|
| `eq` / `neq` | Equality with type-aware comparison |
| `gt` / `gte` / `lt` / `lte` | Numeric comparison (coerces to float64) |
| `in` / `nin` | Membership in an array |
| `regex` | RE2 regex match (compiled and cached) |
| `pct` | Deterministic percentage rollout via FNV-1a hash |

See `docs/operators.md` for the full reference with edge cases.

## AND/OR Combinators

A condition can also be a **group** of conditions joined by AND or OR:

```json
{
  "condition": {
    "combinator": "and",
    "conditions": [
      {"field": "user.age", "op": "gte", "value": 18},
      {"field": "user.country", "op": "eq", "value": "US"}
    ]
  },
  "then": {"value": true},
  "else": {"value": false}
}
```

**AND** short-circuits on the first `false`. If the user's age is 15, the engine never checks country.

**OR** short-circuits on the first `true`. If the user is 25, it doesn't matter what country they're in.

Combinators can nest up to 3 levels deep. Max 10 conditions per group. These limits are enforced by `ValidateRule` in `pkg/engine/validate.go`.

## Decision Path Tracing

Every evaluation returns a `path` array showing exactly why the engine returned what it did:

```json
{
  "value": "team",
  "path": [
    "org.employees gte 100 → false",
    "org.employees gte 10 → true",
    "→ value: team"
  ],
  "default": false,
  "elapsed": "29µs"
}
```

No other open source tool does this at the tree level. When a user reports "I got the wrong feature flag value," you look at the path and see exactly which branch was taken and why.

## The Value Problem (false, 0, "")

JSON has falsy values: `false`, `0`, `""`. In Go, `json:"value,omitempty"` silently drops all of these. A kill switch that returns `false` would serialize as `{}`, not `{"value": false}`.

Arbiter solves this with a custom `UnmarshalJSON` on the `Node` type. It uses `json.RawMessage` to detect whether the `"value"` key was literally present in the JSON, and sets a `HasValue` sentinel boolean. The `IsLeaf()` method checks `HasValue`, not the value itself.

This is about 20 lines in `pkg/engine/types.go`. It's the kind of edge case that separates a working system from a demo.

## Default Values

Every rule can have an optional `default_value`. If the engine reaches a dead end (missing `then`/`else` branch, exceeds max depth of 20), it returns the default value with `"default": true` in the result.

If no default is set and the tree bottoms out, the value is `nil` and `default` is `true`. The caller knows something unexpected happened.

## Percentage Rollouts

The `pct` operator uses FNV-1a hashing:

```
hash(rule_id + ":" + field_value) % 100 < pct_value
```

Three properties make this work:

1. **Deterministic**: Same user, same rule, same result. Always.
2. **Decorrelated**: The `rule_id` is part of the hash key, so a user who's in the 25% bucket for rule A isn't necessarily in the 25% bucket for rule B.
3. **Uniform**: FNV-1a has good distribution. 25% means roughly 25% of users, not 25% of some skewed subset.

## Storage

Rules are stored in SQLite with WAL mode. Two connection pools: one for writes (single writer, serialized), one for reads (4 concurrent readers). This matches SQLite's concurrency model exactly.

Every update creates an immutable version snapshot in `rule_versions`. Rollback copies a past version forward as v+1. It's non-destructive... you never lose history.

Evaluation history is stored with a retention of 1,000 entries per rule, pruned automatically every 100th insert.

All of this is in `pkg/store/sqlite.go`.
