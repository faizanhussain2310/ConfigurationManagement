# Backend Code Reading Guide

This guide tells you exactly which files to read, in what order, and what to pay attention to in each one. Designed for someone with ~1 year of experience who wants to understand the full system.

## How the backend is organized

```
cmd/arbiter/main.go      ← starts the server
cmd/wasm/main.go          ← WASM build target (separate)

pkg/engine/               ← pure evaluation logic (no database, no HTTP)
pkg/store/                ← SQLite persistence layer
pkg/api/                  ← HTTP handlers, routes, middleware
pkg/auth/                 ← JWT tokens and password hashing
pkg/webhooks/             ← async event delivery
```

The key insight: **layers never reach upward**. Engine doesn't know about Store. Store doesn't know about API. This keeps the evaluation logic testable and portable (which is why WASM compilation works).

---

## Reading order

### Step 1: Types (the vocabulary)

**Read: `pkg/engine/types.go` (176 lines)**

This defines every data structure the system uses. Start here because every other file refers to these types.

Pay attention to:
- `Rule` struct — the main entity. `Tree` is `json.RawMessage` (raw JSON bytes, not parsed yet)
- `Node` struct — a decision tree node. Either a leaf (has `Value`) or a branch (has `Condition` + `Then` + `Else`)
- `Condition` struct — either a single comparison (`field`, `op`, `value`) or a logical group (`combinator` + `conditions`)
- The custom `UnmarshalJSON` on Node (lines 78-122). This is the trickiest part. Go's `omitempty` drops `false`, `0`, and `""` during marshaling, but we need to preserve them as valid leaf values. The custom unmarshaler uses a `HasValue` bool to track whether `"value"` was explicitly present in the JSON

**Read: `pkg/engine/context.go` (26 lines)**

Tiny file. `GetField()` does dot-notation traversal. `"user.age"` with context `{"user": {"age": 25}}` returns `25`. This is how conditions access nested data.

### Step 2: Evaluation (the core algorithm)

**Read: `pkg/engine/evaluate.go` (129 lines)**

This is the heart of the system. Read it top to bottom.

- `Evaluate()` (line 11) — entry point. Takes raw tree JSON + context + rule ID. Returns `EvalResult` with value, decision path, and timing
- `evalNode()` (line 48) — recursive tree walk. Leaf? Return value. Branch? Evaluate condition, then recurse into `then` or `else`
- `evalCondition()` (line 85) — dispatches to single condition or combinator
- `evalCombinator()` (line 107) — AND/OR logic with short-circuit. For AND: stop on first false. For OR: stop on first true

The `path` slice tracks every decision made during evaluation. This is what makes debugging possible... you can see exactly why a rule returned what it did.

**Read: `pkg/engine/operators.go` (191 lines)**

10 operators. The interesting ones:
- `evalRegex()` — caches compiled regex patterns in a `sync.RWMutex`-protected map. Without this, every evaluation would recompile the regex
- `evalPct()` — percentage rollout. Uses FNV-1a hash of `ruleID + ":" + fieldValue` to get a deterministic number 0-99. If the hash result < threshold, it's true. This means the same user always gets the same result for the same rule, but different rules give different distributions (decorrelation)
- `evalIn()` / `evalNin()` — array membership with type-aware comparison (handles JSON number/string mismatches)

### Step 3: Validation

**Read: `pkg/engine/validate.go` (170 lines)**

Called before any rule is saved. Enforces:
- Required fields (ID, name, type)
- Tree depth max 20 (prevents stack overflow during evaluation)
- Combinator nesting max 3 (AND inside OR inside AND is the deepest)
- Max 10 conditions per group
- Regex patterns must compile
- Percentage values must be 0-100

### Step 4: Composition

**Read: `pkg/engine/compose.go` (183 lines)**

Composite rules reference other rules by ID and combine their results.

- `ComposeConfig` — just a strategy name + list of child rule IDs
- `CombineResults()` — four strategies: `all_true` (logical AND), `any_true` (logical OR), `first_match` (first non-default result), `merge_results` (returns map of id → value)
- `DetectCycles()` — DFS cycle detection. Uses 3-state coloring: unvisited (0), in-progress (1), done (2). If you hit a node that's in-progress, you've found a cycle

### Step 5: Persistence

**Read: `pkg/store/migrations.go` (63 lines)**

Schema definition. 6 tables. Read this to understand the data model before looking at queries.

**Read: `pkg/store/sqlite.go` (544 lines)**

The biggest file. This is the database layer.

Key patterns:
- **Two connection pools** (line ~30). `writeDB` has `max_connections=1` (SQLite only allows one writer). `readDB` has `max_connections=4`. All mutations go through writeDB, all reads through readDB
- **WAL mode** — Write-Ahead Logging. Readers don't block writers. This is set via PRAGMA at connection time
- **Automatic versioning** — every `UpdateRule()` call inserts a new row in `rule_versions` and bumps the version number. The old state is never lost
- **History pruning** — `InsertEvalHistory()` uses an atomic counter (`pruneCounters` map). Every 100 inserts, it deletes old entries beyond the 1000 limit. This avoids running a COUNT query on every insert
- **Rollback** — `RollbackToVersion()` doesn't delete anything. It copies the old version's data into the current rule and creates a new version. So "rollback to v1" when you're at v3 creates v4 with v1's content

