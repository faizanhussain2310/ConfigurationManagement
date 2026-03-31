package store

import (
	"context"
	"database/sql"
	"time"
)

// Webhook represents a webhook subscription.
type Webhook struct {
	ID        int       `json:"id"`
	URL       string    `json:"url"`
	Events    string    `json:"events"` // comma-separated: "rule.created,rule.updated" or "*"
	Secret    string    `json:"secret,omitempty"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateWebhook inserts a new webhook subscription.
func (s *Store) CreateWebhook(ctx context.Context, url, events, secret string) (*Webhook, error) {
	now := time.Now().UTC()
	result, err := s.writeDB.ExecContext(ctx,
		`INSERT INTO webhook_subscriptions (url, events, secret, active, created_at) VALUES (?, ?, ?, 1, ?)`,
		url, events, secret, now,
	)
	if err != nil {
		return nil, err
	}
	id, _ := result.LastInsertId()
	return &Webhook{
		ID:        int(id),
		URL:       url,
		Events:    events,
		Secret:    secret,
		Active:    true,
		CreatedAt: now,
	}, nil
}

// ListWebhooks returns all webhook subscriptions.
func (s *Store) ListWebhooks(ctx context.Context) ([]Webhook, error) {
	rows, err := s.readDB.QueryContext(ctx,
		`SELECT id, url, events, secret, active, created_at FROM webhook_subscriptions ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hooks []Webhook
	for rows.Next() {
		var h Webhook
		var active int
		if err := rows.Scan(&h.ID, &h.URL, &h.Events, &h.Secret, &active, &h.CreatedAt); err != nil {
			return nil, err
		}
		h.Active = active == 1
		hooks = append(hooks, h)
	}
	if hooks == nil {
		hooks = []Webhook{}
	}
	return hooks, rows.Err()
}

// GetActiveWebhooks returns active webhooks matching the given event.
func (s *Store) GetActiveWebhooks(ctx context.Context, event string) ([]Webhook, error) {
	rows, err := s.readDB.QueryContext(ctx,
		`SELECT id, url, events, secret, active, created_at FROM webhook_subscriptions
		 WHERE active = 1 AND (events = '*' OR events LIKE '%' || ? || '%')`, event)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hooks []Webhook
	for rows.Next() {
		var h Webhook
		var active int
		if err := rows.Scan(&h.ID, &h.URL, &h.Events, &h.Secret, &active, &h.CreatedAt); err != nil {
			return nil, err
		}
		h.Active = active == 1
		hooks = append(hooks, h)
	}
	return hooks, rows.Err()
}

// DeleteWebhook removes a webhook subscription.
func (s *Store) DeleteWebhook(ctx context.Context, id int) error {
	result, err := s.writeDB.ExecContext(ctx,
		`DELETE FROM webhook_subscriptions WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
