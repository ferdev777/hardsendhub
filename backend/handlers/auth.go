package handlers

import (
	"encoding/json"
	"net/http"

	"hardsend/auth"
	"hardsend/config"
	"hardsend/models"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	cfg *config.Config
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{cfg: cfg}
}

// Login handles POST /api/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	// Validate credentials against hardcoded superadmin
	if req.Username != h.cfg.AdminUsername || req.Password != h.cfg.AdminPassword {
		http.Error(w, `{"error":"Invalid credentials"}`, http.StatusUnauthorized)
		return
	}

	// Generate JWT token
	token, err := auth.GenerateToken(req.Username, h.cfg.JWTSecret, h.cfg.JWTExpiry)
	if err != nil {
		http.Error(w, `{"error":"Failed to generate token"}`, http.StatusInternalServerError)
		return
	}

	resp := models.LoginResponse{
		Token: token,
		User:  req.Username,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ValidateToken handles GET /api/validate - checks if a token is still valid.
func (h *AuthHandler) ValidateToken(w http.ResponseWriter, r *http.Request) {
	// If we reach here, the auth middleware already validated the token
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"valid": true})
}
