package store

const schema = `
CREATE TABLE IF NOT EXISTS rules (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    type TEXT NOT NULL CHECK (type IN ('feature_flag', 'decision_tree', 'kill_switch', 'composite')),
    version INTEGER NOT NULL DEFAULT 1,
    tree TEXT NOT NULL,
    default_value TEXT,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'draft', 'disabled')),
    environment TEXT NOT NULL DEFAULT 'production',
    active_from DATETIME,
    active_until DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS rule_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id TEXT NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    type TEXT NOT NULL,
    tree TEXT NOT NULL,
    default_value TEXT,
    status TEXT NOT NULL,
    environment TEXT NOT NULL DEFAULT 'production',
    active_from DATETIME,
    active_until DATETIME,
    modified_by TEXT DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(rule_id, version)
);

CREATE TABLE IF NOT EXISTS _meta (
    key TEXT PRIMARY KEY,
    value TEXT
);

CREATE TABLE IF NOT EXISTS eval_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id TEXT NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
    context TEXT NOT NULL,
    result TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_eval_history_rule_id ON eval_history(rule_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_rule_versions_rule_id ON rule_versions(rule_id, version DESC);
CREATE INDEX IF NOT EXISTS idx_rules_environment ON rules(environment);

CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'viewer' CHECK (role IN ('admin', 'editor', 'viewer')),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS webhook_subscriptions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url TEXT NOT NULL,
    events TEXT NOT NULL DEFAULT '*',
    secret TEXT DEFAULT '',
    active INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`
