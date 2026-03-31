# API Reference

All endpoints return JSON. Errors follow the shape `{"error": "message"}`.

Base URL: `http://localhost:8080`

## Health Check

### `GET /api/health`

```bash
curl http://localhost:8080/api/health
```

Response `200`:
```json
{"status": "ok"}
```

---

## Rules CRUD

### `POST /api/rules` — Create a rule

```bash
curl -X POST http://localhost:8080/api/rules \
  -H "Content-Type: application/json" \
  -d '{
    "id": "my_flag",
    "name": "My Feature Flag",
    "description": "Controls the new checkout flow",
    "type": "feature_flag",
    "tree": {"value": true},
    "default_value": false,
    "status": "active"
  }'
```

**Required fields:** `id`, `name`, `type`, `tree`

**Optional fields:** `description`, `default_value`, `status` (defaults to `"active"`)

**Valid types:** `feature_flag`, `decision_tree`, `kill_switch`

**Valid statuses:** `active`, `draft`, `disabled`

Response `201`:
```json
{
  "id": "my_flag",
  "name": "My Feature Flag",
  "description": "Controls the new checkout flow",
  "type": "feature_flag",
  "version": 1,
  "tree": {"value": true},
  "default_value": false,
  "status": "active",
  "created_at": "2026-03-31T12:00:00Z",
  "updated_at": "2026-03-31T12:00:00Z"
}
```

Error `409` if ID already exists:
```json
{"error": "rule with id 'my_flag' already exists"}
```

Error `400` on validation failure:
```json
{"error": "invalid operator: 'foo' (valid: eq, neq, gt, gte, lt, lte, in, nin, regex, pct)"}
```

---

### `GET /api/rules` — List rules (paginated)

```bash
curl "http://localhost:8080/api/rules?limit=10&offset=0"
```

**Query params:**
- `limit` (default: 50, max: 100)
- `offset` (default: 0)

Response `200`:
```json
{
  "rules": [
    {
      "id": "pricing_tier",
      "name": "Pricing Tier",
      "type": "decision_tree",
      "version": 1,
      "status": "active",
      "...": "..."
    }
  ],
  "total": 4,
  "limit": 10,
  "offset": 0
}
```

`total` is the count of all rules (not just this page). Use it for pagination UI:
- Total pages = `ceil(total / limit)`
- Next page = `offset + limit`

---

### `GET /api/rules/:id` — Get a single rule

```bash
curl http://localhost:8080/api/rules/pricing_tier
```

Response `200`: Full rule object (same shape as create response).

Error `404`:
```json
{"error": "rule not found"}
```

---

### `PUT /api/rules/:id` — Update a rule

Updates the rule and creates a new version (v+1) atomically.

```bash
curl -X PUT http://localhost:8080/api/rules/pricing_tier \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Pricing Tier v2",
    "type": "decision_tree",
    "tree": {"value": "enterprise"},
    "status": "active"
  }'
```

The `id` in the URL takes precedence over any `id` in the body. You don't need to send `id` in the body.

Response `200`: Updated rule with incremented `version`.

---

### `DELETE /api/rules/:id` — Delete a rule

Hard delete. Cascades to all versions and evaluation history.

```bash
curl -X DELETE http://localhost:8080/api/rules/my_flag
```

Response `200`:
```json
{"status": "deleted"}
```

Error `404` if rule doesn't exist.

---

## Evaluation

### `POST /api/rules/:id/evaluate` — Evaluate a single rule

Send a context object, get a value back with the decision path.

```bash
curl -X POST http://localhost:8080/api/rules/pricing_tier/evaluate \
  -H "Content-Type: application/json" \
  -d '{"context": {"org": {"employees": 50}}}'
```

Response `200`:
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

**Fields explained:**
- `value`: The result of the evaluation. Can be any JSON type (boolean, string, number, null).
- `path`: Array of strings showing each decision the engine made. Read top to bottom.
- `default`: `true` if the engine reached a dead end and returned the rule's default value.
- `elapsed`: How long the evaluation took (tree walk only, not counting DB fetch).

**Empty context** is valid. Missing fields in conditions evaluate to `false`:

```bash
curl -X POST http://localhost:8080/api/rules/pricing_tier/evaluate \
  -H "Content-Type: application/json" \
  -d '{"context": {}}'
```

---

### `POST /api/evaluate` — Batch evaluate multiple rules

Evaluates multiple rules against the same context in parallel. Each rule is independent. One rule failing doesn't affect the others.

```bash
curl -X POST http://localhost:8080/api/evaluate \
  -H "Content-Type: application/json" \
  -d '{
    "rule_ids": ["pricing_tier", "dark_mode_rollout", "nonexistent"],
    "context": {"org": {"employees": 50}, "user": {"id": "user_123"}}
  }'
```

