# Beginner's Guide: Reading the Arbiter Codebase

You're an SDE-1 with about a year of experience. This guide tells you exactly how to read through the entire Arbiter project, what to read in what order, what to understand at each step, and why things are built the way they are. No jargon without explanation.

## What is Arbiter?

Arbiter is a **configuration management system** (like PhonePe's Chimera). It lets you control application behavior without deploying new code. Think of it like a remote control for your app.

Three examples of what it does:
1. **Feature flags** - "Show the new checkout flow to 25% of users" (without deploying)
2. **Decision trees** - "If the user has 100+ employees, show enterprise pricing. Otherwise, show team pricing."
3. **Kill switches** - "Turn off the recommendation engine during an incident" (one click)

The key idea: your app asks Arbiter "what should I do?" at runtime, and Arbiter evaluates a tree of conditions against the user's context (who they are, what plan they're on, what country they're from) to return a decision.

## How to read this project

Read in this exact order. Each step builds on the previous one.

---

### Phase 1: Understand the data model (30 min)

**Start here. Everything else references these types.**

#### 1. `pkg/engine/types.go`

This file defines every data structure in the system. The important ones:

- **Rule** - The main thing. Has an ID, name, type (feature_flag/decision_tree/kill_switch/composite), and a `tree` field that contains the decision logic as JSON.
  - `Environment` field (production/staging/development) - rules belong to an environment, like how you have different configs for prod vs staging
  - `ActiveFrom` / `ActiveUntil` - optional time window. A rule can be scheduled to activate at midnight and deactivate after a sale ends, for example
  - `Status` - active (in use), draft (being built), disabled (turned off)

- **Node** - A single step in the decision tree. Either a **leaf** (returns a value, like `true` or `"enterprise"`) or a **branch** (has a condition, a "then" path, and an "else" path). Like an if/else in code, but stored as data.

- **Condition** - "Is user.age greater than 18?" → `{field: "user.age", op: "gt", value: 18}`. Can also be a group: "age > 18 AND country in [US, CA]" using a **combinator** (AND/OR logic).

- **EvalResult** - What you get back after evaluating a rule. Contains the `value` (the decision), the `path` (every step the engine took to reach that decision, for debugging), and how long it took.

**Why this file matters:** Every other file in the project either creates, stores, evaluates, or displays these types. If you understand them, the rest clicks.

#### 2. `pkg/engine/context.go` (tiny, 26 lines)

One function: `GetField()`. It takes a context like `{"user": {"age": 25}}` and a dot-path like `"user.age"` and returns `25`. This is how conditions reach into nested data.

---

### Phase 2: Understand the evaluation engine (45 min)

**This is the core algorithm. The heart of the system.**

#### 3. `pkg/engine/evaluate.go`

Read top to bottom. Four functions:

1. `Evaluate()` - Entry point. Takes the tree JSON + context. Returns an EvalResult.
2. `evalNode()` - Recursive. "Is this a leaf? Return the value. Is this a branch? Check the condition, then go into the `then` or `else` child." This recursion IS the tree walk.
3. `evalCondition()` - Checks if a condition is a single comparison or a group (AND/OR), dispatches accordingly.
4. `evalCombinator()` - AND logic: stop on first false (short-circuit). OR logic: stop on first true.

The `path` slice tracks every decision. This is what makes Arbiter debuggable... you can see "user.age gt 18 -> true, user.country in [US,CA] -> false, -> else branch".

#### 4. `pkg/engine/operators.go`

10 comparison operators. The straightforward ones (eq, gt, lt, etc.) are simple comparisons. The interesting ones:

- `pct` (percentage) - For gradual rollouts. "Show this to 25% of users." Uses a hash function (FNV-1a) to turn the user ID into a number 0-99. If the number is less than 25, they're in the 25%. The same user always gets the same number for the same rule (deterministic), but different rules give different numbers (decorrelated). This is clever because you don't need to store per-user state.
- `regex` - Pattern matching with a cache. Compiling regex patterns is expensive, so Arbiter caches compiled patterns in a thread-safe map. First evaluation compiles, subsequent ones reuse.

#### 5. `pkg/engine/validate.go`

Runs before saving any rule. Enforces limits:
- Tree can't be deeper than 20 levels (prevents stack overflow during evaluation)
- AND/OR groups can nest at most 3 deep (prevents insane complexity)
- Max 10 conditions per group
- Regex patterns must actually compile
- Percentage values must be 0-100

#### 6. `pkg/engine/compose.go`

Composite rules combine multiple child rules. Four strategies:
- `all_true` - All children must return true (like AND)
- `any_true` - At least one child returns true (like OR)
- `first_match` - Return the first child that returns a non-default value
- `merge_results` - Return a map of {rule_id: value} for all children

Also has cycle detection. If rule A references rule B, and B references A, that's an infinite loop. `DetectCycles()` uses DFS (depth-first search) with three-state coloring to catch this before saving.

---

### Phase 3: Understand persistence (45 min)

**How rules get stored and retrieved from the database.**

#### 7. `pkg/store/migrations.go`

The database schema. 6 tables:
- `rules` - Current state of each rule
- `rule_versions` - Immutable snapshots (every edit creates a new version)
- `eval_history` - Past evaluation results for debugging
- `users` - User accounts with roles
- `webhook_subscriptions` - URLs to notify when rules change
- `_meta` - Key-value store for system state (like "has the database been seeded?")

Read this before looking at queries. It tells you what data exists and how tables relate to each other.

#### 8. `pkg/store/sqlite.go`

The biggest file. All database operations. Key patterns to understand:

**Two connection pools** - SQLite only allows one writer at a time. So Arbiter opens TWO connections to the same database: `writeDB` (max 1 connection, for inserts/updates/deletes) and `readDB` (max 4 connections, for selects). This means reads never block writes, and writes never block reads. This is called WAL (Write-Ahead Logging) mode.

**Automatic versioning** - Every `UpdateRule()` does two things in one transaction: (1) updates the rule, (2) inserts a snapshot into `rule_versions`. If either fails, both roll back. You never lose history.

**Rollback** - Doesn't delete anything. "Rollback to v1" when you're at v3 copies v1's data into the current rule and creates v4. So your history is: v1, v2, v3, v4(=v1's data). Nothing is ever lost.

