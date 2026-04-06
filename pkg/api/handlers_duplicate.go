package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// DuplicateRule handles POST /api/rules/:id/duplicate.
func (h *Handler) DuplicateRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	newID := id + "-copy"

	// Check if copy ID already exists, append number if needed
	for i := 0; ; i++ {
		candidateID := newID
		if i > 0 {
			candidateID = newID + "-" + string(rune('0'+i))
		}
		exists, err := h.Store.RuleExists(r.Context(), candidateID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !exists {
			newID = candidateID
			break
		}
		if i > 9 {
			writeError(w, http.StatusConflict, "too many copies exist")
			return
		}
	}

	rule, err := h.Store.DuplicateRule(r.Context(), id, newID, getUsername(r))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, rule)
}
