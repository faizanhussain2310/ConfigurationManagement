# Frontend Overview

The dashboard is a React 18 SPA built with Vite. No routing library... it's a single page with a sidebar, tabs, and conditional rendering. The entire frontend compiles to ~775KB (gzipped ~250KB) and gets embedded into the Go binary via `//go:embed`.

## Architecture

```
main.tsx                    ← mounts React, wraps everything in AuthProvider
  └─ App.tsx                ← auth gate: shows LoginPage or Dashboard
      ├─ LoginPage.tsx      ← username/password form
      └─ Dashboard          ← main layout (sidebar + tabs + content)
           ├─ TopBar.tsx         ← header with import, webhooks, user info, logout
           ├─ RuleList.tsx       ← sidebar: list rules + create new
           ├─ Editor.tsx         ← JSON editor tab (CodeMirror)
           ├─ TestPanel.tsx      ← evaluate rules with custom context
           ├─ TreeEditor.tsx     ← visual drag-and-drop tree (ReactFlow)
           ├─ HistoryView.tsx    ← versions + evaluation history
           ├─ WebhookManager.tsx ← admin-only webhook CRUD
           ├─ DiffView.tsx       ← version comparison
           └─ PathView.tsx       ← decision path visualization
```

## How state flows

There's no Redux, no Zustand, no state library. Just React hooks and prop drilling.

**Auth state** lives in `useAuth.tsx` (React Context). The `AuthProvider` wraps the entire app. Every component can call `useAuth()` to get `username`, `role`, `login()`, `logout()`. JWT token is stored in `localStorage` and automatically attached to every API request by `client.ts`.

**Rule list** lives in `useRules.ts` hook. Called once in `Dashboard`. Returns `rules`, `loading`, `refresh()`. When you create/update/delete a rule, the parent calls `refresh()` to re-fetch the list.

**Selected rule** is `useState<Rule | null>` in Dashboard. Passed down as props to Editor, TestPanel, TreeEditor, HistoryView.

**Tab state** is `useState<Tab>` in Dashboard. Switches which component renders in the main area.

**Toast notifications** are a simple `useState` with a 3-second `setTimeout` to auto-dismiss.

## File-by-file breakdown

### `api/client.ts` — API client (140 lines)

All HTTP calls go through one `request<T>()` function that:
- Adds `Content-Type: application/json`
- Adds `Authorization: Bearer <token>` if logged in
- Throws on non-2xx responses (parses the error message from the JSON body)
- On 401, clears the stored token (forces re-login)

The `api` object exposes every endpoint as a typed function. Auth endpoints (login, me, register, listUsers), rule CRUD, evaluation, history, versions, webhooks.

`setToken()` and `getToken()` manage the JWT in both memory and localStorage.

### `hooks/useAuth.tsx` — Auth context (63 lines)

On mount: checks if a token exists in localStorage. If yes, calls `api.me()` to validate it. If the token is expired or invalid, clears it.

`login()` calls the API, stores the token, sets username + role.
`logout()` clears everything.

### `components/LoginPage.tsx` — Login form (77 lines)

Simple form with username, password, submit button. Shows error messages inline. Displays "Default: admin / admin" hint.

### `components/TopBar.tsx` — Header bar (49 lines)

Shows: app name, webhooks button (admin only), import button (file picker), username + role badge, logout button.

The import flow: reads a `.json` file, calls `api.importRule()`. If it gets a conflict (rule already exists), asks user to confirm force-overwrite.

### `components/RuleList.tsx` — Sidebar (96 lines)

Two parts:
1. **Create form** — collapsible. Generates a random UUID for the rule ID, lets you pick a name and type, then calls `api.createRule()` with a minimal tree
2. **Rule list** — maps over `rules` array, shows name + type badge + status badge. Clicking a rule calls `onSelect(rule)`

Active rule gets highlighted with a left border accent.

### `components/Editor.tsx` — Rule editor (178 lines)

The JSON editing tab. Uses CodeMirror 6 (`@uiw/react-codemirror`) with JSON syntax highlighting.

State is local: `name`, `description`, `type`, `status`, `treeJson`, `defaultJson`. When you select a different rule, `useEffect` resets all state from the new rule's data.

Save flow: parse JSON → validate → call `api.updateRule()` → notify parent.

Also has: export (opens download URL), duplicate (calls API), delete (with confirmation dialog).

**Tradeoff:** We store the tree as a JSON string in state and parse it on save, rather than keeping a parsed object. This is simpler but means the JSON editor and visual editor can't share live state... they're independent views of the same data.