**Environment filtering** - `ListRules()` conditionally adds `WHERE environment = ?` when a filter is provided. Empty string means "show all."

**Audit trail** - Every version records who made the change (`modified_by`), pulled from the JWT token of the logged-in user.

**Nullable time handling** - `active_from` and `active_until` can be null (meaning "no schedule"). SQLite doesn't have a native nullable time type, so Go uses `sql.NullTime` to handle this.

#### 9. `pkg/store/seeds.go`

Creates 4 example rules on first run so the dashboard isn't empty. Also creates a default admin user (admin/admin). Uses the `_meta` table to track whether seeding already happened.

#### 10. `pkg/store/users.go` and `pkg/store/webhooks.go`

Simple CRUD. User management and webhook subscriptions. Note: `GetUserByUsername` returns the password hash (needed for login), but `ListUsers` doesn't (you should never send password hashes to the frontend).

---

### Phase 4: Understand authentication (20 min)

#### 11. `pkg/auth/auth.go`

Two things here:
- **JWT tokens** - When you log in, the server creates a signed token (JSON Web Token) containing your username and role. The token expires in 24 hours. Every API request sends this token in the `Authorization` header. The server validates the signature (using HMAC-SHA256) to confirm the token wasn't tampered with.
- **bcrypt passwords** - Passwords are hashed with bcrypt before storage. bcrypt is intentionally slow (that's the feature). Even if someone steals the database, they can't recover passwords quickly.
- **Roles** - Three levels: viewer (read-only), editor (can modify rules), admin (everything). `RoleAtLeast()` compares levels numerically: admin(3) >= editor(2) >= viewer(1).

---

### Phase 5: Understand the HTTP layer (30 min)

**This is the glue between the frontend and everything else.**

#### 12. `pkg/api/middleware.go`

Every request passes through middleware (functions that run before the handler):
1. `Logger` - Records how long each request took
2. `CORS` - Allows the frontend to talk to the backend (browser security thing)
3. `MaxBodySize` - Limits request bodies to 1MB (prevents abuse)
4. `AuthOptional` - Extracts the JWT token if present, stores the user info in the request context
5. `AuthRequired` - Returns 401 if no valid token (used on write endpoints)
6. `RequireRole` - Returns 403 if the user's role isn't high enough

#### 13. `pkg/api/routes.go`

Maps URLs to handler functions. Important detail: `POST /api/rules/import` must be registered BEFORE `/api/rules/{id}` because the router matches in order, and "import" would otherwise be treated as a rule ID.

Route groups:
- Public: health check, list rules, get rule, evaluate
- Editor+: create, update, delete, rollback, duplicate, import
- Admin: register user, list users, webhook management

#### 14. `pkg/api/handlers.go`

The main handler file. Most handlers follow the same pattern: parse input, validate, call store, return JSON.

The interesting handler is `EvaluateRule()`:
1. Fetches the rule from the store
2. Checks `IsScheduleActive()` - if the rule is outside its time window, returns the default value immediately
3. Calls `engine.Evaluate()` with the tree and context
4. Fires `go store.InsertEvalHistory()` (async, doesn't slow down the response)
5. Returns the result

`BatchEvaluate()` evaluates multiple rules in parallel using a worker pool (max 10 goroutines).

#### 15. Other handler files

- `handlers_auth.go` - Login checks bcrypt hash, generates JWT. Register is admin-only.
- `handlers_webhook.go` - Webhook CRUD with auto-generated HMAC secrets.
- `handlers_version.go` - Version listing and rollback.
- `handlers_import.go` - Export downloads JSON, import uploads JSON (with conflict detection).
- `handlers_duplicate.go` - Copies a rule with a new ID.

---

### Phase 6: Understand webhooks (10 min)

#### 16. `pkg/webhooks/fire.go`

When a rule is created/updated/deleted, Arbiter notifies external systems. The `Fire()` method launches a goroutine (async, doesn't block the API response). For each matching webhook subscription:
1. Marshal the payload to JSON
2. Compute HMAC-SHA256 signature using the webhook's secret key
3. Send an HTTP POST with a 10-second timeout

The signature goes in the `X-Arbiter-Signature` header. Receivers can verify the payload wasn't tampered with by computing the same HMAC and comparing.

---

### Phase 7: Understand the entry point (5 min)

#### 17. `cmd/arbiter/main.go`

Ties everything together: reads environment variables (PORT, DB_PATH, JWT_SECRET), creates the database store, runs seeds, builds the HTTP router, starts the server with graceful shutdown (waits up to 5 seconds for in-flight requests before stopping).

---

### Phase 8: Understand the frontend (30 min)

Read `docs/frontend-overview.md` for the full breakdown. Quick summary:

- React 18 SPA with Vite, no router library (single page with tabs)
- `api/client.ts` - Every API call goes through one `request()` function that adds auth headers and handles errors
- `hooks/useAuth.tsx` - Auth state management (login, logout, token validation)
- `hooks/useRules.ts` - Fetches and caches the rule list, supports environment filtering
- `components/Editor.tsx` - JSON editor (CodeMirror) with environment dropdown and schedule pickers
- `components/TreeEditor.tsx` - Visual drag-and-drop tree editor (ReactFlow)
- `components/HistoryView.tsx` - Version history with "by {username}" audit labels
- `components/UserManager.tsx` - Admin-only user management (create/view users)
- `components/WebhookManager.tsx` - Admin-only webhook management

The entire frontend compiles into the Go binary via `//go:embed`. No separate frontend server needed.

---

### Phase 9: WASM and SDK (optional, 15 min)

#### 18. `cmd/wasm/main.go`

Compiles the evaluation engine to WebAssembly. This means you can evaluate rules in the browser without hitting the server. Two JS functions: `evaluate()` and `validateRule()`. The `wasm/arbiter-loader.js` file loads and wraps the WASM binary.

#### 19. `sdk/arbiter/`

Go client library for the API. Wraps all 14 endpoints with typed functions and JWT auth support. Useful for other Go services that need to fetch and evaluate rules.

---

## Mental model

Think of Arbiter as four concentric circles:

```
┌──────────────────────────────────┐
│  Frontend (React Dashboard)      │  ← What users see and interact with
│  ┌──────────────────────────┐    │
│  │  API (HTTP handlers)     │    │  ← Receives requests, coordinates everything
│  │  ┌──────────────────┐    │    │
│  │  │  Store (SQLite)   │    │    │  ← Persists rules, users, history
│  │  │  ┌──────────┐    │    │    │
│  │  │  │  Engine   │    │    │    │  ← Pure evaluation logic (the brain)
│  │  │  └──────────┘    │    │    │
│  │  └──────────────────┘    │    │
│  └──────────────────────────┘    │
└──────────────────────────────────┘
```

The inner circle (Engine) has zero dependencies. It doesn't know about databases, HTTP, or React. You could drop it into a WASM binary or a CLI tool and it would work the same. That's the whole design goal.

## Quick reference: run commands

```bash
# Run the server
make run                          # or: go run ./cmd/arbiter/

# Run all tests
make test                         # or: go test ./... -count=1

# Build the frontend
cd web && npm run build

# Build WASM
make wasm

# Build the full binary (includes embedded frontend)
make build
```

## Common patterns you'll see everywhere

1. **`json.RawMessage`** - Delays JSON parsing. The tree is stored as raw bytes until evaluation time. This avoids parsing JSON on every database read when you might not even evaluate the rule.

2. **`context.Context`** as first parameter - Go convention. Carries request-scoped data (deadlines, cancellation, auth claims) through function calls.

3. **`writeError(w, status, message)`** - Every error response uses this helper. Ensures consistent `{"error": "message"}` JSON format.

4. **`getUsername(r)`** - Extracts the logged-in user's name from the request context. Used to populate `modified_by` on every rule change.

5. **`go func() { ... }()`** - Goroutines for async work (eval history writes, webhook delivery). The main request doesn't wait for these to finish.
