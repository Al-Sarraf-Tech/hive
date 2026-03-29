package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/jalsarraf0/hive/daemon/internal/auth"
)

// authStatus returns whether auth is enabled and whether initial setup is needed.
func (h *Handler) authStatus(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{
		"auth_enabled": h.authSvc != nil,
		"needs_setup":  false,
	}
	if h.authSvc != nil {
		needs, err := h.authSvc.NeedsSetup()
		if err != nil {
			writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		resp["needs_setup"] = needs
	}
	writeJSONStatus(w, http.StatusOK, resp)
}

// authSetup creates the first admin user (only works when no users exist).
func (h *Handler) authSetup(w http.ResponseWriter, r *http.Request) {
	if h.authSvc == nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "user auth not enabled — using legacy token auth"})
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	user, err := h.authSvc.Setup(req.Username, req.Password)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, auth.ErrSetupComplete) {
			status = http.StatusConflict
		}
		writeJSONStatus(w, status, map[string]string{"error": err.Error()})
		return
	}

	// Auto-login after setup
	access, refresh, err := h.authSvc.Login(req.Username, req.Password)
	if err != nil {
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSONStatus(w, http.StatusCreated, map[string]any{
		"user":          user.Info(),
		"access_token":  access,
		"refresh_token": refresh,
	})
}

// authLogin authenticates with username/password and returns JWT tokens.
func (h *Handler) authLogin(w http.ResponseWriter, r *http.Request) {
	if h.authSvc == nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "user auth not enabled — use bearer token"})
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	access, refresh, err := h.authSvc.Login(req.Username, req.Password)
	if err != nil {
		status := http.StatusUnauthorized
		if errors.Is(err, auth.ErrRateLimited) {
			status = http.StatusTooManyRequests
		}
		writeJSONStatus(w, status, map[string]string{"error": err.Error()})
		return
	}

	writeJSONStatus(w, http.StatusOK, map[string]any{
		"access_token":  access,
		"refresh_token": refresh,
	})
}

// authRefresh issues a new access token from a valid refresh token.
func (h *Handler) authRefresh(w http.ResponseWriter, r *http.Request) {
	if h.authSvc == nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "user auth not enabled"})
		return
	}

	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	access, err := h.authSvc.RefreshAccessToken(req.RefreshToken)
	if err != nil {
		writeJSONStatus(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	writeJSONStatus(w, http.StatusOK, map[string]any{
		"access_token": access,
	})
}

// authMe returns the current user's info.
func (h *Handler) authMe(w http.ResponseWriter, r *http.Request) {
	claims := h.extractClaims(r)
	if claims == nil {
		// Legacy token — return minimal info
		writeJSONStatus(w, http.StatusOK, map[string]any{
			"username": "admin",
			"role":     "admin",
			"auth_mode": "token",
		})
		return
	}

	user, err := h.authSvc.GetUser(claims.Username)
	if err != nil {
		writeJSONStatus(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSONStatus(w, http.StatusOK, map[string]any{
		"user":      user.Info(),
		"auth_mode": "jwt",
	})
}

// authChangePassword changes the calling user's password.
func (h *Handler) authChangePassword(w http.ResponseWriter, r *http.Request) {
	claims := h.extractClaims(r)
	if claims == nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "password change requires JWT auth"})
		return
	}

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if err := h.authSvc.ChangePassword(claims.Username, req.OldPassword, req.NewPassword); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, auth.ErrInvalidPassword) {
			status = http.StatusUnauthorized
		}
		writeJSONStatus(w, status, map[string]string{"error": err.Error()})
		return
	}

	writeJSONStatus(w, http.StatusOK, map[string]string{"status": "password changed"})
}

// authListUsers returns all users (admin only).
func (h *Handler) authListUsers(w http.ResponseWriter, r *http.Request) {
	if !h.requireRole(w, r, auth.RoleAdmin) {
		return
	}

	users, err := h.authSvc.ListUsers()
	if err != nil {
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSONStatus(w, http.StatusOK, map[string]any{"users": users})
}

// authCreateUser creates a new user (admin only).
func (h *Handler) authCreateUser(w http.ResponseWriter, r *http.Request) {
	if !h.requireRole(w, r, auth.RoleAdmin) {
		return
	}

	var req struct {
		Username string    `json:"username"`
		Password string    `json:"password"`
		Role     auth.Role `json:"role"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	user, err := h.authSvc.CreateUser(req.Username, req.Password, req.Role)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, auth.ErrUserExists) {
			status = http.StatusConflict
		}
		writeJSONStatus(w, status, map[string]string{"error": err.Error()})
		return
	}

	writeJSONStatus(w, http.StatusCreated, user.Info())
}

// authDeleteUser deletes a user (admin only).
func (h *Handler) authDeleteUser(w http.ResponseWriter, r *http.Request) {
	if !h.requireRole(w, r, auth.RoleAdmin) {
		return
	}

	username := r.PathValue("username")
	claims := h.extractClaims(r)
	if claims != nil && claims.Username == username {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "cannot delete yourself"})
		return
	}

	if err := h.authSvc.DeleteUser(username); err != nil {
		status := http.StatusNotFound
		if !errors.Is(err, auth.ErrUserNotFound) {
			status = http.StatusInternalServerError
		}
		writeJSONStatus(w, status, map[string]string{"error": err.Error()})
		return
	}

	writeJSONStatus(w, http.StatusOK, map[string]string{"status": "user deleted"})
}

// authSetRole changes a user's role (admin only).
func (h *Handler) authSetRole(w http.ResponseWriter, r *http.Request) {
	if !h.requireRole(w, r, auth.RoleAdmin) {
		return
	}

	username := r.PathValue("username")
	var req struct {
		Role auth.Role `json:"role"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if err := h.authSvc.SetRole(username, req.Role); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSONStatus(w, http.StatusOK, map[string]string{"status": "role updated"})
}

// extractClaims returns JWT claims from the request, or nil if using legacy token.
func (h *Handler) extractClaims(r *http.Request) *auth.Claims {
	if h.authSvc == nil {
		return nil
	}
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil
	}
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := h.authSvc.ValidateToken(tokenStr)
	if err != nil {
		return nil
	}
	return claims
}

// requireRole checks if the caller has the required role. Returns false and writes error if not.
func (h *Handler) requireRole(w http.ResponseWriter, r *http.Request, required auth.Role) bool {
	if h.authSvc == nil {
		// Legacy token mode — treat as admin
		return true
	}

	claims := h.extractClaims(r)
	if claims == nil {
		// Legacy token — treat as admin
		return true
	}

	if auth.Role(claims.Role) != auth.RoleAdmin && auth.Role(claims.Role) != required {
		writeJSONStatus(w, http.StatusForbidden, map[string]string{"error": "forbidden: admin role required"})
		return false
	}
	return true
}

// writeJSONStatus writes a JSON response with an explicit status code.
func writeJSONStatus(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// readJSON reads and decodes a JSON request body.
func readJSON(r *http.Request, v any) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}
