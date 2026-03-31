# Arbiter — v2 Backlog

Deferred items from /plan-ceo-review on 2026-03-31. Each item was considered for v1 and explicitly deferred.

## P2 — High Value, Build After v1 Is Solid

### WASM Compilation (Approach C)
- **What:** Compile decision trees to WebAssembly modules. Server becomes a rule compiler + distribution system. Client SDKs embed WASM runtime for local evaluation.
- **Why:** 5000x latency reduction (1.2ms HTTP vs 0.23us WASM). Language-agnostic SDKs. The interview mic-drop demo.
- **Effort:** L (human: ~1-2 months / CC: ~3-4 hours)
- **Depends on:** Solid v1 engine with all operators working correctly. Approach B must be 100% shipped first.
- **Start here:** `pkg/compiler/wat.go` — generate WAT (WebAssembly Text Format) from the Node tree. Start with `eq` and `gte` operators only. Use `wazero` (pure Go WASM runtime) for the Go SDK.

### Rule Composition / Composite Rules
- **What:** A "meta-rule" type that references and combines multiple child rules. Combine strategies: `all_true`, `any_true`, `first_match`, `merge_results`.
- **Why:** Enables the PhonePe-style multi-node pattern in clean-room form. Lets users build complex evaluation pipelines from simple building blocks.
- **Effort:** M (human: ~1 week / CC: ~45 min)
- **Depends on:** v1 engine with AND/OR combinators working. Need circular reference detection (topological sort), evaluation ordering, and error propagation strategy.
- **Start here:** New `composite` rule type. `POST /api/rules` with `type: "composite"` and `rules: ["rule_a", "rule_b"]`. Evaluate fetches child rules, evaluates each, combines results.

## P3 — Nice to Have, Build When Bored

### OpenFeature Compatibility
- **What:** Make the Arbiter API compatible with the OpenFeature specification (CNCF incubating standard). Implement an OpenFeature provider for Go.
- **Why:** OpenFeature is becoming the standard for feature flag APIs. Compatibility means Arbiter can be a drop-in backend for any OpenFeature SDK.
- **Effort:** M (human: ~1 week / CC: ~30 min)
- **Depends on:** v1 API finalized. OpenFeature spec is stable enough to target.
- **Start here:** Read the OpenFeature Go SDK spec. Implement `Provider` interface that wraps Arbiter's evaluate endpoint.

### Authentication / RBAC
- **What:** Add authentication to the dashboard and API. Role-based access control: admin (full access), editor (create/edit rules), viewer (read-only + evaluate).
- **Why:** Required for any real deployment. Not needed for portfolio demo.
- **Effort:** M (human: ~1 week / CC: ~30 min)
- **Depends on:** Nothing. Can be added independently.
- **Start here:** JWT-based auth middleware in `pkg/api/middleware.go`. Users table in SQLite. Login page in React.

### Webhook Notifications
- **What:** Fire webhooks on rule create/update/delete/rollback events. Configurable per rule or globally.
- **Why:** Enables integration with Slack, PagerDuty, CI/CD pipelines.
- **Effort:** S (human: ~2-3 days / CC: ~15 min)
- **Depends on:** v1 API. Add webhook_subscriptions table, async HTTP POST on events.
- **Start here:** `pkg/store/webhooks.go` + `pkg/api/handlers_webhook.go`.

### Visual Tree Editor
- **What:** Upgrade the read-only tree visualization to an interactive editor. Drag-and-drop to rearrange nodes. Click to edit conditions. No JSON required.
- **Why:** Makes rule creation accessible to non-engineers. Product managers can build rules visually.
- **Effort:** M (human: ~2 weeks / CC: ~1 hour)
- **Depends on:** Visual tree view (v1) working. Switch from react-d3-tree to react-flow for interactive editing.
- **Start here:** `web/src/components/TreeEditor.tsx` using react-flow. Bi-directional sync with JSON editor.

### Pre-Computed Version Diffs
- **What:** Compute and store JSON Patch diffs when a version is created, instead of computing on-the-fly when viewing history.
- **Why:** Faster history tab rendering for rules with many versions (20+).
- **Effort:** S (human: ~1 day / CC: ~10 min)
- **Depends on:** Version history (v1) working. Add `diff TEXT` column to `rule_versions` table.
- **Start here:** In `store.CreateVersion()`, compute diff from previous version and store alongside the snapshot.
