# Architecture

This doc explains how Arbiter's pieces fit together, how a request flows through the system, and the concurrency patterns used. Read this first if you're new to the codebase.

## Directory Map

```
arbiter/
├── cmd/arbiter/              # Entry point
│   ├── main.go               # Server startup, graceful shutdown, env config
│   ├── embed.go              # Production: loads embedded web assets
│   └── embed_dev.go          # Dev mode: loads from filesystem (or nil)
│
├── pkg/
│   ├── engine/               # Core logic (zero dependencies on store or HTTP)
│   │   ├── types.go          # Data structs: Rule, Node, Condition, EvalResult
│   │   ├── evaluate.go       # Tree walker: Evaluate() → evalNode() → evalCondition()
│   │   ├── operators.go      # 10 operators: eq, neq, gt, gte, lt, lte, in, nin, regex, pct
│   │   ├── context.go        # GetField(): dot-notation field extraction from context map
│   │   ├── validate.go       # ValidateRule(): checks tree structure, operator values, depth limits
│   │   └── evaluate_test.go  # 18 unit tests
│   │
│   ├── store/                # SQLite persistence layer
│   │   ├── sqlite.go         # Store struct, CRUD, versioning, eval history, import
│   │   ├── migrations.go     # DDL schema (CREATE TABLE IF NOT EXISTS)
│   │   ├── seeds.go          # 4 example rules seeded on first run
│   │   └── sqlite_test.go    # 22 integration tests
│   │
│   └── api/                  # HTTP layer (depends on engine + store)
│       ├── routes.go         # Chi router setup, SPA fallback handler
│       ├── handlers.go       # Core: CRUD, evaluate, batch evaluate, history
│       ├── handlers_version.go   # Version list, rollback
│       ├── handlers_import.go    # Export (download JSON), import (upload JSON)
│       ├── handlers_duplicate.go # Rule duplication with auto-naming
│       └── middleware.go     # Logger, CORS, MaxBodySize (1MB)
│
├── web/                      # React frontend (embedded in Go binary)
│   ├── embed.go              # Production: //go:embed all:dist
│   ├── embed_dev.go          # Dev: os.DirFS("web/dist") or nil
│   ├── src/
│   │   ├── App.tsx           # Main layout: sidebar + tabbed content + toasts
│   │   ├── api/client.ts     # TypeScript API client (all 14 endpoints)
│   │   ├── hooks/useRules.ts # React hook for rule state management
│   │   ├── components/
│   │   │   ├── RuleList.tsx      # Sidebar: list of rules + create form
│   │   │   ├── Editor.tsx        # Rule editor with CodeMirror for tree JSON
│   │   │   ├── TestPanel.tsx     # Live evaluation: enter context, see result
│   │   │   ├── TreeView.tsx      # Visual tree rendering with react-d3-tree
│   │   │   ├── HistoryView.tsx   # Version history + evaluation history tabs
│   │   │   ├── DiffView.tsx      # Side-by-side version diff
│   │   │   ├── PathView.tsx      # Renders evaluation path trace
│   │   │   └── TopBar.tsx        # Header bar
│   │   └── styles/global.css # Dark theme CSS
│   ├── package.json
│   └── vite.config.ts
│
├── examples/                 # Reference JSON files (not used by seed logic)
├── Dockerfile                # Multi-stage: node → go → alpine
├── .github/workflows/ci.yml  # Go test + frontend build + binary build
└── go.mod
```

## Layer Boundaries

Arbiter has three backend layers. Each layer only depends on the one below it:

```
┌─────────────────────────────────────────┐
│  api/  (HTTP handlers, routing)         │  ← Depends on engine + store
├─────────────────────────────────────────┤
│  store/  (SQLite CRUD, versioning)      │  ← Depends on engine (for types)
├─────────────────────────────────────────┤
│  engine/  (evaluate, operators, types)  │  ← Zero dependencies (pure logic)
└─────────────────────────────────────────┘
```