Response `200`:
```json
{
  "results": [
    {
      "rule_id": "pricing_tier",
      "result": {"value": "team", "path": [...], "default": false, "elapsed": "29µs"}
    },
    {
      "rule_id": "dark_mode_rollout",
      "result": {"value": false, "path": [...], "default": false, "elapsed": "12µs"}
    },
    {
      "rule_id": "nonexistent",
      "result": {"error": "rule not found", "path": []}
    }
  ]
}
```

Results preserve the order of `rule_ids`. Up to 10 rules are evaluated concurrently (worker pool).

---

## Evaluation History

### `GET /api/rules/:id/history` — Get evaluation history (paginated)

```bash
curl "http://localhost:8080/api/rules/pricing_tier/history?limit=10&offset=0"
```

**Query params:** `limit` (default: 50, max: 100), `offset` (default: 0)

Response `200`:
```json
{
  "entries": [
    {
      "id": 42,
      "rule_id": "pricing_tier",
      "context": {"org": {"employees": 50}},
      "result": {"value": "team", "path": [...], "default": false, "elapsed": "29µs"},
      "created_at": "2026-03-31T12:05:00Z"
    }
  ],
  "total": 156,
  "limit": 10,
  "offset": 0
}
```

Entries are ordered newest first. Max 1,000 entries retained per rule (older entries are automatically pruned).

---

## Versioning

### `GET /api/rules/:id/versions` — List version history

```bash
curl http://localhost:8080/api/rules/pricing_tier/versions
```

Response `200`:
```json
{
  "versions": [
    {"version": 3, "name": "Pricing Tier v2", "status": "active", "created_at": "2026-03-31T14:00:00Z"},
    {"version": 2, "name": "Pricing Tier", "status": "active", "created_at": "2026-03-31T13:00:00Z"},
    {"version": 1, "name": "Pricing Tier", "status": "active", "created_at": "2026-03-31T12:00:00Z"}
  ]
}
```

Versions are ordered newest first. This is the summary view (no tree JSON). The dashboard fetches full snapshots individually to compute diffs.

---

### `POST /api/rules/:id/rollback/:version` — Rollback to a version

Creates a **new** version (v+1) that copies the target version's snapshot. Non-destructive: the version chain is preserved.

```bash
curl -X POST http://localhost:8080/api/rules/pricing_tier/rollback/1
```

If the rule is at v3 and you rollback to v1, this creates v4 with v1's content. v1, v2, and v3 still exist in history.

Response `200`: The rule at its new version (same shape as GET rule).

Error `404` if the rule or target version doesn't exist.

---

## Import / Export

### `GET /api/rules/:id/export` — Export a rule as JSON

Downloads the rule as a `.json` file (includes `Content-Disposition` header).

```bash
curl http://localhost:8080/api/rules/pricing_tier/export -o pricing_tier.json
```

The exported file is the same shape as the rule object. You can import it into another Arbiter instance.

---

### `POST /api/rules/import` — Import a rule from JSON

The request body IS the rule JSON (same shape as create).

```bash
curl -X POST http://localhost:8080/api/rules/import \
  -H "Content-Type: application/json" \
  -d @pricing_tier.json
```

**Conflict handling:**

If a rule with the same `id` already exists, you get `409`:
```json
{
  "error": "conflict: rule pricing_tier already exists",
  "existing": {
    "id": "pricing_tier",
    "name": "Pricing Tier",
    "version": 3,
    "...": "..."
  }
}
```

To force-overwrite, add `?force=true`:
```bash
curl -X POST "http://localhost:8080/api/rules/import?force=true" \
  -H "Content-Type: application/json" \
  -d @pricing_tier.json
```

Force import creates a new version (v+1) on the existing rule. The existing version history is preserved.

Response `201`: The imported rule.

---

## Duplicate

### `POST /api/rules/:id/duplicate` — Duplicate a rule

Creates a copy with `-copy` suffix. If `pricing_tier-copy` already exists, tries `pricing_tier-copy-1`, `pricing_tier-copy-2`, etc. (up to 9 copies).

```bash
curl -X POST http://localhost:8080/api/rules/pricing_tier/duplicate
```

Response `201`:
```json
{
  "id": "pricing_tier-copy",
  "name": "Pricing Tier-copy",
  "version": 1,
  "...": "..."
}
```

The duplicate starts at version 1 with its own version history. It copies the source rule's current tree, default value, description, type, and status.

---

## Error Shapes

All errors follow this format:

```json
{"error": "human-readable error message"}
```

| Status | When |
|--------|------|
| `400` | Invalid JSON, validation failure (bad operator, tree too deep, etc.) |
| `404` | Rule or version not found |
| `409` | ID collision on create or import (import includes `existing` metadata) |
| `413` | Request body exceeds 1MB |
| `500` | Database or internal error |
