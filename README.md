# Arbiter

**Feature flags and rule engines are the same thing. This is the tool that proves it.**

Arbiter is a unified rule engine and feature flag system. It treats every configuration decision as a decision tree: context in, value out. A feature flag returns `true/false`. A pricing calculator returns `"enterprise"`. A rollout gate returns whether this specific user is in the 25% bucket. Same engine, same API, same dashboard.

Ships as a **single binary** with an embedded React dashboard. No Postgres, no Redis, no infrastructure. `docker run` and you're live.

## Quick Start

```bash
# From source
go build -o arbiter ./cmd/arbiter/
./arbiter

# With Docker
docker build -t arbiter .
docker run -p 8080:8080 -v arbiter-data:/data arbiter
```

Open `http://localhost:8080` for the dashboard. The API is at `http://localhost:8080/api/`.

## How It Works

Every rule in Arbiter is a decision tree. You send a context (JSON object with user attributes, org data, whatever you want), and the engine walks the tree, evaluating conditions at each node until it reaches a leaf value.

```
Context: {"user": {"age": 25, "country": "US"}}
                    │
            ┌───────▼────────┐
            │ age >= 18?     │
            └───┬────────┬───┘
              true     false
            ┌───▼───┐ ┌──▼──┐
            │ "adult"│ │"minor"│
            └───────┘ └──────┘

Result: "adult"
Path: ["user.age gte 18 → true", "→ value: adult"]
```

The path trace shows exactly why the engine returned what it did. No more guessing why a user got a specific feature flag value.

## Arbiter vs. the Field

```
                          Arbiter  Unleash  Flagsmith  Flipt   Zen
Decision trees              ✅       ❌        ❌       ❌      ✅*
Feature flags               ✅       ✅        ✅       ✅      ❌
Unified abstraction         ✅       ❌        ❌       ❌      ❌
Single binary               ✅       ❌        ❌       ✅      ❌
Embedded dashboard          ✅       ✅        ✅       ✅      ❌
Decision path tracing       ✅       ❌        ❌       ❌      ❌
AND/OR boolean logic        ✅       ❌        Partial  Partial ✅
Version history + diff      ✅       ❌        ❌       ✅      ❌
SQLite (zero deps)          ✅       ❌(PG)    ❌(PG)   ✅      N/A

* Zen uses decision tables (rows/columns), not trees (if/then/else)
```

## API

```
POST   /api/rules                        Create a rule
GET    /api/rules?limit=50&offset=0      List all rules (paginated)
GET    /api/rules/:id                    Get a rule
PUT    /api/rules/:id                    Update a rule (creates new version)
DELETE /api/rules/:id                    Delete a rule (cascade)
POST   /api/rules/:id/evaluate           Evaluate with context
POST   /api/evaluate                     Batch evaluate multiple rules
GET    /api/rules/:id/history            Evaluation history (paginated)
GET    /api/rules/:id/versions           Version history
POST   /api/rules/:id/rollback/:version  Rollback to a version (creates v+1)
POST   /api/rules/:id/duplicate          Duplicate a rule
GET    /api/rules/:id/export             Export as JSON
POST   /api/rules/import                 Import from JSON
GET    /api/health                       Health check
```

### Evaluate a rule

```bash
curl -X POST http://localhost:8080/api/rules/pricing_tier/evaluate \
  -H "Content-Type: application/json" \
  -d '{"context": {"org": {"employees": 50}}}'
```

Response:
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

## Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `eq`     | Equals | `{"field": "user.country", "op": "eq", "value": "US"}` |
| `neq`    | Not equals | |
| `gt`, `gte` | Greater than (or equal) | `{"field": "user.age", "op": "gte", "value": 18}` |
| `lt`, `lte` | Less than (or equal) | |
| `in`     | Value in list | `{"field": "user.country", "op": "in", "value": ["US", "CA"]}` |
| `nin`    | Value not in list | |
| `regex`  | Regex match (RE2) | `{"field": "user.email", "op": "regex", "value": "@company\\.com$"}` |
| `pct`    | Percentage rollout | `{"field": "user.id", "op": "pct", "value": 25}` |

The `pct` operator uses FNV-1a hashing: `hash(rule_id + ":" + field_value) % 100 < pct_value`. Deterministic per user per rule, with rule_id in the hash for decorrelation across rules.

## AND/OR Combinators

Conditions support boolean logic with `and`/`or` combinators:

```json
{
  "condition": {
    "combinator": "or",
    "conditions": [
      {"field": "user.age", "op": "gte", "value": 18},
      {"field": "user.country", "op": "eq", "value": "US"}
    ]
  },
  "then": {"value": true},
  "else": {"value": false}
}
```

Combinators short-circuit: AND stops on first false, OR stops on first true. Nesting up to 3 levels deep, max 10 conditions per group.

## Technology Choices

| Choice | Why |
|--------|-----|
| **Go** | Single binary via `//go:embed`. Fast compilation (~2s). Standard library covers HTTP, JSON, hashing, testing. |
| **SQLite** | Zero infrastructure. WAL mode for concurrent reads. File-on-disk backup. `modernc.org/sqlite` (pure Go, no CGo). |
| **Chi** | 100% `net/http` compatible. No framework lock-in. |
| **React + Vite** | First-class CodeMirror 6 and react-d3-tree support. Standard choice for interactive dashboards. |
| **CodeMirror 6** | 100KB vs Monaco's 4MB. Matters when embedded in a Go binary. |
| **FNV-1a** | In Go stdlib (`hash/fnv`). Fast, good distribution for percentage rollouts. |

## Development

```bash
# Backend (with hot reload via air or manual restart)
go run -tags dev ./cmd/arbiter/

# Frontend (Vite dev server with HMR, proxies /api to :8080)
cd web && npm run dev

# Run tests
go test ./... -v

# Build production binary
cd web && npm ci && npm run build
cd .. && go build -o arbiter ./cmd/arbiter/
```

## Architecture

```
arbiter/
├── cmd/arbiter/          # Entry point, embed setup
├── pkg/
│   ├── engine/           # Core: types, evaluate, operators, validate
│   ├── store/            # SQLite: migrations, seeds, CRUD
│   └── api/              # HTTP: routes, handlers, middleware
├── web/src/              # React dashboard
│   ├── components/       # RuleList, Editor, TestPanel, TreeView, HistoryView
│   ├── hooks/            # useRules
│   └── api/              # client.ts
├── examples/             # Reference rule JSON files
└── Dockerfile            # Multi-stage build
```

## License

MIT
