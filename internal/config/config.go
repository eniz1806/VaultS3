package config

import (
	"encoding/hex"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Storage    StorageConfig    `yaml:"storage"`
	Auth       AuthConfig       `yaml:"auth"`
	Encryption EncryptionConfig `yaml:"encryption"`
}

type ServerConfig struct {
	Address string `yaml:"address"`
	Port    int    `yaml:"port"`
	Domain  string `yaml:"domain"` // base domain for virtual-hosted URLs (e.g. "localhost", "s3.example.com")
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
			Address: "0.0.0.0",
			Port:    9000,
		},
		Storage: StorageConfig{
			DataDir:     "./data",
			MetadataDir: "./metadata",
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
