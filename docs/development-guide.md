# Development Guide

How to set up, run, test, and build Arbiter from source.

## Prerequisites

- **Go 1.23+** (`go version`)
- **Node.js 20+** (`node --version`)
- **npm 10+** (`npm --version`)

## Project Setup

```bash
git clone https://github.com/faizanhussain2310/ConfigurationManagement.git
cd ConfigurationManagement
```

Install Go dependencies:
```bash
go mod download
```

Install frontend dependencies:
```bash
cd web && npm ci && cd ..
```

## Running in Development

You have two options depending on what you're working on.

### Option 1: Backend only (API, no dashboard)

```bash
go run -tags dev ./cmd/arbiter/
```

The `-tags dev` flag activates `embed_dev.go`, which skips embedding the frontend. The API works at `http://localhost:8080/api/`, but the dashboard returns nothing (no `web/dist/` directory).

Use this when you're only working on the Go backend and testing with `curl`.

### Option 2: Full stack (API + dashboard with hot reload)

Terminal 1 (Go backend):
```bash
go run -tags dev ./cmd/arbiter/
```

Terminal 2 (Vite dev server):
```bash
cd web && npm run dev
```

Vite runs on `http://localhost:5173` and proxies `/api` requests to `http://localhost:8080`. Edit React components and see changes instantly via HMR (hot module replacement).

**Why two terminals?** The Go server serves the API. Vite serves the frontend with hot reload. In production, the frontend is compiled and embedded into the Go binary, so you only need one process. In development, you want Vite's instant feedback loop.

### Option 3: Production-like (embedded dashboard)

```bash
cd web && npm run build && cd ..
go run ./cmd/arbiter/
```

This builds the frontend into `web/dist/`, then runs Go without the `dev` tag, so `embed.go` picks up the built files. The dashboard is at `http://localhost:8080/`.

## Environment Variables

| Variable | Default | What it does |
|----------|---------|-------------|
| `ARBITER_DB_PATH` | `arbiter.db` | Path to the SQLite database file |
| `PORT` | `8080` | HTTP server port |

Example:
```bash
ARBITER_DB_PATH=/tmp/test.db PORT=3000 go run -tags dev ./cmd/arbiter/
```

## Running Tests

### All tests

```bash
go test ./... -v
```

This runs 40 tests total: 18 engine tests + 22 store tests.

### Engine tests only (fast, no database)

```bash
go test ./pkg/engine/ -v
```

These test the evaluation logic, operators, field extraction, and validation. No SQLite involved. Run in ~0.2 seconds.

### Store tests only (creates temp SQLite databases)

```bash
go test ./pkg/store/ -v
```

Each test creates a temporary SQLite database in `t.TempDir()`, runs operations, and cleans up. Tests CRUD, versioning, rollback, duplicate, import, eval history, pagination, and cascade delete. Run in ~0.8 seconds.

### Run a specific test

```bash
go test ./pkg/engine/ -run TestEvaluateNestedTree -v
go test ./pkg/store/ -run TestRollbackToVersion -v
```

## Building for Production

### From source

```bash
# Step 1: Build frontend (produces web/dist/)
cd web && npm ci && npm run build && cd ..

# Step 2: Build Go binary (embeds web/dist/ into the binary)
go build -o arbiter ./cmd/arbiter/

# Step 3: Run
./arbiter
```

**Build order matters.** Step 1 must complete before Step 2. The Go compiler's `//go:embed all:dist` directive in `web/embed.go` reads files from `web/dist/` at compile time. If `web/dist/` doesn't exist or is empty, the build fails (except for the `.gitkeep` file).

The output is a single binary (`arbiter`, ~30MB) that contains the API server, the React dashboard, and the SQLite driver. No runtime dependencies.

### With Docker

```bash
docker build -t arbiter .
docker run -p 8080:8080 -v arbiter-data:/data arbiter
```

The Dockerfile is a multi-stage build:

```
Stage 1: node:20-alpine
  └─ npm ci && npm run build     → produces web/dist/

Stage 2: golang:1.23-alpine
  └─ COPY web/dist from stage 1
  └─ go build                    → produces /app/arbiter binary

Stage 3: alpine:3.19
  └─ COPY arbiter from stage 2   → minimal runtime image (~20MB)
  └─ EXPOSE 8080
  └─ VOLUME /data                → SQLite database persisted here
```

The `-v arbiter-data:/data` flag creates a Docker volume so the SQLite database survives container restarts. Without it, your data disappears when the container stops.

## How the Embed Mechanism Works

This is the trickiest part of the build. Here's why it's set up the way it is.

### The problem

