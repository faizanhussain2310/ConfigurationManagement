# Technology Tradeoffs

Every technology choice in Arbiter has a "why this" and "why not the alternatives." These are fair game for interview questions.

## Why Go?

| | Go | Node.js | Python | Java | Rust |
|-|----|---------|---------|----|------|
| Single binary | Yes | No (needs runtime) | No (needs runtime) | Sort of (fat JAR, ~50MB+) | Yes |
| Compile speed | ~2s | N/A (interpreted) | N/A | ~30s+ | ~60s+ |
| Runtime speed | Fast (~100ns eval) | Medium (~1us eval) | Slow (~10us eval) | Fast (~100ns) | Fastest (~50ns) |
| Ecosystem for CLI/server tools | Excellent | Good | Good for scripts | Heavy | Growing |
| Learning curve | Low-medium | Low | Lowest | High (Spring, etc.) | Very high |

**Why Go wins:** Single binary output means `//go:embed` works and distribution is trivial. Fast compilation means tight dev loops. Standard library covers HTTP server, JSON, hashing, testing with zero external dependencies. One command: `go build -o arbiter ./cmd/arbiter/`.

**Why not Rust:** Rust would be faster, but the learning curve is 3-6 months. For a project with a tight timeline, Go is the pragmatic choice. Rust is the right call for v2's WASM compiler if decision trees ever need sub-microsecond local evaluation.

**Why not Node.js:** No single binary. You'd need to bundle with `pkg` or similar, which is brittle. TypeScript adds compile step complexity without Go's built-in tooling benefits (testing, benchmarks, profiling all built in).

## Why SQLite?

| | SQLite | PostgreSQL | MySQL | Redis |
|-|--------|-----------|-------|-------|
| Setup | Zero (file on disk) | Requires server process | Requires server process | Requires server process |
| Docker complexity | One container | Two containers (docker-compose) | Two containers | Two containers |
| Concurrent reads | Excellent (WAL mode) | Excellent | Good | Excellent |
| Concurrent writes | One writer at a time | Excellent | Good | Excellent |
| Data model fit | Perfect (10-100 rules) | Overkill | Overkill | No relational queries |
| Backup | Copy the file | pg_dump | mysqldump | RDB/AOF |

**Why SQLite wins:** The "one binary" constraint makes SQLite the only real choice. PostgreSQL requires a separate server process, which means docker-compose instead of `docker run`, which violates the "one command to start" goal. For 10-100 rules with modest evaluation traffic, SQLite's single-writer limitation is irrelevant.

**Why not PostgreSQL:** Postgres is the better database. Period. But it requires a separate server, doubling deployment complexity. If Arbiter ever needed 10K+ writes/sec, Postgres would be the right migration target. The Store struct's methods make this swap possible without touching the engine or API.

**Why `modernc.org/sqlite` instead of `mattn/go-sqlite3`:** modernc is pure Go (no CGo), so cross-compilation works out of the box and Docker builds don't need a C compiler. The 2-3x speed penalty doesn't matter at this traffic level.

### SQLite Configuration

Arbiter runs these PRAGMAs on every connection:

```sql
PRAGMA journal_mode = WAL;          -- concurrent readers + one writer
PRAGMA busy_timeout = 5000;         -- wait 5s on lock contention instead of failing
PRAGMA synchronous = NORMAL;        -- safe with WAL, 10x faster than FULL
PRAGMA foreign_keys = ON;           -- SQLite defaults to OFF, cascade deletes need this
```

Two `*sql.DB` pools:
- **writeDB**: `MaxOpenConns(1)` serializes all writes through one connection
- **readDB**: `MaxOpenConns(4)` allows concurrent reads for list/get/export

DSN includes `_txlock=immediate` so Go's `database/sql` issues `BEGIN IMMEDIATE` instead of deferred `BEGIN`. This prevents write starvation under concurrent load.

## Why React + Vite?

| | React + Vite | Vue 3 | Svelte | Plain HTML/JS |
|-|-------------|-------|--------|---------------|
| Ecosystem size | Largest | Large | Growing | N/A |
| Bundle size (typical) | ~150KB | ~100KB | ~50KB | ~0KB |
| CodeMirror integration | Well-documented | Good | Limited docs | Manual |
| d3/react-d3-tree compat | Native | Needs wrapper | Needs wrapper | Manual |
| Learning resources | Most | Many | Fewer | N/A |

**Why React wins:** The dashboard needs CodeMirror 6 (JSON editor) and react-d3-tree (visual tree view). Both have first-class React support. React's ecosystem is the largest, which means more documentation when you get stuck. Standard choice that no one will question.

**Why not Svelte:** Svelte produces smaller bundles (~50KB vs ~150KB), which matters since we embed the frontend in the Go binary. But the CodeMirror 6 and d3 integration documentation for Svelte is thin. The 100KB difference is negligible in a ~30MB Docker image.

**Why not plain HTML/JS:** You could build the entire dashboard with vanilla JS. Smallest bundle, shows fundamentals. The tradeoff: building a three-panel interactive dashboard with a code editor, tree visualization, and live evaluation panel in vanilla JS is ~3x more code and much harder to maintain. React's component model earns its 150KB here.

## Why Chi?

| | Chi | Gin | Echo | net/http |
|-|-----|-----|------|----------|
| API compatibility | 100% net/http | Custom context | Custom context | N/A (is stdlib) |
| Route params | Yes (clean) | Yes | Yes | Manual parsing |
| Middleware | net/http compatible | Gin-only | Echo-only | Manual |
| Performance | Fast | Slightly faster | Fast | Fast |

**Why Chi wins:** Full compatibility with Go's `net/http` standard library. Any middleware written for net/http works with Chi. Gin uses its own `gin.Context` type, locking you into Gin's ecosystem. Using Chi (which is just net/http underneath) shows you understand the standard library, not just a framework.

## Why CodeMirror 6?

| | CodeMirror 6 | Monaco | Ace | textarea |
|-|-------------|-------|-----|----------|
| Bundle size | ~100KB | ~4MB | ~300KB | 0KB |
| JSON highlighting | Yes | Yes | Yes | No |
| Line numbers | Yes | Yes | Yes | No |
| Error markers | Yes (extensions) | Yes (built-in) | Yes | No |
| Embedded binary impact | Negligible | +4MB to Go binary | Moderate | None |

**Why CodeMirror 6 wins:** 100KB vs 4MB. Since the frontend gets embedded in the Go binary via `//go:embed`, every MB of frontend code inflates the binary. Monaco is VS Code's editor, overkill for editing JSON. CodeMirror 6 does exactly what we need at 1/40th the size.

## Why FNV-1a for Percentage Rollouts?

| | FNV-1a | MurmurHash3 | SHA-256 | CRC32 |
|-|--------|------------|---------|-------|
| Speed | Very fast (ns) | Fast (ns) | Slow (us) | Very fast (ns) |
| Distribution quality | Good | Excellent | Perfect | Poor |
| Go stdlib | Yes (`hash/fnv`) | No (external) | Yes (`crypto/sha256`) | Yes |
| Use case fit | Perfect | Good | Overkill | Bad (uneven buckets) |

**Why FNV-1a wins:** It's in Go's standard library, fast, and has good enough distribution for percentage rollouts. MurmurHash3 has better distribution but requires an external dependency. SHA-256 is cryptographic, meant for security, not speed. CRC32 has poor distribution which would give uneven rollout percentages.

The hash formula: `FNV-1a(rule_id + ":" + field_value) % 100 < pct_value`. Including `rule_id` in the key decorrelates buckets across rules, so a user in the 25% group for rule A isn't necessarily in the 25% group for rule B.