**Why this matters:**

- `engine/` has no imports from `store/` or `api/`. You can use the engine as a library without a database or HTTP server. This makes it testable in isolation.
- `store/` imports `engine/` only for the type definitions (`Rule`, `EvalResult`, etc.). It doesn't call `Evaluate()`.
- `api/` is the glue. It reads HTTP requests, calls store for persistence, calls engine for evaluation, and writes HTTP responses.

If you're reading the code for the first time, start with `engine/types.go` (the data model), then `engine/evaluate.go` (the core algorithm), then `store/sqlite.go` (how it's persisted), then `api/handlers.go` (how it's exposed).

## Request Flow: Evaluate a Rule

Here's what happens when a client calls `POST /api/rules/pricing_tier/evaluate`:

```
Client
  │
  │  POST /api/rules/pricing_tier/evaluate
  │  Body: {"context": {"org": {"employees": 50}}}
  │
  ▼
routes.go                    Chi matches /api/rules/{id}/evaluate
  │
  ▼
middleware.go                Logger → CORS → MaxBodySize (1MB limit)
  │
  ▼
handlers.go:EvaluateRule()
  │
  ├─► store.GetRule(ctx, "pricing_tier")
  │     │
  │     └─► readDB.QueryRowContext()     ← reads from the read pool
  │           returns Rule with tree JSON
  │
  ├─► json.Decode(body) → extract context map
  │
  ├─► engine.Evaluate(rule.Tree, context, rule.ID, rule.DefaultValue)
  │     │
  │     ├─► json.Unmarshal(tree) → Node struct
  │     │
  │     ├─► evalNode(root, ctx, ..., depth=0)
  │     │     │
  │     │     ├─► Is leaf? (HasValue == true) → return value
  │     │     │
  │     │     ├─► evalCondition(condition, ctx, ruleID)
  │     │     │     │
  │     │     │     ├─► GetField(ctx, "org.employees") → 50
  │     │     │     │
  │     │     │     └─► EvalOperator("gte", 50, 100, "pricing_tier") → false
  │     │     │
  │     │     ├─► path += "org.employees gte 100 → false"
  │     │     │
  │     │     └─► evalNode(node.Else, ctx, ..., depth=1)  ← recurse into else
  │     │           │
  │     │           ├─► EvalOperator("gte", 50, 10, ...) → true
  │     │           ├─► path += "org.employees gte 10 → true"
  │     │           └─► evalNode(then, ..., depth=2) → "team"
  │     │
  │     └─► return EvalResult{Value: "team", Path: [...], Elapsed: "29µs"}
  │
  ├─► go store.InsertEvalHistory(...)    ← async goroutine, doesn't block response
  │
  └─► writeJSON(200, result)
        │
        ▼
      Client receives:
      {"value": "team", "path": [...], "default": false, "elapsed": "29µs"}
```

Key things to notice:

1. **Rule fetch uses the read pool** (`readDB`), evaluation is pure in-memory, history insert uses the write pool (`writeDB`).
2. **Eval history is async.** The `go` keyword fires a goroutine so the client gets the response immediately without waiting for the database write.
3. **The engine never touches the database.** It receives a `json.RawMessage` tree and a `map[string]any` context. Pure function.

## Request Flow: Update a Rule

```
PUT /api/rules/pricing_tier
  │
  ▼
handlers.go:UpdateRule()
  │
  ├─► json.Decode(body) → Rule struct
  ├─► engine.ValidateRule(&rule)         ← checks tree depth, operators, etc.
  │
  ├─► store.UpdateRule(ctx, &rule)
  │     │
  │     ├─► writeDB.BeginTx()           ← BEGIN IMMEDIATE (single writer)
  │     ├─► SELECT version FROM rules   ← get current version (e.g., 3)
  │     ├─► UPDATE rules SET ... version=4
  │     ├─► INSERT INTO rule_versions   ← immutable snapshot of v4
  │     └─► tx.Commit()                 ← atomic: both succeed or both fail
  │
  └─► store.GetRule() → writeJSON(200, updated rule)
```

Every update atomically increments the version and creates a snapshot in `rule_versions`. If either the UPDATE or the INSERT fails, the transaction rolls back and nothing changes.

## Concurrency Patterns

Arbiter uses four distinct concurrency patterns. Each solves a specific problem.

### 1. Two SQLite Connection Pools

**Problem:** SQLite allows many concurrent readers but only one writer. Go's `database/sql` pools connections. If a single pool has 4 connections, a write could block reads, or multiple writes could contend.

**Solution:** Two separate `*sql.DB` instances:

```go
// store/sqlite.go
writeDB.SetMaxOpenConns(1)   // one writer, serialized
readDB.SetMaxOpenConns(4)    // four concurrent readers
```

- All `SELECT` queries go through `readDB` (GetRule, ListRules, ListVersions, etc.)
- All `INSERT/UPDATE/DELETE` go through `writeDB` (CreateRule, UpdateRule, DeleteRule, etc.)
- Both pools run the same PRAGMAs (WAL mode, foreign keys, etc.)

The DSN includes `_txlock=immediate`, which means `writeDB.BeginTx()` issues `BEGIN IMMEDIATE` instead of the default deferred `BEGIN`. This acquires the write lock immediately instead of waiting until the first write statement. Prevents a subtle deadlock where two deferred transactions both read, then both try to write.

### 2. Async Eval History with Prune Counter

**Problem:** Recording eval history shouldn't slow down the response. But we also need to prune old entries (1,000 max per rule) without pruning on every insert.

**Solution:**

```go
// handlers.go — fire and forget
go h.Store.InsertEvalHistory(r.Context(), id, ctxJSON, resultJSON)

// store/sqlite.go — prune every 100th insert
pruneCounters sync.Map        // map[string]*atomic.Int64
counter.Add(1)
if count % 100 == 0 {
    s.pruneEvalHistory(ctx, ruleID)
}
```

- `sync.Map` holds per-rule counters. No global lock, each rule has its own counter.
- `atomic.Int64` allows lock-free incrementing. Multiple goroutines can safely increment.
- Every 100th insert triggers a prune: `DELETE WHERE id NOT IN (SELECT id ... LIMIT 1000)`.
- The `go` keyword means the HTTP response returns immediately. The database write happens in the background.

### 3. Worker Pool for Batch Evaluate

**Problem:** `POST /api/evaluate` evaluates multiple rules in one call. Each evaluation is independent (read-only). Doing them sequentially wastes time.

**Solution:** Semaphore channel pattern:

```go
// handlers.go:BatchEvaluate()
sem := make(chan struct{}, 10)    // max 10 concurrent goroutines

for i, id := range body.RuleIDs {
    sem <- struct{}{}             // acquire slot (blocks if 10 are running)
    go func(idx int, ruleID string) {
        defer func() { <-sem }()  // release slot when done
        // ... evaluate rule ...
        ch <- indexedResult{idx, result}
    }(i, id)
}
```

- The buffered channel `sem` acts as a counting semaphore. Capacity 10 means at most 10 goroutines run concurrently.
- `sem <- struct{}{}` blocks when the buffer is full, naturally throttling concurrency.
- Results are collected via `ch` channel with an index to preserve order.
- Individual failures don't kill the batch. Each result slot gets its own error.

### 4. Regex Cache with RWMutex

**Problem:** Compiling regexes is expensive. Rules with `regex` operators get evaluated many times with the same pattern.

**Solution:**

```go
// operators.go
var (
    regexMu    sync.RWMutex
    regexCache = make(map[string]*regexp.Regexp)
)

func getCompiledRegex(pattern string) (*regexp.Regexp, error) {
    regexMu.RLock()                    // many readers can hold this simultaneously
    if re, ok := regexCache[pattern]; ok {
        regexMu.RUnlock()
        return re, nil
    }
    regexMu.RUnlock()

    re, err := regexp.Compile(pattern)
    // ...
    regexMu.Lock()                     // exclusive write lock
    regexCache[pattern] = re
    regexMu.Unlock()
    return re, nil
}
```

- `RLock()` for reads: many goroutines can check the cache simultaneously (no contention during evaluation).
- `Lock()` for writes: only one goroutine compiles and stores a new regex at a time.
- Cache is cleared entirely when a rule is deleted (`ClearRegexCache()`), because the pattern might have been the only reference.

## The Embed Mechanism

Arbiter embeds the React dashboard inside the Go binary using `//go:embed`. This is how a single binary serves both the API and the frontend.

### How it works:

```
web/embed.go (production, build tag: !dev):
  //go:embed all:dist        ← compiler includes all files from web/dist/
  var distFS embed.FS         ← in-memory filesystem baked into the binary

web/embed_dev.go (dev mode, build tag: dev):
  return os.DirFS("web/dist") ← reads from actual filesystem (or nil)
```

The `cmd/arbiter/embed.go` and `cmd/arbiter/embed_dev.go` files call `web.DistFS()` and pass the result to the router.

**Why two packages?** Go's `//go:embed` can only reference files in or below the package's directory. `cmd/arbiter/` can't embed `../../web/dist/`. So the `web/` package owns the embed directive, and `cmd/arbiter/` imports it.

### Build tag mechanics:

```bash
go build ./cmd/arbiter/               # production: uses embed.go (!dev)
go run -tags dev ./cmd/arbiter/       # dev mode: uses embed_dev.go (dev)
```

The `!dev` tag means "included by default, excluded when dev is set." You don't need to pass any tag for production builds.

### SPA fallback routing:

The React app uses client-side routing. If a user bookmarks `/rules/pricing_tier` and hits refresh, the browser sends `GET /rules/pricing_tier` to the server. Without SPA fallback, that's a 404.

`routes.go` handles this:

1. If the path starts with `/api/`, route normally
2. If the path matches a real file in `web/dist/` (JS, CSS, images), serve it
3. Otherwise, serve `index.html` and let React's router handle it

## Middleware Stack

Every request passes through three middleware layers in order:

```
Request → Logger → CORS → MaxBodySize → Handler → Response
```

1. **Logger** (`middleware.go:10`): Wraps `ResponseWriter` to capture status code. Logs `METHOD /path STATUS duration` after the handler completes.
2. **CORS** (`middleware.go:20`): Adds `Access-Control-Allow-Origin: *` headers. Handles OPTIONS preflight with 204. Permissive for development.
3. **MaxBodySize** (`middleware.go:34`): Wraps `r.Body` with `http.MaxBytesReader(1MB)`. Any request body over 1MB returns 413.

## Database Schema Relationships

```
rules (1) ──────── (N) rule_versions
  │                      │
  │ id ←─── rule_id ─────┘   ON DELETE CASCADE
  │
  │ id ←─── rule_id ─────┐
  │                      │
rules (1) ──────── (N) eval_history
                              ON DELETE CASCADE

_meta (standalone key-value table, tracks seed status)
```

- Deleting a rule cascades to all its versions and eval history. One `DELETE` cleans everything.
- `rule_versions` has a `UNIQUE(rule_id, version)` constraint. Can't have two v3s for the same rule.
- `PRAGMA foreign_keys = ON` is set on every connection. Without it, SQLite ignores foreign keys entirely (it defaults to OFF).

## Error Handling Pattern

All API errors follow the same shape:

```json
{"error": "human-readable message"}
```

HTTP status codes used:
- `400` Bad Request: invalid JSON, validation failure
- `404` Not Found: rule/version doesn't exist
- `409` Conflict: rule ID collision on create/import (import includes `existing` rule metadata)
- `500` Internal Server Error: database failures
- `204` No Content: CORS preflight

The `writeError` helper in `handlers.go:28` enforces this. Every handler uses it for error responses.