Go's `//go:embed` directive can only reference files in the same directory or subdirectories of the package. `cmd/arbiter/main.go` can't write `//go:embed ../../web/dist` because embed can't go up directories.

### The solution

The embed directive lives in `web/embed.go`, which is in the `web/` package (same directory as `dist/`):

```go
// web/embed.go (build tag: !dev)
//go:embed all:dist
var distFS embed.FS

func DistFS() (fs.FS, error) {
    return fs.Sub(distFS, "dist")
}
```

`cmd/arbiter/embed.go` imports this package:

```go
// cmd/arbiter/embed.go (build tag: !dev)
import "github.com/faizanhussain/arbiter/web"

func getWebFS() fs.FS {
    webFS, _ := web.DistFS()
    return webFS
}
```

### Build tags

Two pairs of files with build tags control the behavior:

| File | Build tag | When active | What it does |
|------|-----------|-------------|-------------|
| `web/embed.go` | `!dev` | Production (default) | Embeds `web/dist/` into the binary |
| `web/embed_dev.go` | `dev` | `go run -tags dev` | Returns `os.DirFS("web/dist")` or nil |
| `cmd/arbiter/embed.go` | `!dev` | Production (default) | Calls `web.DistFS()` |
| `cmd/arbiter/embed_dev.go` | `dev` | `go run -tags dev` | Calls `web.DistFS()`, logs warning if nil |

The `!dev` tag means "included by default." You don't pass any special flag for production builds. The `dev` tag is only active when you explicitly pass `-tags dev`.

### The .gitkeep file

`web/dist/.gitkeep` is committed to the repo so that `web/dist/` exists on fresh clones. Without it, `//go:embed all:dist` fails because the directory doesn't exist. The `.gitignore` has:

```
web/dist/
!web/dist/.gitkeep
```

This ignores all build output in `web/dist/` except the `.gitkeep` marker.

## SPA Fallback Routing

The React app uses client-side routing (React Router). URLs like `/rules/pricing_tier` exist only in the browser. If a user refreshes the page, the browser sends `GET /rules/pricing_tier` to the Go server.

Without special handling, that's a 404. The SPA fallback in `routes.go` fixes this:

1. If the path starts with `/api/`, handle it normally (API routes)
2. Try to open the path as a file in `web/dist/` (JS, CSS, images, fonts)
3. If the file exists, serve it
4. If not, serve `index.html` and let React Router handle the path

This is why you can bookmark any dashboard URL and it works after refresh.

## Database Files

When Arbiter runs, it creates these files:

| File | What it is |
|------|-----------|
| `arbiter.db` | The SQLite database (main file) |
| `arbiter.db-wal` | Write-Ahead Log (WAL mode journal) |
| `arbiter.db-shm` | Shared memory file (WAL index) |

All three are gitignored. The `-wal` and `-shm` files are managed by SQLite automatically. Don't delete them while the server is running.

To start fresh: stop the server, delete all three files, restart. The database will be recreated and seeded with example rules.

## CI Pipeline

`.github/workflows/ci.yml` runs on every push and PR to `main`:

1. `go test ./...` — runs all 40 tests
2. `cd web && npm ci && npm run build` — verifies the frontend builds
3. `go build -o arbiter ./cmd/arbiter/` — verifies the full binary builds (with embedded frontend)

All three must pass for the CI to be green.

## Common Tasks

### Add a new operator

1. Add the case in `pkg/engine/operators.go` `EvalOperator()` switch
2. Write the evaluation function (e.g., `evalMyOp()`)
3. Add validation in `pkg/engine/validate.go` if the operator has type constraints
4. Add tests in `pkg/engine/evaluate_test.go`
5. Update `docs/operators.md`

### Add a new API endpoint

1. Write the handler in `pkg/api/handlers.go` (or a new `handlers_*.go` file)
2. Register the route in `pkg/api/routes.go`
3. If it's a static path (like `/api/rules/import`), register it BEFORE `/{id}` routes
4. Add the endpoint to the TypeScript client in `web/src/api/client.ts`
5. Update `docs/api-reference.md`

### Add a new seed rule

1. Add an entry to the `seedRules()` slice in `pkg/store/seeds.go`
2. Add a reference JSON file in `examples/`
3. Delete your local `arbiter.db` and restart to see the new seed

Note: seeds only run on a completely empty database that hasn't been seeded before. The `_meta.seeded_at` key prevents re-seeding.

### Reset the database

```bash
rm arbiter.db arbiter.db-wal arbiter.db-shm
go run -tags dev ./cmd/arbiter/
```

Fresh database, reseeded with the 4 example rules.
