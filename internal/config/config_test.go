package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return p
}

func TestLoad_Defaults(t *testing.T) {
	p := writeConfig(t, "server:\n  port: 8080\n")
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("port: got %d, want 8080", cfg.Server.Port)
	}
	if cfg.Server.Address != "0.0.0.0" {
		t.Errorf("address: got %q, want 0.0.0.0", cfg.Server.Address)
	}
	if cfg.Storage.DataDir != "./data" {
		t.Errorf("data_dir: got %q, want ./data", cfg.Storage.DataDir)
	}
	if cfg.Storage.MetadataDir != "./metadata" {
		t.Errorf("metadata_dir: got %q, want ./metadata", cfg.Storage.MetadataDir)
	}
	if cfg.Lifecycle.ScanIntervalSecs != 3600 {
		t.Errorf("lifecycle scan interval: got %d, want 3600", cfg.Lifecycle.ScanIntervalSecs)
	}
	if cfg.Security.AuditRetentionDays != 90 {
		t.Errorf("audit retention: got %d, want 90", cfg.Security.AuditRetentionDays)
	}
	if cfg.Server.ShutdownTimeoutSecs != 30 {
		t.Errorf("shutdown timeout: got %d, want 30", cfg.Server.ShutdownTimeoutSecs)
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	p := writeConfig(t, "")
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Should get all defaults
	if cfg.Server.Port != 9000 {
		t.Errorf("default port: got %d, want 9000", cfg.Server.Port)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	p := writeConfig(t, "{{invalid yaml}}")
	_, err := Load(p)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoad_NonexistentFile(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoad_EncryptionValid(t *testing.T) {
	// 32 bytes = 64 hex chars
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	p := writeConfig(t, "encryption:\n  enabled: true\n  key: "+key+"\n")
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Encryption.Enabled {
		t.Error("encryption should be enabled")
	}
}

func TestLoad_EncryptionInvalidKey(t *testing.T) {
	p := writeConfig(t, "encryption:\n  enabled: true\n  key: tooshort\n")
	_, err := Load(p)
	if err == nil {
		t.Error("expected error for invalid encryption key")
	}
}

func TestLoad_EncryptionWrongLength(t *testing.T) {
	// 16 bytes = 32 hex chars (too short)
	key := "0123456789abcdef0123456789abcdef"
	p := writeConfig(t, "encryption:\n  enabled: true\n  key: "+key+"\n")
	_, err := Load(p)
	if err == nil {
		t.Error("expected error for wrong key length")
	}
}

func TestLoad_EncryptionDisabled(t *testing.T) {
	p := writeConfig(t, "encryption:\n  enabled: false\n  key: invalid\n")
	_, err := Load(p)
	if err != nil {
		t.Fatalf("Load with disabled encryption should not validate key: %v", err)
	}
}

func TestKeyBytes_Disabled(t *testing.T) {
	e := EncryptionConfig{Enabled: false}
	key, err := e.KeyBytes()
	if err != nil {
		t.Fatalf("KeyBytes: %v", err)
	}
	if key != nil {
		t.Error("expected nil key when disabled")
	}
}

func TestKeyBytes_InvalidHex(t *testing.T) {
	e := EncryptionConfig{Enabled: true, Key: "zzzz"}
	_, err := e.KeyBytes()
	if err == nil {
		t.Error("expected error for invalid hex")
	}
}

func TestListenAddr(t *testing.T) {
	cfg := Config{Server: ServerConfig{Address: "127.0.0.1", Port: 8080}}
	if got := cfg.ListenAddr(); got != "127.0.0.1:8080" {
		t.Errorf("ListenAddr: got %q, want 127.0.0.1:8080", got)
	}
}

func TestLoad_OverrideDefaults(t *testing.T) {
	yaml := `
server:
  address: "192.168.1.1"
  port: 3000
storage:
  data_dir: "/custom/data"
  metadata_dir: "/custom/meta"
auth:
  admin_access_key: "mykey"
  admin_secret_key: "mysecret"
compression:
  enabled: true
`
	p := writeConfig(t, yaml)
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Address != "192.168.1.1" {
		t.Errorf("address: got %q", cfg.Server.Address)
	}
	if cfg.Server.Port != 3000 {
		t.Errorf("port: got %d", cfg.Server.Port)
	}
	if cfg.Storage.DataDir != "/custom/data" {
		t.Errorf("data_dir: got %q", cfg.Storage.DataDir)
	}
	if cfg.Auth.AdminAccessKey != "mykey" {
		t.Errorf("access key: got %q", cfg.Auth.AdminAccessKey)
	}
	if !cfg.Compression.Enabled {
		t.Error("compression should be enabled")
	}
}
