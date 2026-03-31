# Changelog

All notable changes to Arbiter are documented in this file.

## [2.0.0] - 2026-03-31

### Added
- **Auth/RBAC**: JWT authentication with bcrypt password hashing, 3-tier role hierarchy (admin > editor > viewer)
- Login page with JWT token management and auto-refresh
- AuthOptional/AuthRequired/RequireRole middleware stack
- Default admin user seeded on first run (admin/admin)
- **Webhook Notifications**: HMAC-SHA256 signed webhook delivery with async dispatch
- Webhook subscription management UI (admin only) with create/delete
- Supports event filtering: rule.created, rule.updated, rule.deleted, or wildcard (*)
- **Rule Composition**: Composite rule type combining multiple child rules
- 4 composition strategies: all_true, any_true, first_match, merge_results
- Circular reference detection via DFS topological sort
- **Visual Tree Editor**: Interactive drag-and-drop tree editor using ReactFlow (@xyflow/react)
- Custom node types: Condition (field/op/value), Value (leaf), Combinator (AND/OR)
- Inline editing of node values with live preview
- **Go SDK**: Client library at `sdk/arbiter/` with full API coverage
- Supports all 14 REST endpoints with typed responses
- JWT auth token support via `WithAuth()` method
- 17 new tests covering auth (JWT, passwords, roles), composition (all 4 strategies, cycle detection), and validation

### Changed
- Rule type now includes 'composite' option in both backend and frontend
- Routes restructured with auth-protected groups (read=public, write=editor+, admin=admin)
- TopBar shows logged-in user with role badge and logout button
- Replaced react-d3-tree with ReactFlow for interactive visual editing

## [1.0.0] - 2026-03-31

### Added
- Decision tree evaluation engine with 10 operators (eq, neq, gt, gte, lt, lte, in, nin, regex, pct)
- AND/OR combinator logic in conditions with short-circuit evaluation
- Decision path tracing on every evaluation (shows exactly why a result was returned)
- Deterministic percentage rollouts via FNV-1a hashing with per-rule decorrelation
- Rule versioning with immutable snapshots and non-destructive rollback
- Version diff view in dashboard (side-by-side comparison)
- Import/Export rules as JSON with conflict detection and force-overwrite
- Rule duplication with automatic naming
- Evaluation history with pagination and per-rule retention (1,000 entries max)
- React dashboard embedded in Go binary via `//go:embed`
- CodeMirror 6 JSON editor with syntax highlighting and error markers
- Visual tree rendering with react-d3-tree
- Test panel for live rule evaluation with context
- 4 seeded example rules on first run (feature flag, decision tree, kill switch, percentage rollout)
- SQLite storage with WAL mode, two connection pools (read/write separation)
- 14 REST API endpoints with pagination, validation, and error handling
- Multi-stage Dockerfile for single-container deployment
- GitHub Actions CI pipeline
- 18 engine unit tests covering evaluation, operators, validation, and edge cases
