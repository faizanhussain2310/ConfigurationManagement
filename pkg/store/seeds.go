package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"time"

	"github.com/faizanhussain/arbiter/pkg/engine"
	"golang.org/x/crypto/bcrypt"
)

func bcryptHash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

// Seed inserts example rules on first run. Only seeds if:
// 1. The rules table is empty
// 2. The _meta table has no "seeded_at" key
func (s *Store) Seed(ctx context.Context) error {
	// Check if already seeded
	_, err := s.GetMeta(ctx, "seeded_at")
	if err == nil {
		return nil // already seeded
	}
	if err != sql.ErrNoRows {
		return err
	}

	// Check if rules table is empty
	var count int
	if err := s.readDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM rules`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil // rules exist, don't seed
	}

	log.Println("Seeding database with example rules...")

	for _, r := range seedRules() {
		if err := s.CreateRule(ctx, r); err != nil {
			return err
		}
	}

	return s.SetMeta(ctx, "seeded_at", time.Now().UTC().Format(time.RFC3339))
}

func seedRules() []*engine.Rule {
	return []*engine.Rule{
		{
			ID:          "new_user_onboarding",
			Name:        "New User Onboarding",
			Description: "Shows onboarding flow for new users based on signup date and country",
			Type:        "feature_flag",
			Tree: toJSON(map[string]any{
				"condition": map[string]any{
					"combinator": "and",
					"conditions": []any{
						map[string]any{"field": "user.signup_days", "op": "lte", "value": 7},
						map[string]any{"field": "user.country", "op": "in", "value": []any{"US", "CA", "GB", "AU"}},
					},
				},
				"then": map[string]any{"value": true},
				"else": map[string]any{"value": false},
			}),
			DefaultValue: toJSON(false),
			Status:       "active",
		},
		{
			ID:          "pricing_tier",
			Name:        "Pricing Tier",
			Description: "Returns pricing tier based on organization size",
			Type:        "decision_tree",
			Tree: toJSON(map[string]any{
				"condition": map[string]any{"field": "org.employees", "op": "gte", "value": 100},
				"then":      map[string]any{"value": "enterprise"},
				"else": map[string]any{
					"condition": map[string]any{"field": "org.employees", "op": "gte", "value": 10},
					"then":      map[string]any{"value": "team"},
					"else":      map[string]any{"value": "starter"},
				},
			}),
			DefaultValue: toJSON("starter"),
			Status:       "active",
		},
		{
			ID:          "dark_mode_rollout",
			Name:        "Dark Mode Rollout",
			Description: "Rolling out dark mode to 25% of users",
			Type:        "feature_flag",
			Tree: toJSON(map[string]any{
				"condition": map[string]any{"field": "user.id", "op": "pct", "value": 25},
				"then":      map[string]any{"value": true},
				"else":      map[string]any{"value": false},
			}),
			DefaultValue: toJSON(false),
			Status:       "active",
		},
		{
			ID:          "emergency_shutdown",
			Name:        "Emergency Shutdown",
			Description: "Kill switch for disabling all non-essential features during incidents",
			Type:        "kill_switch",
			Tree: toJSON(map[string]any{
				"value": false,
			}),
			DefaultValue: toJSON(false),
			Status:       "active",
		},
	}
}

// SeedAdmin creates a default admin user if no users exist.
// Default credentials: admin / admin
func (s *Store) SeedAdmin(ctx context.Context) error {
	count, err := s.UserCount(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	log.Println("Seeding default admin user (admin/admin)...")

	// bcrypt hash of "admin" at cost 10
	adminHash, hashErr := bcryptHash("admin")
	if hashErr != nil {
		return hashErr
	}

	_, err = s.CreateUser(ctx, "admin", adminHash, "admin")
	return err
}

func toJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
