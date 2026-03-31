# Data Model

This doc explains the database schema, how the tables relate, and the versioning lifecycle.

## Tables

### `rules` — The live state of every rule

```sql
CREATE TABLE rules (
    id TEXT PRIMARY KEY,               -- user-chosen, e.g. "pricing_tier"
    name TEXT NOT NULL,                -- display name, e.g. "Pricing Tier"
    description TEXT DEFAULT '',       -- optional description
    type TEXT NOT NULL,                -- one of: feature_flag, decision_tree, kill_switch
    version INTEGER NOT NULL DEFAULT 1,-- current version number
    tree TEXT NOT NULL,                -- the decision tree as JSON
    default_value TEXT,                -- optional default value as JSON (nullable)
    status TEXT NOT NULL DEFAULT 'active', -- one of: active, draft, disabled
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
```

This is the "current state" table. When you `GET /api/rules/pricing_tier`, this is what you get. There's exactly one row per rule.

**`id` is user-chosen, not auto-generated.** This is deliberate. IDs like `pricing_tier` or `dark_mode_rollout` are meaningful and appear in application code. UUIDs would be useless in `if (evaluate("pricing_tier", ctx))`.

**`tree` is a JSON string stored as TEXT.** SQLite doesn't have a native JSON type, but TEXT works fine. The engine parses it into a `Node` struct on every evaluation. This is fast (~microseconds) because trees are small (typically under 1KB).

**`default_value` is nullable.** If null, the engine returns `nil` when the tree bottoms out. If set, the engine returns the default value with `"default": true` in the result.

**`version` always equals the highest version in `rule_versions` for this rule.** They're incremented atomically in the same transaction. If they ever disagree, something went wrong.

---

### `rule_versions` — Immutable snapshots of every change

```sql
CREATE TABLE rule_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id TEXT NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    type TEXT NOT NULL,
    tree TEXT NOT NULL,              -- full JSON snapshot at this version
    default_value TEXT,
    status TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    UNIQUE(rule_id, version)        -- can't have two v3s for the same rule
);
```

Every time a rule changes, a full snapshot is stored here. Not a diff, the complete state. This makes rollback trivial (copy the snapshot) and diffs straightforward (compare any two versions).

**Why full snapshots instead of diffs?**

Diffs (like JSON Patch) are smaller but harder to work with. To reconstruct v1 from v5, you'd need to apply 4 reverse patches. With full snapshots, you just read the row. The storage cost is negligible... a typical rule version is under 1KB, and rules rarely have more than 20-30 versions.

**The UNIQUE constraint** prevents the same version number from being inserted twice for a rule. Combined with `BEGIN IMMEDIATE` transactions, this guarantees version numbers are sequential with no gaps.

---

### `eval_history` — Every evaluation recorded

```sql
CREATE TABLE eval_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id TEXT NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
    context TEXT NOT NULL,           -- the input context as JSON
    result TEXT NOT NULL,            -- the full EvalResult as JSON
    created_at DATETIME NOT NULL
);

CREATE INDEX idx_eval_history_rule_id ON eval_history(rule_id, created_at DESC);
```

Every time a rule is evaluated, the context and result are stored here (asynchronously, in a background goroutine). This is useful for debugging: "what context did user X send, and what did they get back?"

**Retention:** Max 1,000 entries per rule. Pruning happens every 100th insert (not every insert, to avoid the cost). The prune deletes everything except the 1,000 most recent entries.

**The composite index** `(rule_id, created_at DESC)` makes `SELECT ... WHERE rule_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?` fast. Without it, SQLite would scan the entire table.

---

### `_meta` — Key-value metadata

```sql
CREATE TABLE _meta (
    key TEXT PRIMARY KEY,
    value TEXT
);
```

Currently stores one key: `seeded_at`. This tracks whether the seed data has been inserted. Without it, restarting the server after deleting all rules would re-seed.

The seed logic checks:
1. Does `_meta` have a `seeded_at` key? If yes, don't seed.
2. Is the `rules` table empty? If no, don't seed.
3. Otherwise, seed 4 example rules and set `seeded_at`.

---

## Relationships

```
rules (1) ◄──────── (N) rule_versions    ON DELETE CASCADE
rules (1) ◄──────── (N) eval_history     ON DELETE CASCADE
_meta                                     standalone
```

