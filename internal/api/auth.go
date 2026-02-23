package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

type loginRequest struct {
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
}

type loginResponse struct {
	Token string `json:"token"`
}

type meResponse struct {
	User      string `json:"user"`
	AccessKey string `json:"accessKey"`
}

func (h *APIHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.AccessKey != h.cfg.Auth.AdminAccessKey || req.SecretKey != h.cfg.Auth.AdminSecretKey {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := h.jwt.Generate("admin", 24*time.Hour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	writeJSON(w, http.StatusOK, loginResponse{Token: token})
}

func (h *APIHandler) handleMe(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, meResponse{
		User:      "admin",
		AccessKey: h.cfg.Auth.AdminAccessKey,
	})
}

func (h *APIHandler) authenticate(r *http.Request) error {
	// Check Authorization header first
	auth := r.Header.Get("Authorization")
	if auth != "" {
		token := strings.TrimPrefix(auth, "Bearer ")
		if token == auth {
			return fmt.Errorf("invalid authorization format")
		}
		_, err := h.jwt.Validate(token)
		return err
	}

	// Fall back to query param (for browser download links)
	if token := r.URL.Query().Get("token"); token != "" {
		_, err := h.jwt.Validate(token)
		return err
	}

	return fmt.Errorf("missing authorization")
}