**Read: `pkg/store/seeds.go` (143 lines)**

4 example rules seeded on first run. The `_meta` table tracks whether seeding already happened. Also seeds a default admin user.

**Read: `pkg/store/users.go` (110 lines) and `pkg/store/webhooks.go` (100 lines)**

Straightforward CRUD. Nothing tricky here. Note that `GetUserByUsername` returns the password hash (for login), while `ListUsers` does not.

### Step 6: Authentication

**Read: `pkg/auth/auth.go` (110 lines)**

- JWT tokens with HS256 signing, 24-hour expiry
- bcrypt for password hashing (industry standard, intentionally slow to resist brute force)
- `RoleAtLeast()` — maps roles to levels (viewer=1, editor=2, admin=3) and compares. An admin passes a "requires editor" check because 3 >= 2

### Step 7: HTTP Layer

**Read: `pkg/api/middleware.go` (128 lines)**

Middleware runs on every request in order:
1. `Logger` — logs method, path, status code, duration
2. `CORS` — allows any origin (permissive for dev)
3. `MaxBodySize` — 1MB limit to prevent abuse
4. `AuthOptional` — extracts JWT from `Authorization: Bearer <token>` header if present, stores claims in request context

Then specific routes add:
5. `AuthRequired` — returns 401 if no valid JWT
6. `RequireRole("editor")` — returns 403 if role is insufficient

**Read: `pkg/api/routes.go` (145 lines)**

Chi router configuration. Important: `POST /api/rules/import` must be registered before `/api/rules/{id}` because Chi matches in order and "import" would otherwise match as an ID parameter.

Public routes: health, list rules, get rule, evaluate, batch evaluate, history, versions, export
Editor+ routes: create, update, delete, rollback, duplicate, import
Admin routes: register user, list users, webhook CRUD

**Read: `pkg/api/handlers.go` (350 lines)**

The meat of the API. Most handlers follow the same pattern: parse input → validate → call store → return JSON.

The interesting handler is `evaluateComposite()` (line 288). Composite evaluation happens here (not in the engine package) because it needs to fetch child rules from the store. The engine package stays pure... it only evaluates trees, never touches the database.

`BatchEvaluate()` (line 232) uses a worker pool with a semaphore channel (`sem := make(chan struct{}, 10)`) to limit concurrency to 10 goroutines.

**Read: `pkg/api/handlers_auth.go` (104 lines) and `pkg/api/handlers_webhook.go` (72 lines)**

Standard handlers. Login checks bcrypt hash, generates JWT. Register is admin-only. Webhook create generates a random HMAC secret.

### Step 8: Webhooks

**Read: `pkg/webhooks/fire.go` (113 lines)**

`Fire()` launches a goroutine (async, doesn't block the HTTP response). Inside:
1. Fetch active webhook subscriptions matching the event
2. For each webhook: marshal payload → compute HMAC-SHA256 signature → POST with 10-second timeout

The signature is `hex(HMAC-SHA256(webhook_secret, payload_bytes))` in the `X-Arbiter-Signature` header. Receivers can verify the signature to ensure the payload wasn't tampered with.

### Step 9: Entry point

**Read: `cmd/arbiter/main.go` (84 lines)**

Ties everything together. Reads env vars (`PORT`, `DB_PATH`, `ARBITER_JWT_SECRET`), creates the store, runs migrations + seeds, builds the router, starts the server with graceful shutdown on SIGINT/SIGTERM.

### Step 10: WASM

**Read: `cmd/wasm/main.go` (83 lines)**

Thin bridge between JavaScript and the engine. Registers `arbiter.evaluate()` and `arbiter.validateRule()` on the global JS object. Uses `syscall/js` to convert between Go and JS types. The `select {}` at the end keeps the Go runtime alive (WASM modules exit when main returns).

---

## Concurrency patterns to understand

1. **Read/write DB separation** — 1 writer, 4 readers. SQLite limitation turned into a feature
2. **Async eval history** — `go h.Store.InsertEvalHistory(...)` fires a goroutine so the evaluation response isn't delayed by database writes
3. **Worker pool** — Batch evaluate uses `chan struct{}` as a semaphore to cap goroutines at 10
4. **Regex cache** — `sync.RWMutex` protects a map of compiled regex patterns. Read lock for cache hits, write lock for cache misses
5. **Prune counter** — Atomic counter per rule. Every 100 history inserts, prune old entries. Avoids expensive COUNT queries

## Testing

```bash
go test ./... -count=1          # run all tests
go test ./pkg/engine/ -v         # engine tests with verbose output
go test ./pkg/auth/ -v           # auth tests
go test ./pkg/store/ -v          # store integration tests (uses real SQLite)
```

Tests use real SQLite databases (created in temp dirs), not mocks. This means the tests catch real issues like SQL syntax errors and type mismatches.
