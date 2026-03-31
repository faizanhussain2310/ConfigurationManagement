package api

import (
	"encoding/json"
	"net/http"

	"github.com/faizanhussain/arbiter/pkg/auth"
)

// Login handles POST /api/auth/login.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Username == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password required")
		return
	}

	user, err := h.Store.GetUserByUsername(r.Context(), body.Username)
	if err != nil || !auth.CheckPassword(body.Password, user.PasswordHash) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := h.Auth.GenerateToken(user.Username, user.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token":    token,
		"username": user.Username,
		"role":     user.Role,
	})
}

// Register handles POST /api/auth/register. Admin only.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Username == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password required")
		return
	}
	if body.Role == "" {
		body.Role = "viewer"
	}
	validRoles := map[string]bool{"admin": true, "editor": true, "viewer": true}
	if !validRoles[body.Role] {
		writeError(w, http.StatusBadRequest, "role must be admin, editor, or viewer")
		return
	}

	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user, err := h.Store.CreateUser(r.Context(), body.Username, hash, body.Role)
	if err != nil {
		writeError(w, http.StatusConflict, "username already exists")
		return
	}

	writeJSON(w, http.StatusCreated, user)
}

// Me handles GET /api/auth/me. Returns current user info from JWT.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	claims := GetClaims(r)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"username": claims.Username,
		"role":     claims.Role,
	})
}

// ListUsers handles GET /api/auth/users. Admin only.
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.Store.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}
