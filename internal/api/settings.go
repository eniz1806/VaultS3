package api

import (
	"crypto/hmac"
	"net/http"
)

type settingsResponse struct {
	Server struct {
		Address             string `json:"address"`
		Port                int    `json:"port"`
		Domain              string `json:"domain,omitempty"`
		ShutdownTimeoutSecs int    `json:"shutdownTimeoutSecs"`
		TLSEnabled          bool   `json:"tlsEnabled"`
	} `json:"server"`
	Storage struct {
		DataDir     string `json:"dataDir"`
		MetadataDir string `json:"metadataDir"`
	} `json:"storage"`
	Features struct {
		Encryption  bool `json:"encryption"`
		Compression bool `json:"compression"`
		AccessLog   bool `json:"accessLog"`
		RateLimit   bool `json:"rateLimit"`
		Replication bool `json:"replication"`
		Scanner     bool `json:"scanner"`
		Tiering     bool `json:"tiering"`
		Backup      bool `json:"backup"`
		OIDC        bool `json:"oidc"`
		Lambda      bool `json:"lambda"`
		Debug       bool `json:"debug"`
	} `json:"features"`
	Lifecycle struct {
		ScanIntervalSecs   int `json:"scanIntervalSecs"`
		AuditRetentionDays int `json:"auditRetentionDays"`
	} `json:"lifecycle"`
	RateLimit struct {
		RequestsPerSec float64 `json:"requestsPerSec"`
		BurstSize      int     `json:"burstSize"`
		PerKeyRPS      float64 `json:"perKeyRps"`
		PerKeyBurst    int     `json:"perKeyBurst"`
	} `json:"rateLimit,omitempty"`
	Memory struct {
		MaxSearchEntries int `json:"maxSearchEntries"`
		GoMemLimitMB     int `json:"goMemLimitMb,omitempty"`
	} `json:"memory"`
}

func (h *APIHandler) handleSettings(w http.ResponseWriter, _ *http.Request) {
	var resp settingsResponse

	resp.Server.Address = h.cfg.Server.Address
	resp.Server.Port = h.cfg.Server.Port
	resp.Server.Domain = h.cfg.Server.Domain
	resp.Server.ShutdownTimeoutSecs = h.cfg.Server.ShutdownTimeoutSecs
	resp.Server.TLSEnabled = h.cfg.Server.TLS.Enabled

	resp.Storage.DataDir = h.cfg.Storage.DataDir
	resp.Storage.MetadataDir = h.cfg.Storage.MetadataDir

	resp.Features.Encryption = h.cfg.Encryption.Enabled
	resp.Features.Compression = h.cfg.Compression.Enabled
	resp.Features.AccessLog = h.cfg.Logging.Enabled
	resp.Features.RateLimit = h.cfg.RateLimit.Enabled
	resp.Features.Replication = h.cfg.Replication.Enabled
	resp.Features.Scanner = h.cfg.Scanner.Enabled
	resp.Features.Tiering = h.cfg.Tiering.Enabled
	resp.Features.Backup = h.cfg.Backup.Enabled
	resp.Features.OIDC = h.cfg.OIDC.Enabled
	resp.Features.Lambda = h.cfg.Lambda.Enabled
	resp.Features.Debug = h.cfg.Debug

	resp.Lifecycle.ScanIntervalSecs = h.cfg.Lifecycle.ScanIntervalSecs
	resp.Lifecycle.AuditRetentionDays = h.cfg.Security.AuditRetentionDays

	if h.cfg.RateLimit.Enabled {
		resp.RateLimit.RequestsPerSec = h.cfg.RateLimit.RequestsPerSec
		resp.RateLimit.BurstSize = h.cfg.RateLimit.BurstSize
		resp.RateLimit.PerKeyRPS = h.cfg.RateLimit.PerKeyRPS
		resp.RateLimit.PerKeyBurst = h.cfg.RateLimit.PerKeyBurst
	}

	resp.Memory.MaxSearchEntries = h.cfg.Memory.MaxSearchEntries
	resp.Memory.GoMemLimitMB = h.cfg.Memory.GoMemLimitMB

	writeJSON(w, http.StatusOK, resp)
}

type changeCredentialsRequest struct {
	CurrentSecretKey string `json:"currentSecretKey"`
	NewAccessKey     string `json:"newAccessKey"`
	NewSecretKey     string `json:"newSecretKey"`
}

func (h *APIHandler) handleChangeCredentials(w http.ResponseWriter, r *http.Request) {
	var req changeCredentialsRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.NewAccessKey == "" || req.NewSecretKey == "" {
		writeError(w, http.StatusBadRequest, "new access key and secret key are required")
		return
	}

	if len(req.NewSecretKey) < 8 {
		writeError(w, http.StatusBadRequest, "secret key must be at least 8 characters")
		return
	}

	// Verify current secret key
	if !hmac.Equal([]byte(req.CurrentSecretKey), []byte(h.cfg.Auth.AdminSecretKey)) {
		writeError(w, http.StatusForbidden, "current secret key is incorrect")
		return
	}

	// Update in-memory config
	h.cfg.Auth.AdminAccessKey = req.NewAccessKey
	h.cfg.Auth.AdminSecretKey = req.NewSecretKey

	// Update S3 authenticator so SigV4 auth uses new credentials
	if h.s3Auth != nil {
		h.s3Auth.UpdateAdminCredentials(req.NewAccessKey, req.NewSecretKey)
	}

	// Re-initialize JWT service with new secret key
	h.jwt = NewJWTService(req.NewSecretKey)

	// Persist to metadata store
	if h.store != nil {
		_ = h.store.SetAdminCredentials(req.NewAccessKey, req.NewSecretKey)
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "credentials updated successfully"})
}
