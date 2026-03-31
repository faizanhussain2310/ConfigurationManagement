package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// ListVersions handles GET /api/rules/:id/versions.
func (h *Handler) ListVersions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	versions, err := h.Store.ListVersions(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"versions": versions})
}

// RollbackToVersion handles POST /api/rules/:id/rollback/:version.
func (h *Handler) RollbackToVersion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	versionStr := chi.URLParam(r, "version")

	version, err := strconv.Atoi(versionStr)
	if err != nil || version < 1 {
		writeError(w, http.StatusBadRequest, "invalid version number")
		return
	}

	rule, err := h.Store.RollbackToVersion(r.Context(), id, version)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "rule or version not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, rule)
}
