package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/faizanhussain/arbiter/pkg/auth"
	"github.com/go-chi/chi/v5"
)

// CreateWebhook handles POST /api/webhooks.
func (h *Handler) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL    string `json:"url"`
		Events string `json:"events"` // comma-separated or "*"
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}
	if body.Events == "" {
		body.Events = "*"
	}

	secret := auth.GenerateWebhookSecret()

	hook, err := h.Store.CreateWebhook(r.Context(), body.URL, body.Events, secret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, hook)
}

// ListWebhooks handles GET /api/webhooks.
func (h *Handler) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	hooks, err := h.Store.ListWebhooks(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"webhooks": hooks})
}

// DeleteWebhook handles DELETE /api/webhooks/:id.
func (h *Handler) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid webhook ID")
		return
	}

	if err := h.Store.DeleteWebhook(r.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "webhook not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