### `components/TreeEditor.tsx` — Visual tree editor (293 lines)

The most complex frontend component. Uses ReactFlow (`@xyflow/react`) to render the decision tree as a node graph.

**Three custom node types:**
- `ConditionNode` — field input, operator dropdown, value input. Two output handles: "then" (green) and "else" (red)
- `ValueNode` — single input for the leaf value. One input handle
- `CombinatorNode` — AND/OR dropdown with condition count. Two output handles like ConditionNode

**Tree → Flow conversion:** `treeToFlow()` recursively walks the tree JSON and creates ReactFlow nodes + edges. Each node gets callbacks that mutate the original tree object and trigger a re-render.

**Layout:** Nodes are positioned by the recursive function: parent at (x, y), then-child at (x-150, y+150), else-child at (x+150, y+150). This creates a binary tree layout. Users can drag nodes to rearrange.

**Tradeoff:** The visual editor is read-only in the current integration. It reads from `selectedRule.tree` but doesn't write back changes to the Editor tab. The `onChange` prop exists but isn't wired up in App.tsx. Reason: syncing two editors (JSON text and visual graph) in real-time is complex and error-prone. For v2, the visual editor is a visualization tool with inline editing on individual nodes.

### `components/TestPanel.tsx` — Rule tester (84 lines)

JSON textarea for context input. "Evaluate" button calls `api.evaluate()`. Shows the result: value, default flag, elapsed time, and the decision path via `PathView`.

### `components/HistoryView.tsx` — Version history (127 lines)

Two sub-tabs:
1. **Versions** — lists all versions with rollback buttons. Clicking a version shows a DiffView comparing it to the current version
2. **Evaluations** — recent evaluation history with context and result JSON

### `components/DiffView.tsx` — Version diff (72 lines)

Side-by-side comparison of two rule versions. Shows changes in name, status, description, and tree JSON. Changed fields are highlighted with green (added) / red (removed) styling.

**Tradeoff:** This is a simplified diff, not a line-by-line JSON diff. It compares top-level fields. For most use cases this is enough, but deep tree changes show as "entire tree changed" rather than highlighting the specific node that changed.

### `components/PathView.tsx` — Decision path (26 lines)

Renders the `path` array from an EvalResult as colored steps. "→ true" steps are green, "→ false" steps are red, "→ value" steps are blue.

### `components/WebhookManager.tsx` — Webhook admin (126 lines)

Admin-only view. Create form with URL + events input. Table listing all webhooks with truncated secrets and delete buttons. Only visible to users with admin role.

### `styles/global.css` — All styles (479 lines)

Single CSS file with CSS custom properties (variables) for the dark theme. No CSS modules, no styled-components, no Tailwind. Just class names.

Key sections:
- CSS variables (`:root`) — colors, radius
- Layout grid (sidebar + topbar + main)
- Form controls (inputs, buttons, selects)
- Rule list items with active state
- Tabs with underline indicator
- Cards for content sections
- Toast notifications with slide-in animation
- ReactFlow node styling (condition/value/combinator colors)
- ReactFlow dark theme overrides

## Key tradeoffs

| Decision | Why | Alternative considered |
|----------|-----|----------------------|
| No router library | Single-page app with tabs, not multiple pages. Adding react-router would be overhead for no benefit | react-router |
| No state management library | App state is small (selected rule + tab + auth). Prop drilling works fine at this scale | Redux, Zustand |
| CodeMirror for JSON editing | Syntax highlighting, error markers, folding. Textarea would be painful for large trees | Monaco (too heavy at ~2MB) |
| ReactFlow for visual editor | Best React library for node graphs. D3 was used in v1 but was read-only | react-d3-tree (replaced) |
| Single CSS file | Fast to write, easy to find styles. CSS modules would add complexity without clear benefit at this scale | CSS modules, Tailwind |
| JWT in localStorage | Simple. Works across tabs. The common objection is XSS vulnerability, but if an attacker has XSS, httpOnly cookies don't help much either (they can just make requests from the compromised page) | httpOnly cookies |
| Auth context in hooks | Clean separation. Components don't need to know about localStorage or tokens | Passing token as prop everywhere |

## Building

```bash
cd web
npm install       # install dependencies
npm run build     # TypeScript check + Vite production build → dist/
```

The built files land in `web/dist/`. The Go binary embeds this directory via `//go:embed` in `web/embed.go`. When you run the server, it serves these files directly... no separate frontend server needed.

For development, you can run `npm run dev` for Vite's dev server with hot reload, but you'll need to proxy API calls to the Go backend (or just run the Go server and rebuild on changes).
