package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/faizanhussain/arbiter/pkg/store"
)

// Event types for webhook notifications.
const (
	EventRuleCreated  = "rule.created"
	EventRuleUpdated  = "rule.updated"
	EventRuleDeleted  = "rule.deleted"
	EventRuleRollback = "rule.rollback"
)

// Payload is the JSON body sent to webhook endpoints.
type Payload struct {
	Event     string `json:"event"`
	RuleID    string `json:"rule_id"`
	Timestamp string `json:"timestamp"`
	Data      any    `json:"data,omitempty"`
}

// Dispatcher fires webhooks asynchronously.
type Dispatcher struct {
	Store  *store.Store
	Client *http.Client
}

// NewDispatcher creates a webhook dispatcher.
func NewDispatcher(s *store.Store) *Dispatcher {
	return &Dispatcher{
		Store: s,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Fire sends a webhook event to all matching subscribers asynchronously.
func (d *Dispatcher) Fire(event, ruleID string, data any) {
	go d.fireAsync(event, ruleID, data)
}

func (d *Dispatcher) fireAsync(event, ruleID string, data any) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	hooks, err := d.Store.GetActiveWebhooks(ctx, event)
	if err != nil {
		log.Printf("webhook: failed to get subscribers: %v", err)
		return
	}
	if len(hooks) == 0 {
		return
	}

	payload := Payload{
		Event:     event,
		RuleID:    ruleID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      data,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("webhook: failed to marshal payload: %v", err)
		return
	}

	for _, hook := range hooks {
		go d.deliver(hook, body)
	}
}

func (d *Dispatcher) deliver(hook store.Webhook, body []byte) {
	req, err := http.NewRequest("POST", hook.URL, bytes.NewReader(body))
	if err != nil {
		log.Printf("webhook: bad URL %s: %v", hook.URL, err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Arbiter-Webhook/1.0")

	// HMAC signature if secret is set
	if hook.Secret != "" {
		mac := hmac.New(sha256.New, []byte(hook.Secret))
		mac.Write(body)
		sig := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Arbiter-Signature", "sha256="+sig)
	}

	resp, err := d.Client.Do(req)
	if err != nil {
		log.Printf("webhook: delivery failed to %s: %v", hook.URL, err)
		return
	}
	resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("webhook: %s returned %d", hook.URL, resp.StatusCode)
	}
}