**Cascade delete** means: when you `DELETE FROM rules WHERE id = 'pricing_tier'`, SQLite automatically deletes all `rule_versions` and `eval_history` rows for that rule. You don't need separate DELETE statements.

**Important:** This only works because `PRAGMA foreign_keys = ON` is set on every connection. SQLite defaults to `OFF`. Without this pragma, foreign keys are parsed but ignored, and cascade deletes silently do nothing.

---

## Versioning Lifecycle

Here's the version number progression for a rule through its life:

### 1. Create

```
POST /api/rules → CreateRule()
  Transaction:
    INSERT INTO rules (... version=1 ...)
    INSERT INTO rule_versions (... version=1 ...)
  Result: rules.version = 1, rule_versions has [v1]
```

### 2. Update

```
PUT /api/rules/pricing_tier → UpdateRule()
  Transaction:
    SELECT version FROM rules → 1
    UPDATE rules SET ... version=2 ...
    INSERT INTO rule_versions (... version=2 ...)
  Result: rules.version = 2, rule_versions has [v1, v2]
```

### 3. Update again

```
PUT /api/rules/pricing_tier → UpdateRule()
  Transaction:
    SELECT version FROM rules → 2
    UPDATE rules SET ... version=3 ...
    INSERT INTO rule_versions (... version=3 ...)
  Result: rules.version = 3, rule_versions has [v1, v2, v3]
```

### 4. Rollback to v1

```
POST /api/rules/pricing_tier/rollback/1 → RollbackToVersion()
  Transaction:
    SELECT * FROM rule_versions WHERE version=1  ← get v1 snapshot
    SELECT version FROM rules → 3                ← current version
    UPDATE rules SET ... version=4 ... (with v1's data)
    INSERT INTO rule_versions (... version=4 ...) (with v1's data)
  Result: rules.version = 4, rule_versions has [v1, v2, v3, v4]
         v4 has the same content as v1
```

**Rollback doesn't delete anything.** It copies the target version's snapshot forward as a new version. v2 and v3 still exist. You can rollback to v3 later if you want. The version chain is always append-only.

### 5. Force import

```
POST /api/rules/import?force=true → ImportRule() → UpdateRule()
  Same as a normal update: creates v5 with the imported data.
  Result: rules.version = 5, rule_versions has [v1, v2, v3, v4, v5]
```

### Visual timeline

```
v1 (create) → v2 (update) → v3 (update) → v4 (rollback to v1) → v5 (import)
                                              ↑
                                    same content as v1
```

---

## Go Structs

The Go types that map to these tables are in `pkg/engine/types.go`:

```go
type Rule struct {
    ID           string          `json:"id"`
    Name         string          `json:"name"`
    Description  string          `json:"description"`
    Type         string          `json:"type"`
    Version      int             `json:"version"`
    Tree         json.RawMessage `json:"tree"`
    DefaultValue json.RawMessage `json:"default_value,omitempty"`
    Status       string          `json:"status"`
    CreatedAt    time.Time       `json:"created_at"`
    UpdatedAt    time.Time       `json:"updated_at"`
}
```

**`json.RawMessage` for Tree and DefaultValue.** These are stored as JSON strings in SQLite and passed as raw JSON bytes to the engine. They're only unmarshaled into `Node` structs during evaluation, not during CRUD operations. This avoids unnecessary parsing.

**The SQLite scan workaround.** When reading from SQLite, the `modernc.org/sqlite` driver returns TEXT columns as Go `string`, not `[]byte`. But `json.RawMessage` is `[]byte`. Direct scanning fails. The fix in `sqlite.go`:

```go
var treeStr string                      // scan into string first
rows.Scan(..., &treeStr, ...)
r.Tree = json.RawMessage(treeStr)       // then convert
```

This is in `scanRule()` and `scanRuleRows()`.

---

## Seed Data

On first startup, Arbiter seeds 4 example rules (defined in `pkg/store/seeds.go`):

| ID | Type | What it demonstrates |
|----|------|---------------------|
| `new_user_onboarding` | feature_flag | AND combinator (signup_days + country) |
| `pricing_tier` | decision_tree | Nested tree (3 tiers based on employee count) |
| `dark_mode_rollout` | feature_flag | Percentage rollout (25% via pct operator) |
| `emergency_shutdown` | kill_switch | Simple leaf node (value: false) |

Seeds are Go structs, not loaded from files. The `examples/` folder contains the same rules as reference JSON for humans to read, but the seed logic doesn't read them.
