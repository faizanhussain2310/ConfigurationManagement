# Frontend Architecture

High-level overview of the React dashboard, its component structure, state management, and how it communicates with the backend.

## Tech Stack

| Layer | Choice | Why |
|-------|--------|-----|
| Framework | React 18 | Industry standard, component model fits the dashboard layout |
| Build tool | Vite | Fast dev server, production build in ~2.5s |
| Language | TypeScript | Type safety for API responses and component props |
| JSON editor | CodeMirror 6 | Syntax highlighting, folding, error markers. Lighter than Monaco (~780KB vs ~2MB) |
| Visual editor | ReactFlow (@xyflow/react) | Best React library for interactive node graphs |
| CSS | Single global CSS file | Simple. No CSS-in-JS overhead. Dark theme with CSS variables |
| State | React hooks + Context | No Redux/Zustand. App state is small enough for prop drilling |
| Routing | None (tabs) | Single-page with tab switching. No URL-based routing needed |

## Component Tree

```
<AuthProvider>                          ← React Context: username, role, login(), logout()
  <App>                                 ← Auth gate: loading → login → dashboard
    ├── <LoginPage />                   ← Shown when not authenticated
    └── <Dashboard>                     ← Shown when authenticated
         ├── <TopBar>                   ← [Arbiter] [env filter ▼] ... [Users] [Webhooks] [Import] [user badge] [Logout]
         ├── <RuleList>                 ← Sidebar: [+ New Rule] + list of rules with badges
         │    └── Create form           ← Collapsible: ID, name, type, environment
         └── Main area (one of):
              ├── <Editor>              ← Tab: name, type, status, env, schedule, tree JSON, default value
              ├── <TestPanel>           ← Tab: context input → evaluate → result + path
              ├── <TreeEditor>          ← Tab: visual drag-and-drop node graph
              ├── <HistoryView>         ← Tab: versions (with audit) + evaluations
              │    └── <DiffView>       ← Expandable: side-by-side version comparison
              ├── <WebhookManager>      ← Admin view: webhook CRUD table
              └── <UserManager>         ← Admin view: user CRUD table
```

## State Flow

There is no global state store. State lives in three places:

### 1. Auth Context (`hooks/useAuth.tsx`)

```
AuthProvider (wraps entire app)
  └── useAuth() hook → { username, role, login(), logout(), loading }
```

On mount: checks localStorage for an existing JWT token. If found, calls `GET /api/auth/me` to validate. If expired, clears token and shows login page.

Token management: `setToken()` in `client.ts` stores the JWT in both a module-level variable (for immediate use) and localStorage (for persistence across page refreshes).

### 2. Rule List State (`hooks/useRules.ts`)

```
useRules(environment) → { rules, loading, error, refresh() }
```

Accepts an optional environment filter. Calls `GET /api/rules?environment=X` on mount and when environment changes. Returns the full list. Components call `refresh()` after mutations (create/update/delete).

### 3. Dashboard Local State (`App.tsx`)

```
Dashboard
  ├── selectedRule: Rule | null       ← which rule is selected in sidebar
  ├── tab: 'editor' | 'test' | 'visual' | 'history'
  ├── adminView: 'webhooks' | 'users' | null
  ├── environment: string             ← filter passed to useRules
  └── toast: { msg, type } | null     ← auto-dismiss after 3s
```

Each component receives what it needs via props. No component reaches up to parent state. Data flows down, events flow up via callbacks.

## Data Flow Diagram

```
                    ┌──────────────────────────┐
                    │     React Dashboard       │
                    └────────────┬─────────────┘
                                 │
                    ┌────────────▼─────────────┐
                    │      api/client.ts        │
                    │                           │
                    │  request<T>(path, opts)   │ ← Adds Content-Type + Authorization header
                    │  ├── fetch(BASE + path)   │ ← BASE = '/api'
                    │  ├── res.json()           │
                    │  ├── if 401 → clearToken  │ ← Auto-logout on expired token
                    │  └── throw on non-2xx     │
                    └────────────┬─────────────┘
                                 │ HTTP
                    ┌────────────▼─────────────┐
                    │     Go Backend (:8080)     │
                    │                           │
                    │  Same binary serves both  │
                    │  API (/api/*) and static   │
                    │  files (index.html, JS)    │
                    └───────────────────────────┘
```

**Key detail**: In production, there is no separate frontend server. The React app is compiled to static files (`web/dist/`), which are embedded into the Go binary via `//go:embed`. The Go server serves both the API and the dashboard. For development, you can run `npm run dev` for Vite's hot-reload server.

## Component Details

### TopBar

```
[Arbiter] [Environment ▼] ──────────── [Users] [Webhooks] [Import Rule] [admin badge] [Logout]
```

- Environment dropdown: "All Environments", "Production", "Staging", "Development"
- Changing environment updates the filter in Dashboard, which triggers useRules to re-fetch
- Users and Webhooks buttons only appear for admin role
- Import reads a `.json` file, calls `POST /api/rules/import`, handles conflict with force-overwrite prompt

### RuleList (Sidebar)

