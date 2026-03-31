package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/faizanhussain/arbiter/pkg/engine"
	"github.com/go-chi/chi/v5"
)

// ExportRule handles GET /api/rules/:id/export. Downloads rule as JSON file.
func (h *Handler) ExportRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	rule, err := h.Store.GetRule(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "rule not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename="+rule.ID+".json")
	json.NewEncoder(w).Encode(rule)
}

// ImportRule handles POST /api/rules/import. Accepts application/json body.
func (h *Handler) ImportRule(w http.ResponseWriter, r *http.Request) {
	var rule engine.Rule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if err := engine.ValidateRule(&rule); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	force := strings.ToLower(r.URL.Query().Get("force")) == "true"

	if err := h.Store.ImportRule(r.Context(), &rule, force); err != nil {
		if strings.Contains(err.Error(), "conflict") {
			existing, _ := h.Store.GetRule(r.Context(), rule.ID)
			writeJSON(w, http.StatusConflict, map[string]any{
				"error":    err.Error(),
				"existing": existing,
			})
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Fetch the final state
	imported, err := h.Store.GetRule(r.Context(), rule.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, imported)
}
