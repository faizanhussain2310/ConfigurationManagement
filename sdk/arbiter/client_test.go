package arbiter

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestServer creates an httptest.Server with a mux that handles the routes
// needed by the tests. Returns the server and a cleanup function.
func newTestServer() *httptest.Server {
	mux := http.NewServeMux()

	// GET /api/health
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// POST /api/rules
	mux.HandleFunc("POST /api/rules", func(w http.ResponseWriter, r *http.Request) {
		var rule Rule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
			return
		}
		rule.ID = "generated-id"
		rule.Version = 1
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(rule)
	})

	// GET /api/rules (list)
	mux.HandleFunc("GET /api/rules", func(w http.ResponseWriter, r *http.Request) {
		resp := ListRulesResponse{
			Rules: []Rule{
				{ID: "rule-1", Name: "Test Rule 1"},
				{ID: "rule-2", Name: "Test Rule 2"},
			},
			Total:  2,
			Limit:  10,
			Offset: 0,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// GET /api/rules/{id}
	mux.HandleFunc("GET /api/rules/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "not-found" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"rule not found"}`))
			return
		}
		rule := Rule{ID: id, Name: "Test Rule", Version: 1, Status: "active"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rule)
	})

	// POST /api/rules/{id}/evaluate
	mux.HandleFunc("POST /api/rules/{id}/evaluate", func(w http.ResponseWriter, r *http.Request) {
		result := EvalResult{
			Value:   true,
			Path:    []string{"root", "left"},
			Default: false,
			Elapsed: "1.2ms",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	// POST /api/evaluate (batch)
	mux.HandleFunc("POST /api/evaluate", func(w http.ResponseWriter, r *http.Request) {
		var req BatchEvalRequest
		json.NewDecoder(r.Body).Decode(&req)

		results := make(map[string]EvalResult)
		for _, id := range req.RuleIDs {
			results[id] = EvalResult{Value: true, Elapsed: "0.5ms"}
		}
		resp := BatchEvalResponse{Results: results}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	return httptest.NewServer(mux)
}

func TestHealth(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	c := New(srv.URL)
	if err := c.Health(context.Background()); err != nil {
		t.Fatalf("Health() error: %v", err)
	}
}

func TestCreateRule(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	c := New(srv.URL)
	rule, err := c.CreateRule(context.Background(), Rule{
		Name:        "My Rule",
		Description: "A test rule",
		Type:        "decision_tree",
	})
	if err != nil {
		t.Fatalf("CreateRule() error: %v", err)
	}
	if rule.ID != "generated-id" {
		t.Errorf("expected ID 'generated-id', got %q", rule.ID)
	}
	if rule.Name != "My Rule" {
		t.Errorf("expected Name 'My Rule', got %q", rule.Name)
	}
}

func TestListRules(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	c := New(srv.URL)
	resp, err := c.ListRules(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("ListRules() error: %v", err)
	}
	if len(resp.Rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(resp.Rules))
	}
	if resp.Total != 2 {
		t.Errorf("expected total 2, got %d", resp.Total)
	}
}

func TestGetRule(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	c := New(srv.URL)
	rule, err := c.GetRule(context.Background(), "abc-123")
	if err != nil {
		t.Fatalf("GetRule() error: %v", err)
	}
	if rule.ID != "abc-123" {
		t.Errorf("expected ID 'abc-123', got %q", rule.ID)
	}
	if rule.Status != "active" {
		t.Errorf("expected Status 'active', got %q", rule.Status)
	}
}

func TestGetRule_NotFound(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.GetRule(context.Background(), "not-found")
	if err == nil {
		t.Fatal("expected error for not-found rule, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", apiErr.StatusCode)
	}
	if apiErr.Message != "rule not found" {
		t.Errorf("expected message 'rule not found', got %q", apiErr.Message)
	}
}

func TestEvaluate(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	c := New(srv.URL)
	result, err := c.Evaluate(context.Background(), "rule-1", map[string]any{
		"country": "US",
		"age":     25,
	})
	if err != nil {
		t.Fatalf("Evaluate() error: %v", err)
	}
	if result.Value != true {
		t.Errorf("expected value true, got %v", result.Value)
	}
	if len(result.Path) != 2 {
		t.Errorf("expected path length 2, got %d", len(result.Path))
	}
	if result.Elapsed != "1.2ms" {
		t.Errorf("expected elapsed '1.2ms', got %q", result.Elapsed)
	}
}

func TestBatchEvaluate(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	c := New(srv.URL)
	resp, err := c.BatchEvaluate(context.Background(), []string{"rule-1", "rule-2"}, map[string]any{
		"country": "US",
	})
	if err != nil {
		t.Fatalf("BatchEvaluate() error: %v", err)
	}
	if len(resp.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(resp.Results))
	}
}

func TestWithAuth(t *testing.T) {
	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(srv.URL).WithAuth("my-jwt-token")
	_ = c.Health(context.Background())

	expected := "Bearer my-jwt-token"
	if capturedAuth != expected {
		t.Errorf("expected Authorization %q, got %q", expected, capturedAuth)
	}
}

func TestBadRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid rule type"}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.CreateRule(context.Background(), Rule{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", apiErr.StatusCode)
	}
	if apiErr.Message != "invalid rule type" {
		t.Errorf("expected message 'invalid rule type', got %q", apiErr.Message)
	}
}
