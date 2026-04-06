# System Architecture

Full technical architecture of Arbiter: layer diagrams, data flow, API surface, concurrency patterns, and design decisions.

## Directory Map

```
arbiter/
├── cmd/arbiter/              # Entry point
│   ├── main.go               # Server startup, graceful shutdown, env config
│   ├── embed.go              # Production: loads embedded web assets
│   └── embed_dev.go          # Dev mode: loads from filesystem (or nil)
│
├── cmd/wasm/                 # WebAssembly build target
│   └── main.go               # Bridges Go engine ↔ JavaScript
│
├── pkg/
│   ├── engine/               # Core logic (zero dependencies on store or HTTP)
│   │   ├── types.go          # Data structs: Rule, Node, Condition, EvalResult
│   │   ├── evaluate.go       # Tree walker: Evaluate() → evalNode() → evalCondition()
│   │   ├── operators.go      # 10 operators: eq, neq, gt, gte, lt, lte, in, nin, regex, pct
│   │   ├── context.go        # GetField(): dot-notation field extraction
│   │   ├── validate.go       # ValidateRule(): structure, operators, depth limits
│   │   ├── compose.go        # Composite rules: strategies + cycle detection
│   │   └── evaluate_test.go  # 18 unit tests
│   │
│   ├── store/                # SQLite persistence layer
│   │   ├── sqlite.go         # Store struct, CRUD, versioning, eval history
│   │   ├── migrations.go     # DDL schema (6 tables, 3 indexes)
│   │   ├── seeds.go          # 4 example rules + default admin user
│   │   ├── users.go          # User CRUD
│   │   ├── webhooks.go       # Webhook subscription CRUD
│   │   └── sqlite_test.go    # 22 integration tests
│   │
│   ├── api/                  # HTTP layer (depends on engine + store)
│   │   ├── routes.go         # Chi router, SPA fallback
│   │   ├── handlers.go       # CRUD, evaluate, batch evaluate
│   │   ├── handlers_auth.go  # Login, register, me, list users
│   │   ├── handlers_webhook.go  # Webhook CRUD
│   │   ├── handlers_version.go  # Version list, rollback
│   │   ├── handlers_import.go   # Export/import JSON
│   │   ├── handlers_duplicate.go # Rule duplication
│   │   └── middleware.go     # Logger, CORS, MaxBodySize, Auth, RBAC
│   │
│   ├── auth/                 # Authentication + authorization
│   │   └── auth.go           # JWT generation/validation, bcrypt, role hierarchy
│   │
│   └── webhooks/             # Event notifications
│       └── fire.go           # Async HTTP delivery with HMAC signatures
│
├── sdk/arbiter/              # Go client library
├── wasm/                     # WASM loader + example
├── web/                      # React frontend (embedded in binary)
│   ├── src/
│   │   ├── App.tsx           # Auth gate + dashboard layout
│   │   ├── api/client.ts     # TypeScript HTTP client
│   │   ├── hooks/            # useAuth, useRules
│   │   └── components/       # Editor, TreeEditor, HistoryView, UserManager, etc.
│   └── dist/                 # Built output (embedded via //go:embed)
│
├── docs/                     # Documentation
├── Makefile                  # build, run, test, web, wasm, clean
├── Dockerfile                # Multi-stage: node → go → alpine
└── .github/workflows/ci.yml  # Go test + frontend build + binary build
```

## Layer Architecture

Arbiter has strict layer boundaries. Each layer only depends on the one below it. No layer reaches upward.

```
┌─────────────────────────────────────────────────────────────┐
│                    React Dashboard                          │
│  TypeScript SPA embedded in Go binary via //go:embed        │
│  Components: Editor, TreeEditor, TestPanel, HistoryView,    │
│  UserManager, WebhookManager, RuleList, TopBar              │
└──────────────────────────┬──────────────────────────────────┘
                           │ HTTP (fetch → /api/*)
┌──────────────────────────▼──────────────────────────────────┐
│  api/  — HTTP handlers, routing, middleware                 │
│                                                             │
│  Responsibilities:                                          │
│  • Parse HTTP requests, validate input                      │
│  • Call store for persistence                               │
│  • Call engine for evaluation                               │
│  • Enforce auth/RBAC via middleware                          │
│  • Fire webhook events after mutations                      │
│  • Check rule schedules before evaluation                   │
│  • Track modified_by from JWT claims                        │
├─────────────────────────────────────────────────────────────┤
│  store/  — SQLite persistence                               │
│                                                             │
│  Responsibilities:                                          │
│  • CRUD for rules, users, webhooks                          │
│  • Automatic versioning on every update                     │
│  • Evaluation history with auto-pruning                     │
│  • Environment filtering                                    │
│  • Read/write connection pool separation                    │
├─────────────────────────────────────────────────────────────┤
│  engine/  — Pure evaluation logic                           │
│                                                             │
│  Responsibilities:                                          │
│  • Tree evaluation (recursive node walking)                 │
│  • Operator comparison (10 operators)                       │
│  • Rule validation (structure, limits, types)               │
│  • Composite rule combination (4 strategies)                │
│  • Cycle detection in composite references                  │
│  • Decision path tracing                                    │
│                                                             │
│  Zero dependencies on store, api, or any I/O.               │
│  This is why WASM compilation works.                        │
└─────────────────────────────────────────────────────────────┘
```

