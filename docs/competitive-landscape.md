# Competitive Landscape

## What Already Exists in Open Source

| Tool | Category | Language | What It Does | What It Doesn't Do |
|------|----------|----------|-------------|-------------------|
| **Unleash** | Feature flags | Node.js | Gradual rollouts, activation strategies, SDK support (20+ languages). 13k+ GitHub stars. Most popular OSS flag tool. | No decision trees. No rule engine. Flags are boolean only. Can't return `"5% cashback"` or `"variant_a"`. |
| **Flagsmith** | Feature flags + remote config | Python/Django | Feature flags + server-driven configuration. Identity management, user traits. | No tree-based evaluation. Config is key-value, not conditional. Can't express "if region=X and age>18, return Y". |
| **Flipt** | Feature flags | Go | 100% OSS, Git-native flag management, zero paid tiers. | Basic segments and constraints only. No nested conditions, no decision trees. |
| **GrowthBook** | Feature flags + A/B testing | TypeScript | Feature flags with built-in A/B testing and statistical analysis. | Focused on experimentation, not rule evaluation. No arbitrary value returns. |
| **Drools** | Business rules engine | Java | Full RETE-based rule engine. Complex event processing, business rules. | Massive (enterprise Java). Requires JVM. Overkill for feature flags. Not lightweight. |
| **Zen Engine** (gorules) | Business rules engine | Rust + WASM | JSON-based decision tables and expression evaluation. MIT licensed. Multi-language support. | Tables, not trees. Different paradigm (rows and columns vs if/then/else). No management dashboard. |
| **OpenFeature** | Specification | N/A | CNCF standard API spec for feature flags. Vendor-agnostic SDK interface. | Not a backend. Needs a provider (Unleash, Flagsmith, etc.) to store and evaluate flags. |

## The Gap

Nobody has built a unified system that:

1. Treats feature flags and decision trees as the **same primitive** (context in, value out)
2. Ships as a **single binary** with an embedded management dashboard
3. Includes **AND/OR boolean logic**, percentage rollouts, regex matching, and arbitrary value returns
4. Provides **decision path tracing** (shows exactly why a rule returned what it did)
5. Includes **version history with diff view** and rollback
6. Is written in **Go** with zero external service dependencies

Feature flag tools (Unleash, Flagsmith, Flipt) give you booleans and simple segments. Business rule engines (Drools, Zen) give you complex logic but no dashboard and no feature flag workflow. Nobody combines both.

## Arbiter's Position

**"Feature flags and rule engines are the same thing. This is the tool that proves it."**

The core insight: a feature flag is a degenerate case of a decision tree evaluation. `context in, value out`. A flag returns `true/false`. A cashback calculator returns `5.0`. A content variant selector returns `"variant_a"`. Same engine, same API, same dashboard, same evaluation.

### What makes Arbiter different:

1. **Unified abstraction.** One system for flags AND business rules. Not two tools stitched together.
2. **Single binary.** `docker run` and you have a server + dashboard. No Postgres, no Redis, no infrastructure.
3. **Decision path visibility.** Every evaluation returns the exact path through the tree, so you can debug why a user got a specific result. No other OSS tool does this at the tree level.
4. **AND/OR logic in conditions.** Most flag tools only support simple segment matching. Arbiter supports nested boolean logic within conditions.
5. **Built for learning.** The codebase is the documentation. Every technical choice has a "why" and "why not the alternatives."

## Comparison Matrix

```
                          Arbiter  Unleash  Flagsmith  Flipt   Zen
Decision trees              Y        N        N         N       Y*
Feature flags               Y        Y        Y         Y       N
Unified abstraction         Y        N        N         N       N
Single binary               Y        N        N         Y       N
Embedded dashboard          Y        Y        Y         Y       N
Decision path tracing       Y        N        N         N       N
AND/OR boolean logic        Y        N        Partial   Partial Y
Version history + diff      Y        N        N         Y       N
SQLite (zero deps)          Y        N(PG)    N(PG)     Y       N/A

* Zen uses decision tables (rows/columns), not trees (if/then/else)
```

## Where Arbiter is Not the Right Tool

Arbiter is honest about its scope:

- **High-write throughput (10K+ evaluations/sec):** SQLite's single-writer model becomes a bottleneck. You'd want PostgreSQL or a distributed store.
- **Client-side evaluation:** Arbiter evaluates on the server. For latency-sensitive mobile apps, you want SDKs that evaluate locally (Unleash has this, Arbiter doesn't yet).
- **Enterprise RBAC:** No authentication or role-based access in v1. It's an open dashboard.
- **A/B test statistics:** GrowthBook is the right tool if you need statistical significance calculations and experiment analysis.
- **Complex event processing:** Drools handles temporal rules ("if X happens within 5 minutes of Y"). Arbiter evaluates point-in-time context only.

## Future Direction

The v2 vision: compile decision trees to WebAssembly modules for sub-microsecond local evaluation. The server becomes a rule compiler and distribution system. Client SDKs embed a WASM runtime. This closes the client-side evaluation gap while keeping the unified tree model.
