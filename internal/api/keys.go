package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/eniz1806/VaultS3/internal/metadata"
)

type keyListItem struct {
	AccessKey    string `json:"accessKey"`
	MaskedSecret string `json:"maskedSecret"`
	CreatedAt    string `json:"createdAt"`
	IsAdmin      bool   `json:"isAdmin"`
}

type keyCreateResponse struct {
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
	CreatedAt string `json:"createdAt"`
}

func (h *APIHandler) handleListKeys(w http.ResponseWriter, _ *http.Request) {
	keys, err := h.store.ListAccessKeys()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list keys")
		return
	}

	items := make([]keyListItem, 0, len(keys)+1)

	// Include admin key
	items = append(items, keyListItem{
		AccessKey:    h.cfg.Auth.AdminAccessKey,
		MaskedSecret: maskSecret(h.cfg.Auth.AdminSecretKey),
		CreatedAt:    "",
		IsAdmin:      true,
	})

	for _, k := range keys {
		items = append(items, keyListItem{
			AccessKey:    k.AccessKey,
			MaskedSecret: maskSecret(k.SecretKey),
			CreatedAt:    k.CreatedAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, items)
}

func (h *APIHandler) handleCreateKey(w http.ResponseWriter, _ *http.Request) {
	accessKey, err := randomHex(10) // 20 hex chars
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate access key")
		return
	}

	secretKey, err := randomHex(20) // 40 hex chars
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate secret key")
		return
	}

	now := time.Now().UTC()
	key := metadata.AccessKey{
		AccessKey: accessKey,
		SecretKey: secretKey,
		CreatedAt: now,
	}

	if err := h.store.CreateAccessKey(key); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create access key")
		return
	}

	writeJSON(w, http.StatusCreated, keyCreateResponse{
		AccessKey: accessKey,
		SecretKey: secretKey,
		CreatedAt: now.Format(time.RFC3339),
	})
}

func (h *APIHandler) handleDeleteKey(w http.ResponseWriter, _ *http.Request, accessKey string) {
	if accessKey == h.cfg.Auth.AdminAccessKey {
		writeError(w, http.StatusForbidden, "cannot delete admin key")
		return
	}

	if _, err := h.store.GetAccessKey(accessKey); err != nil {
		writeError(w, http.StatusNotFound, "key not found")
		return
	}

	if err := h.store.DeleteAccessKey(accessKey); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete key")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func maskSecret(secret string) string {
	if len(secret) <= 8 {
		return "****"
	}
	return secret[:4] + "****" + secret[len(secret)-4:]
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