**Why strict layers matter**: The engine package has no imports from store or api. You can use it as a standalone library, compile it to WASM, or test it without a database. The store package imports engine only for type definitions (Rule, EvalResult). It never calls Evaluate(). The api package is the only place where all three concerns (HTTP, persistence, evaluation) come together.

## Data Flow: Rule Evaluation

The most important flow in the system. Here's what happens when a client calls `POST /api/rules/pricing_tier/evaluate`:

```
Client sends:  POST /api/rules/pricing_tier/evaluate
               Body: {"context": {"org": {"employees": 50}}}

  │
  ▼
Middleware chain:
  Logger → CORS → MaxBodySize(1MB) → AuthOptional(extract JWT if present)
  │
  ▼
handlers.go → EvaluateRule()
  │
  ├─1─► store.GetRule(ctx, "pricing_tier")
  │       └─► readDB.QueryRowContext()                ← uses read pool (no write lock)
  │             returns Rule { tree: JSON, active_from, active_until, ... }
  │
  ├─2─► rule.IsScheduleActive()
  │       └─► Compares current UTC time against active_from/active_until
  │           If outside window → return default value with path ["rule outside scheduled activation window"]
  │           If inside window (or no schedule) → continue
  │
  ├─3─► json.Decode(request body) → extract context map
  │
  ├─4─► engine.Evaluate(rule.Tree, context, rule.ID, rule.DefaultValue)
  │       │
  │       ├─► json.Unmarshal(tree JSON) → Node struct (recursive)
  │       │
  │       ├─► evalNode(root, ctx, depth=0)
  │       │     │
  │       │     ├─► Is leaf? (HasValue==true) → return value
  │       │     │
  │       │     ├─► evalCondition(condition, ctx)
  │       │     │     ├─► GetField(ctx, "org.employees") → 50
  │       │     │     └─► EvalOperator("gte", 50, 100) → false
  │       │     │
  │       │     ├─► path += "org.employees gte 100 → false"
  │       │     │
  │       │     └─► evalNode(node.Else, ctx, depth=1)        ← recurse into else
  │       │           ├─► EvalOperator("gte", 50, 10) → true
  │       │           ├─► path += "org.employees gte 10 → true"
  │       │           └─► evalNode(then, depth=2) → "team"
  │       │
  │       └─► return EvalResult{ Value: "team", Path: [...], Elapsed: "29us" }
  │
  ├─5─► go store.InsertEvalHistory(...)               ← async goroutine (doesn't block response)
  │
  └─6─► writeJSON(200, result)
          │
          ▼
        Client receives:
        {"value": "team", "path": [...], "default": false, "elapsed": "29us"}
```

