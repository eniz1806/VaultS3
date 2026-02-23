package config

import (
	"encoding/hex"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server      ServerConfig      `yaml:"server"`
	Storage     StorageConfig     `yaml:"storage"`
	Auth        AuthConfig        `yaml:"auth"`
	Encryption  EncryptionConfig  `yaml:"encryption"`
	Compression CompressionConfig `yaml:"compression"`
	Logging     LoggingConfig     `yaml:"logging"`
	Lifecycle   LifecycleConfig   `yaml:"lifecycle"`
	Security    SecurityConfig    `yaml:"security"`
}

type SecurityConfig struct {
	IPAllowlist        []string `yaml:"ip_allowlist"`
	IPBlocklist        []string `yaml:"ip_blocklist"`
	AuditRetentionDays int      `yaml:"audit_retention_days"`
	STSMaxDurationSecs int      `yaml:"sts_max_duration_secs"`
}

type ServerConfig struct {
	Address             string    `yaml:"address"`
	Port                int       `yaml:"port"`
	Domain              string    `yaml:"domain"` // base domain for virtual-hosted URLs (e.g. "localhost", "s3.example.com")
	ShutdownTimeoutSecs int       `yaml:"shutdown_timeout_secs"`
	TLS                 TLSConfig `yaml:"tls"`
}

type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

type StorageConfig struct {
	DataDir     string `yaml:"data_dir"`
	MetadataDir string `yaml:"metadata_dir"`
}

type AuthConfig struct {
	AdminAccessKey string `yaml:"admin_access_key"`
	AdminSecretKey string `yaml:"admin_secret_key"`
}

type EncryptionConfig struct {
	Enabled bool   `yaml:"enabled"`
	Key     string `yaml:"key"` // hex-encoded 32-byte key (64 hex chars)
}

type CompressionConfig struct {
	Enabled bool `yaml:"enabled"`
}

type LoggingConfig struct {
	Enabled  bool   `yaml:"enabled"`
	FilePath string `yaml:"file_path"`
}

type LifecycleConfig struct {
	ScanIntervalSecs int `yaml:"scan_interval_secs"`
}

// KeyBytes returns the decoded encryption key bytes.
func (e *EncryptionConfig) KeyBytes() ([]byte, error) {
	if !e.Enabled {
		return nil, nil
	}
	key, err := hex.DecodeString(e.Key)
	if err != nil {
		return nil, fmt.Errorf("encryption key must be hex-encoded: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes (64 hex chars), got %d bytes", len(key))
	}
	return key, nil
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := &Config{
		Server: ServerConfig{
			Address:             "0.0.0.0",
			Port:                9000,
			ShutdownTimeoutSecs: 30,
		},
		Storage: StorageConfig{
			DataDir:     "./data",
			MetadataDir: "./metadata",
		},
		Logging: LoggingConfig{
			FilePath: "./access.log",
		},
		Lifecycle: LifecycleConfig{
			ScanIntervalSecs: 3600,
		},
		Security: SecurityConfig{
			AuditRetentionDays: 90,
			STSMaxDurationSecs: 43200,
		},
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Validate encryption config
	if cfg.Encryption.Enabled {
		if _, err := cfg.Encryption.KeyBytes(); err != nil {
			return nil, fmt.Errorf("invalid encryption config: %w", err)
		}
	}

	return cfg, nil
}

func (c *Config) ListenAddr() string {
	return fmt.Sprintf("%s:%d", c.Server.Address, c.Server.Port)
}
