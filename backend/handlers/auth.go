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
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// GoogleLogin handles POST /api/auth/google.
func (h *AuthHandler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req models.GoogleLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	// Verify JWT with Google tokeninfo endpoint
	resp, err := http.Get("https://oauth2.googleapis.com/tokeninfo?id_token=" + req.Credential)
	if err != nil || resp.StatusCode != http.StatusOK {
		http.Error(w, `{"error":"Invalid Google token"}`, http.StatusUnauthorized)
		return
	}
	defer resp.Body.Close()

	var tokenInfo struct {
		Email         string `json:"email"`
		EmailVerified string `json:"email_verified"` // sometimes bool, sometimes string "true"
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenInfo); err != nil {
		http.Error(w, `{"error":"Failed to parse Google response"}`, http.StatusInternalServerError)
		return
	}

	// Allowlist check: authorized users
	allowlist := map[string]bool{
		"ferdeveloperwork@gmail.com": true,
		"luis@videodigital.com.ar":   true,
		"lautaropelozo@gmail.com":    true,
	}

	if !allowlist[tokenInfo.Email] {
		http.Error(w, `{"error":"No estás autorizado para acceder a este sistema."}`, http.StatusForbidden)
		return
	}

	// Generate JWT token for our system
	token, err := auth.GenerateToken(tokenInfo.Email, h.cfg.JWTSecret, h.cfg.JWTExpiry)
	if err != nil {
		http.Error(w, `{"error":"Failed to generate token"}`, http.StatusInternalServerError)
		return
	}

	loginResp := models.LoginResponse{
		Token: token,
		User:  tokenInfo.Email,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(loginResp)
}

// ValidateToken handles GET /api/validate - checks if a token is still valid.
func (h *AuthHandler) ValidateToken(w http.ResponseWriter, r *http.Request) {
	// If we reach here, the auth middleware already validated the token
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"valid": true})
}
