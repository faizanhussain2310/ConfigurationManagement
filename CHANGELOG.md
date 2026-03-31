# Changelog

All notable changes to Arbiter are documented in this file.

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
