package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/faizanhussain/arbiter/pkg/auth"
	"github.com/faizanhussain/arbiter/pkg/engine"
	"github.com/faizanhussain/arbiter/pkg/store"
	"github.com/faizanhussain/arbiter/pkg/webhooks"
	"github.com/go-chi/chi/v5"
)

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	Store    *store.Store
	Auth     *auth.Config
	Webhooks *webhooks.Dispatcher
}

// --- JSON helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func parsePagination(r *http.Request, defaultLimit, maxLimit int) (int, int) {
	limit := defaultLimit
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	return limit, offset
}

// --- Core Rule Handlers ---

// HealthCheck returns server status.
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// CreateRule handles POST /api/rules.
func (h *Handler) CreateRule(w http.ResponseWriter, r *http.Request) {
	var rule engine.Rule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if err := engine.ValidateRule(&rule); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Check for conflicts
	exists, err := h.Store.RuleExists(r.Context(), rule.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if exists {
		writeError(w, http.StatusConflict, "rule with id '"+rule.ID+"' already exists")
		return
	}

	if err := h.Store.CreateRule(r.Context(), &rule); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if h.Webhooks != nil {
		h.Webhooks.Fire(webhooks.EventRuleCreated, rule.ID, rule)
	}

	writeJSON(w, http.StatusCreated, rule)
}

// ListRules handles GET /api/rules.
func (h *Handler) ListRules(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r, 50, 100)

	rules, total, err := h.Store.ListRules(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"rules":  rules,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetRule handles GET /api/rules/:id.
func (h *Handler) GetRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rule, err := h.Store.GetRule(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "rule not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

// UpdateRule handles PUT /api/rules/:id.
func (h *Handler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var rule engine.Rule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	rule.ID = id

	if err := engine.ValidateRule(&rule); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.Store.UpdateRule(r.Context(), &rule); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "rule not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Re-fetch for complete response
	updated, err := h.Store.GetRule(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if h.Webhooks != nil {
		h.Webhooks.Fire(webhooks.EventRuleUpdated, id, updated)
	}

	writeJSON(w, http.StatusOK, updated)
}

// DeleteRule handles DELETE /api/rules/:id.
func (h *Handler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.Store.DeleteRule(r.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "rule not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	engine.ClearRegexCache()

	if h.Webhooks != nil {
		h.Webhooks.Fire(webhooks.EventRuleDeleted, id, nil)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// EvaluateRule handles POST /api/rules/:id/evaluate.
func (h *Handler) EvaluateRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	rule, err := h.Store.GetRule(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "rule not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var body struct {
		Context map[string]any `json:"context"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if body.Context == nil {
		body.Context = map[string]any{}
	}

	// Composite rules evaluate differently
	if rule.Type == "composite" {
		h.evaluateComposite(w, r, rule, body.Context)
		return
	}

	result := engine.Evaluate(rule.Tree, body.Context, rule.ID, rule.DefaultValue)

	// Store eval history asynchronously
	ctxJSON, _ := json.Marshal(body.Context)
	resultJSON, _ := json.Marshal(result)
	go h.Store.InsertEvalHistory(r.Context(), id, ctxJSON, resultJSON)

	writeJSON(w, http.StatusOK, result)
}

// BatchEvaluate handles POST /api/evaluate.
func (h *Handler) BatchEvaluate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RuleIDs []string       `json:"rule_ids"`
		Context map[string]any `json:"context"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if body.Context == nil {
		body.Context = map[string]any{}
	}

	type batchResult struct {
		RuleID string            `json:"rule_id"`
		Result engine.EvalResult `json:"result"`
	}

	results := make([]batchResult, len(body.RuleIDs))

	// Use errgroup with worker pool for parallel evaluation
	type indexedResult struct {
		idx    int
		result batchResult
	}
	ch := make(chan indexedResult, len(body.RuleIDs))
	sem := make(chan struct{}, 10) // max 10 concurrent

	for i, id := range body.RuleIDs {
		sem <- struct{}{}
		go func(idx int, ruleID string) {
			defer func() { <-sem }()

			rule, err := h.Store.GetRule(r.Context(), ruleID)
			if err != nil {
				ch <- indexedResult{idx, batchResult{
					RuleID: ruleID,
					Result: engine.EvalResult{Error: "rule not found", Path: []string{}},
				}}
				return
			}

			result := engine.Evaluate(rule.Tree, body.Context, rule.ID, rule.DefaultValue)
			ch <- indexedResult{idx, batchResult{RuleID: ruleID, Result: result}}
		}(i, id)
	}

	for range body.RuleIDs {
		ir := <-ch
		results[ir.idx] = ir.result
	}

	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

// evaluateComposite handles evaluation of composite rules.
func (h *Handler) evaluateComposite(w http.ResponseWriter, r *http.Request, rule *engine.Rule, ctx map[string]any) {
	config, err := engine.ParseComposeConfig(rule.Tree)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid composite config: "+err.Error())
		return
	}

	// Evaluate each child rule
	var children []engine.CompositeChild
	for _, childID := range config.RuleIDs {
		child, err := h.Store.GetRule(r.Context(), childID)
		if err != nil {
			children = append(children, engine.CompositeChild{
				RuleID: childID,
				Result: engine.EvalResult{Error: "child rule not found: " + childID, Path: []string{}},
			})
			continue
		}

		// Prevent evaluating nested composite rules (max 1 level)
		if child.Type == "composite" {
			children = append(children, engine.CompositeChild{
				RuleID: childID,
				Result: engine.EvalResult{Error: "nested composite rules not supported", Path: []string{}},
			})
			continue
		}

		result := engine.Evaluate(child.Tree, ctx, child.ID, child.DefaultValue)
		children = append(children, engine.CompositeChild{
			RuleID: childID,
			Result: result,
		})
	}

	combined := engine.CombineResults(config.Strategy, children)

	// Store eval history asynchronously
	ctxJSON, _ := json.Marshal(ctx)
	resultJSON, _ := json.Marshal(combined)
	go h.Store.InsertEvalHistory(r.Context(), rule.ID, ctxJSON, resultJSON)

	writeJSON(w, http.StatusOK, combined)
}

// GetEvalHistory handles GET /api/rules/:id/history.
func (h *Handler) GetEvalHistory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	limit, offset := parsePagination(r, 50, 100)

	entries, total, err := h.Store.ListEvalHistory(r.Context(), id, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"entries": entries,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}