Key observations:
1. Rule fetch uses the **read pool** (no lock contention with writers)
2. Schedule check happens **before** evaluation (fail fast)
3. Evaluation is **pure in-memory** (no I/O)
4. History write is **async** (client doesn't wait)

## Data Flow: Rule Mutation

When a rule is updated via `PUT /api/rules/{id}`:

```
Client sends:  PUT /api/rules/pricing_tier
               Headers: Authorization: Bearer <JWT>
               Body: { name, description, type, status, environment, tree, active_from, active_until }

  │
  ▼
Middleware: Logger → CORS → MaxBodySize → AuthOptional → AuthRequired → RequireRole("editor")
  │
  ▼
handlers.go → UpdateRule()
  │
  ├─1─► json.Decode(body) → Rule struct
  │
  ├─2─► engine.ValidateRule(&rule)
  │       ├─► Check required fields (id, name, type)
  │       ├─► Validate tree structure (max depth 20)
  │       ├─► Validate operators, regex patterns, pct values
  │       └─► If composite: validate strategy + detect cycles
  │
  ├─3─► getUsername(r) → extract "admin" from JWT claims
  │
  ├─4─► store.UpdateRule(ctx, &rule, "admin")
  │       │
  │       ├─► writeDB.BeginTx()                ← BEGIN IMMEDIATE (exclusive write)
  │       ├─► SELECT version FROM rules        ← current version (e.g., 3)
  │       ├─► UPDATE rules SET ... version=4, environment=X, active_from=Y
  │       ├─► INSERT INTO rule_versions (version=4, modified_by="admin", ...)
  │       └─► tx.Commit()                      ← atomic: both succeed or both fail
  │
  ├─5─► webhooks.Fire("rule.updated", id, rule)  ← async goroutine
  │
  └─6─► store.GetRule() → writeJSON(200, updated rule)
```

## Data Flow: Authentication

```
Login:
  POST /api/auth/login { username, password }
    │
    ├─► store.GetUserByUsername() → { id, password_hash, role }
    ├─► bcrypt.CompareHashAndPassword(hash, password) → match?
    ├─► auth.GenerateToken(username, role) → JWT (HS256, 24h expiry)
    └─► return { token, username, role }

Authenticated request:
  GET /api/rules  (Authorization: Bearer <JWT>)
    │
    ├─► AuthOptional middleware:
    │     ├─► Extract "Bearer <token>" from header
    │     ├─► auth.ValidateToken(token) → Claims { username, role, expiry }
    │     └─► Store claims in request context
    │
    ├─► (for protected routes) AuthRequired middleware:
    │     └─► If no claims in context → 401 "authentication required"
    │
    └─► (for role-restricted routes) RequireRole("editor") middleware:
          └─► If claims.role < required_role → 403 "insufficient permissions"

Role hierarchy:
  admin (3) > editor (2) > viewer (1)
  RoleAtLeast(claims.role, "editor") → true if claims.role >= editor
```

## Data Flow: Webhook Delivery

```
Rule mutation (create/update/delete)
  │
  ├─► webhooks.Fire("rule.updated", ruleID, ruleData)
  │     │
  │     └─► go fireAsync()                    ← goroutine (doesn't block API response)
  │           │
  │           ├─► store.GetActiveWebhooks(ctx, "rule.updated")
  │           │     └─► SELECT * FROM webhook_subscriptions
  │           │         WHERE active=1 AND (events='*' OR events LIKE '%rule.updated%')
  │           │
  │           ├─► Build payload: { event, rule_id, timestamp, data }
  │           │
  │           └─► For each webhook (parallel goroutines):
  │                 ├─► json.Marshal(payload)
  │                 ├─► hmac.New(sha256, webhook.secret)
  │                 ├─► mac.Write(payloadBytes) → signature
  │                 ├─► HTTP POST to webhook.url
  │                 │     Headers: Content-Type: application/json
  │                 │              User-Agent: Arbiter-Webhook/1.0
  │                 │              X-Arbiter-Signature: sha256=<hex>
  │                 └─► Log errors (no retry, fire-and-forget)
  │
  └─► API response returns immediately (not waiting for webhooks)
```

## Implemented APIs

### Public Endpoints (no auth required)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/health` | Server health check |
| GET | `/api/rules?limit=50&offset=0&environment=production` | List rules with pagination and optional environment filter |
| GET | `/api/rules/{id}` | Get a single rule |
| POST | `/api/rules/{id}/evaluate` | Evaluate a rule against a context |
| POST | `/api/evaluate` | Batch evaluate multiple rules |
| GET | `/api/rules/{id}/history?limit=50&offset=0` | Evaluation history |
| GET | `/api/rules/{id}/versions` | List all versions |
| GET | `/api/rules/{id}/export` | Download rule as JSON |
| POST | `/api/auth/login` | Login, returns JWT |

### Editor+ Endpoints (requires editor or admin role)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/rules` | Create a new rule |
| PUT | `/api/rules/{id}` | Update a rule (creates new version) |
| DELETE | `/api/rules/{id}` | Delete a rule (cascades to versions + history) |
| POST | `/api/rules/{id}/rollback/{version}` | Rollback to a previous version |
| POST | `/api/rules/{id}/duplicate` | Duplicate a rule with new ID |
| POST | `/api/rules/import` | Import a rule from JSON |

### Admin Endpoints (requires admin role)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/auth/me` | Get current user info |
| POST | `/api/auth/register` | Create a new user |
| GET | `/api/auth/users` | List all users |
| POST | `/api/webhooks` | Create webhook subscription |
| GET | `/api/webhooks` | List webhook subscriptions |
| DELETE | `/api/webhooks/{id}` | Delete webhook subscription |

## Database Schema

```
┌──────────────────────────────┐       ┌──────────────────────────────────┐
│         rules                │       │        rule_versions             │
├──────────────────────────────┤       ├──────────────────────────────────┤
│ id TEXT PK                   │──1:N──│ id INTEGER PK AUTOINCREMENT      │
│ name TEXT NOT NULL            │       │ rule_id TEXT FK → rules.id       │
│ description TEXT              │       │ version INTEGER                  │
│ type TEXT (feature_flag|     │       │ name, description, type, tree    │
│   decision_tree|kill_switch| │       │ default_value, status            │
│   composite)                 │       │ environment TEXT                  │
│ version INTEGER              │       │ active_from DATETIME nullable     │
│ tree TEXT (JSON)             │       │ active_until DATETIME nullable    │
│ default_value TEXT (JSON)    │       │ modified_by TEXT                  │
│ status TEXT (active|draft|   │       │ created_at DATETIME              │
│   disabled)                  │       │ UNIQUE(rule_id, version)         │
│ environment TEXT              │       └──────────────────────────────────┘
│ active_from DATETIME nullable│
│ active_until DATETIME nullable│      ┌──────────────────────────────────┐
│ created_at, updated_at       │       │        eval_history              │
└──────────────────┬───────────┘       ├──────────────────────────────────┤
                   │              1:N  │ id INTEGER PK AUTOINCREMENT      │
                   └───────────────────│ rule_id TEXT FK → rules.id       │
                                       │ context TEXT (JSON)              │
                                       │ result TEXT (JSON)               │
                                       │ created_at DATETIME              │
                                       └──────────────────────────────────┘

┌──────────────────────────────┐       ┌──────────────────────────────────┐
│         users                │       │    webhook_subscriptions         │
├──────────────────────────────┤       ├──────────────────────────────────┤
│ id INTEGER PK AUTOINCREMENT  │       │ id INTEGER PK AUTOINCREMENT      │
│ username TEXT UNIQUE NOT NULL │       │ url TEXT NOT NULL                 │
│ password_hash TEXT NOT NULL   │       │ events TEXT DEFAULT '*'           │
│ role TEXT (admin|editor|     │       │ secret TEXT                       │
│   viewer)                    │       │ active INTEGER DEFAULT 1          │
│ created_at DATETIME          │       │ created_at DATETIME              │
└──────────────────────────────┘       └──────────────────────────────────┘

┌──────────────────────────────┐
│         _meta                │
├──────────────────────────────┤
│ key TEXT PK                  │  ← Tracks system state (e.g., "seeded_at")
│ value TEXT                   │
└──────────────────────────────┘

Indexes:
  idx_eval_history_rule_id     ON eval_history(rule_id, created_at DESC)
  idx_rule_versions_rule_id    ON rule_versions(rule_id, version DESC)
  idx_rules_environment        ON rules(environment)
```

Cascade behavior: deleting a rule automatically deletes all its versions and evaluation history (ON DELETE CASCADE).

## Concurrency Patterns

### 1. Read/Write Connection Pool Separation

```
writeDB ──► max 1 connection  ──► INSERT, UPDATE, DELETE
readDB  ──► max 4 connections ──► SELECT queries

Both use WAL mode (PRAGMA journal_mode=WAL)
Both use PRAGMA foreign_keys = ON
writeDB uses _txlock=immediate (BEGIN IMMEDIATE, not deferred)
```

SQLite allows many concurrent readers but only one writer. Two pools ensure reads never block behind writes.

### 2. Async Eval History with Prune Counter

```
go store.InsertEvalHistory(...)        ← goroutine, API response returns immediately

Inside InsertEvalHistory:
  pruneCounters sync.Map               ← per-rule atomic counters
  counter.Add(1)
  if count % 100 == 0:
    DELETE old entries beyond 1000 limit
```

Avoids running COUNT on every insert. Prune check is amortized over 100 writes.

### 3. Batch Evaluation Worker Pool

```
sem := make(chan struct{}, 10)          ← buffered channel as semaphore

for each rule:
  sem <- struct{}{}                     ← blocks if 10 goroutines running
  go func() {
    defer func() { <-sem }()           ← release slot when done
    evaluate rule
    send result to collector channel
  }()
```

Caps concurrency at 10. Results collected via indexed channel to preserve order.

### 4. Regex Cache with RWMutex

```
sync.RWMutex protects map[string]*regexp.Regexp

Read path (cache hit):   RLock → lookup → RUnlock      ← many readers, no contention
Write path (cache miss): RUnlock → compile → Lock → store → Unlock
```

## Key Design Decisions

| Decision | Why | Tradeoff |
|----------|-----|----------|
| SQLite, not Postgres | Single-file database. Zero ops. Perfect for config management (low write volume, high read volume). Embedded in the binary. | Can't horizontally scale writes. ~1000 writes/sec max. Fine for config changes. |
| Engine has zero dependencies | Can compile to WASM, use as a library, test without I/O. Makes the evaluation logic portable and trustworthy. | API layer has to coordinate between engine + store (more code in handlers). |
| Immutable version snapshots | Every edit creates a new version. Rollback copies old data forward. Nothing is ever deleted (except when the rule itself is deleted). | Storage grows over time. Acceptable for config data (small, low-volume). |
| Schedule check in API layer, not engine | Engine is pure (no time dependency). Schedule enforcement happens at the HTTP boundary. | If you use the engine as a library (SDK/WASM), you must check schedules yourself. |
| modified_by from JWT, not request body | Can't be faked by the client. The server extracts the username from the validated JWT token. | Anonymous edits (no auth) get empty modified_by. |
| Async eval history + webhooks | Neither should delay the API response. History is fire-and-forget. Webhooks are best-effort. | History writes can be lost if the server crashes between response and write. Acceptable for eval history (it's for debugging, not billing). |
| Frontend embedded in Go binary | Single binary deployment. `go build` produces one file that serves everything. | Must rebuild the Go binary when changing frontend. Dev mode flag (`-tags dev`) works around this. |
| Permissive CORS | `Access-Control-Allow-Origin: *` makes development easy. | Must be tightened for production deployment behind a reverse proxy. |
| JWT in localStorage | Simple. Works across tabs. No server-side session storage. | XSS can steal the token. But if attacker has XSS, httpOnly cookies don't meaningfully help (they can make authenticated requests from the compromised page). |
| bcrypt for passwords | Industry standard. Intentionally slow (resistance to brute force). Configurable cost factor. | Slower login (~100ms to verify). Acceptable for a login endpoint. |
| No ORM | Raw SQL gives full control. Schema is simple (6 tables). SQLite's SQL dialect is small. | More boilerplate for scanning rows into structs. |
| Chi router | Lightweight. Supports middleware groups and URL parameters. No magic. | Less feature-rich than Echo or Gin. But we don't need sessions, template rendering, or binding. |

## The Embed Mechanism

How a single Go binary serves both the API and the React dashboard:

```
Build time:
  1. cd web && npm run build              → web/dist/ (index.html, JS, CSS)
  2. go build ./cmd/arbiter/              → arbiter binary

web/embed.go (build tag: !dev):
  //go:embed all:dist                     ← compiler bakes web/dist/ into the binary
  var distFS embed.FS                     ← in-memory filesystem

cmd/arbiter/embed.go:
  webFS := web.DistFS()                   ← returns the embedded filesystem
  router := api.NewRouter(s, authCfg, webFS)

routes.go SPA fallback:
  GET /* →
    if /api/* → route to API handlers
    if file exists in webFS → serve it (JS, CSS, images)
    else → serve index.html (React handles client-side routing)
```

Dev mode: `go run -tags dev ./cmd/arbiter/` uses `embed_dev.go` which reads from the actual filesystem instead of the embedded copy. Allows hot-reloading the frontend without rebuilding Go.

## Deployment

```dockerfile
# Multi-stage Dockerfile
FROM node:20-alpine AS frontend
  WORKDIR /app/web
  COPY web/ .
  RUN npm ci && npm run build

FROM golang:1.22-alpine AS backend
  COPY --from=frontend /app/web/dist ./web/dist
  RUN go build -o /arbiter ./cmd/arbiter/

FROM alpine:3.19
  COPY --from=backend /arbiter /arbiter
  EXPOSE 8080
  CMD ["/arbiter"]
```

Result: ~15MB container image. Single binary. No runtime dependencies.

Environment variables:
- `ARBITER_DB_PATH` - SQLite file path (default: `arbiter.db`)
- `PORT` - HTTP port (default: `8080`)
- `ARBITER_JWT_SECRET` - JWT signing key (auto-generated if not set)