Each rule shows:
```
┌──────────────────────────────┐
│ New User Onboarding          │
│ [feature flag] [active] [production] v3  │
└──────────────────────────────┘
```

Create form fields: ID (slug), Name, Type (feature_flag/decision_tree/kill_switch), Environment.

### Editor

The main editing interface. Two sections:

**Metadata fields:**
```
┌─────────────┬──────────────┬──────────┬──────────────┐
│ Name        │ Type ▼       │ Status ▼ │ Environment ▼│
├─────────────┴──────────────┴──────────┴──────────────┤
│ Description                                          │
├──────────────────────┬───────────────────────────────┤
│ Active From (picker) │ Active Until (picker)         │
└──────────────────────┴───────────────────────────────┘
```

**JSON editors:**
- Decision Tree: CodeMirror with JSON syntax highlighting (300px height)
- Default Value: Smaller CodeMirror (60px)

Save parses JSON, validates, calls `PUT /api/rules/{id}`, and includes environment + schedule fields.

### HistoryView

Two sub-tabs:

**Versions tab:**
```
v3  New User Onboarding  [active]  by admin  2024-01-15 14:30  [current]
v2  New User Onboarding  [draft]   by editor 2024-01-14 10:00  [Show Diff] [Rollback]
v1  New User Onboarding  [draft]   by admin  2024-01-13 09:00  [Show Diff] [Rollback]
```

The `by {username}` label comes from `modified_by` in the API response. Shows who made each change.

**Evaluations tab:**
Lists past evaluation results with context JSON and returned value.

### UserManager (Admin Only)

```
┌─────────────────────────────────────────────────────┐
│ User Management                          [+ New User]│
├─────────────────────────────────────────────────────┤
│ Create form (collapsible):                          │
│ [Username] [Password] [Role ▼] [Create User]       │
├──────────────┬──────────┬───────────────────────────┤
│ Username     │ Role     │ Created                   │
├──────────────┼──────────┼───────────────────────────┤
│ admin        │ [admin]  │ 2024-01-13                │
│ editor1      │ [editor] │ 2024-01-14                │
│ viewer1      │ [viewer] │ 2024-01-15                │
└──────────────┴──────────┴───────────────────────────┘
```

Role badges are color-coded: red for admin, yellow for editor, blue for viewer.

## API Client Design

`api/client.ts` uses a single `request<T>()` function for all API calls. Every endpoint is a one-liner:

```typescript
// All calls go through this
async function request<T>(path: string, options?: RequestInit): Promise<T>

// Endpoint examples
api.listRules(50, 0, 'production')    // GET /api/rules?limit=50&offset=0&environment=production
api.evaluate('rule_id', { user: {} }) // POST /api/rules/rule_id/evaluate
api.register('user', 'pass', 'editor') // POST /api/auth/register
```

**Error handling**: On non-2xx response, throws `new Error(data.error)`. On 401, clears the stored token (triggers re-login on next render).

**Token management**: The token lives in two places:
1. Module-level variable (`authToken`) for immediate use in requests
2. `localStorage` (`arbiter_token`) for persistence across page refreshes

## CSS Architecture

Single file: `styles/global.css` (~480 lines). Uses CSS custom properties for theming:

```css
:root {
  --bg-primary: #0d1117;     /* Main background */
  --bg-secondary: #161b22;   /* Cards, sidebar */
  --text-primary: #e6edf3;   /* Main text */
  --accent: #58a6ff;          /* Links, active states */
  --success: #3fb950;         /* Green badges/indicators */
  --danger: #f85149;          /* Red badges/delete buttons */
  --warning: #d29922;         /* Yellow badges */
  --border: #30363d;          /* Borders, dividers */
}
```

Layout uses CSS Grid:
```css
.layout {
  display: grid;
  grid-template-columns: 280px 1fr;   /* sidebar | main */
  grid-template-rows: 48px 1fr;        /* topbar | content */
  height: 100vh;
}
```

No CSS modules, no Tailwind, no styled-components. Class names are semantic (`.card`, `.badge`, `.tab`, `.rule-item`).

## Key Design Decisions

| Decision | Why | What we traded off |
|----------|-----|-------------------|
| No router library | App is one page with tabs, not multiple pages. react-router would add complexity for no benefit | Can't bookmark specific rules by URL |
| No state library | State is small: selected rule, active tab, auth info. Prop drilling works at this scale | Would need Zustand/Redux if we added multi-panel editing |
| JWT in localStorage | Simple. Works across tabs. Survives page refresh | Vulnerable to XSS (but httpOnly cookies don't meaningfully help if attacker has XSS) |
| Embedded in Go binary | Single binary deployment. No nginx/CDN config needed | Must rebuild Go binary when changing frontend |
| Single CSS file | Fast to write, easy to find styles | Would need CSS modules if team grows beyond 2-3 people |
| CodeMirror over Monaco | ~780KB vs ~2MB bundle. Good enough for JSON editing | Monaco has richer IntelliSense, but overkill for JSON |
| Visual editor is semi-read-only | Syncing JSON text editor and visual graph in real-time is complex and error-prone | Users must use the JSON tab for full editing power |
