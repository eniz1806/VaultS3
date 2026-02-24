package config

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server        ServerConfig        `yaml:"server"`
	Storage       StorageConfig       `yaml:"storage"`
	Auth          AuthConfig          `yaml:"auth"`
	Encryption    EncryptionConfig    `yaml:"encryption"`
	Compression   CompressionConfig   `yaml:"compression"`
	Logging       LoggingConfig       `yaml:"logging"`
	Lifecycle     LifecycleConfig     `yaml:"lifecycle"`
	Security      SecurityConfig      `yaml:"security"`
	Notifications NotificationsConfig `yaml:"notifications"`
	Replication   ReplicationConfig   `yaml:"replication"`
	Scanner       ScannerConfig       `yaml:"scanner"`
	Tiering       TieringConfig       `yaml:"tiering"`
	Backup        BackupConfig        `yaml:"backup"`
	RateLimit     RateLimitConfig     `yaml:"rate_limit"`
	OIDC          OIDCConfig          `yaml:"oidc"`
	Lambda        LambdaConfig        `yaml:"lambda"`
	Memory        MemoryConfig        `yaml:"memory"`
	Debug         bool                `yaml:"debug"`
}

type MemoryConfig struct {
	MaxSearchEntries int `yaml:"max_search_entries"`
	GoMemLimitMB     int `yaml:"go_mem_limit_mb"`
}

type OIDCConfig struct {
	Enabled         bool              `yaml:"enabled"`
	IssuerURL       string            `yaml:"issuer_url"`
	ClientID        string            `yaml:"client_id"`
	AllowedDomains  []string          `yaml:"allowed_domains"`
	RoleMapping     map[string]string `yaml:"role_mapping"`
	AutoCreateUsers bool              `yaml:"auto_create_users"`
	JWKSCacheSecs   int               `yaml:"jwks_cache_secs"`
}

type LambdaConfig struct {
	Enabled         bool  `yaml:"enabled"`
	MaxResponseSize int64 `yaml:"max_response_size"`
	TimeoutSecs     int   `yaml:"timeout_secs"`
	MaxWorkers      int   `yaml:"max_workers"`
	QueueSize       int   `yaml:"queue_size"`
}

type RateLimitConfig struct {
	Enabled        bool    `yaml:"enabled"`
	RequestsPerSec float64 `yaml:"requests_per_sec"`
	BurstSize      int     `yaml:"burst_size"`
	PerKeyRPS      float64 `yaml:"per_key_rps"`
	PerKeyBurst    int     `yaml:"per_key_burst"`
}

type TieringConfig struct {
	Enabled          bool   `yaml:"enabled"`
	ColdDataDir      string `yaml:"cold_data_dir"`
	MigrateAfterDays int    `yaml:"migrate_after_days"`
	ScanIntervalSecs int    `yaml:"scan_interval_secs"`
}

type BackupTarget struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"` // "local" or "s3"
	Path        string `yaml:"path"`
	S3Endpoint  string `yaml:"s3_endpoint"`
	S3AccessKey string `yaml:"s3_access_key"`
	S3SecretKey string `yaml:"s3_secret_key"`
	S3Bucket    string `yaml:"s3_bucket"`
}

type BackupConfig struct {
	Enabled       bool           `yaml:"enabled"`
	Targets       []BackupTarget `yaml:"targets"`
	ScheduleCron  string         `yaml:"schedule_cron"`
	RetentionDays int            `yaml:"retention_days"`
	Incremental   bool           `yaml:"incremental"`
}

type ScannerConfig struct {
	Enabled          bool   `yaml:"enabled"`
	WebhookURL       string `yaml:"webhook_url"`
	TimeoutSecs      int    `yaml:"timeout_secs"`
	QuarantineBucket string `yaml:"quarantine_bucket"`
	FailClosed       bool   `yaml:"fail_closed"`
	MaxScanSizeBytes int64  `yaml:"max_scan_size_bytes"`
	Workers          int    `yaml:"workers"`
}

type ReplicationPeer struct {
	Name      string `yaml:"name"`
	URL       string `yaml:"url"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
}

type ReplicationConfig struct {
	Enabled          bool              `yaml:"enabled"`
	Peers            []ReplicationPeer `yaml:"peers"`
	ScanIntervalSecs int               `yaml:"scan_interval_secs"`
	MaxRetries       int               `yaml:"max_retries"`
	BatchSize        int               `yaml:"batch_size"`
}

type NotificationsConfig struct {
	MaxWorkers  int               `yaml:"max_workers"`
	QueueSize   int               `yaml:"queue_size"`
	TimeoutSecs int               `yaml:"timeout_secs"`
	MaxRetries  int               `yaml:"max_retries"`
	Kafka       KafkaNotifyConfig `yaml:"kafka"`
	NATS        NATSNotifyConfig  `yaml:"nats"`
	Redis       RedisNotifyConfig `yaml:"redis"`
}

type KafkaNotifyConfig struct {
	Enabled bool     `yaml:"enabled"`
	Brokers []string `yaml:"brokers"`
	Topic   string   `yaml:"topic"`
}

type NATSNotifyConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
	Subject string `yaml:"subject"`
}

type RedisNotifyConfig struct {
	Enabled bool   `yaml:"enabled"`
	Addr    string `yaml:"addr"`
	Channel string `yaml:"channel"`
	ListKey string `yaml:"list_key"`
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
	Level    string `yaml:"level"` // debug, info, warn, error (default: info)
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
		Notifications: NotificationsConfig{
			MaxWorkers:  4,
			QueueSize:   256,
			TimeoutSecs: 10,
			MaxRetries:  3,
		},
		Replication: ReplicationConfig{
			ScanIntervalSecs: 30,
			MaxRetries:       5,
			BatchSize:        100,
		},
		Scanner: ScannerConfig{
			TimeoutSecs:      30,
			QuarantineBucket: "vaults3-quarantine",
			MaxScanSizeBytes: 104857600, // 100MB
			Workers:          2,
		},
		Tiering: TieringConfig{
			MigrateAfterDays: 30,
			ScanIntervalSecs: 3600,
		},
		Backup: BackupConfig{
			ScheduleCron:  "0 2 * * *",
			RetentionDays: 30,
		},
		RateLimit: RateLimitConfig{
			RequestsPerSec: 100,
			BurstSize:      200,
			PerKeyRPS:      50,
			PerKeyBurst:    100,
		},
		OIDC: OIDCConfig{
			JWKSCacheSecs: 3600,
		},
		Lambda: LambdaConfig{
			MaxResponseSize: 10 * 1024 * 1024, // 10MB
			TimeoutSecs:     30,
			MaxWorkers:      4,
			QueueSize:       256,
		},
		Memory: MemoryConfig{
			MaxSearchEntries: 50000,
		},
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Apply environment variable overrides
	applyEnvOverrides(cfg)

	// Validate encryption config
	if cfg.Encryption.Enabled {
		if _, err := cfg.Encryption.KeyBytes(); err != nil {
			return nil, fmt.Errorf("invalid encryption config: %w", err)
		}
	}

	return cfg, nil
}

// applyEnvOverrides applies environment variable overrides to the config.
// Environment variables take precedence over YAML config values.
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("VAULTS3_ACCESS_KEY"); v != "" {
		cfg.Auth.AdminAccessKey = v
	}
	if v := os.Getenv("VAULTS3_SECRET_KEY"); v != "" {
		cfg.Auth.AdminSecretKey = v
	}
	if v := os.Getenv("VAULTS3_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = p
		}
	}
	if v := os.Getenv("VAULTS3_ADDRESS"); v != "" {
		cfg.Server.Address = v
	}
	if v := os.Getenv("VAULTS3_DOMAIN"); v != "" {
		cfg.Server.Domain = v
	}
	if v := os.Getenv("VAULTS3_DATA_DIR"); v != "" {
		cfg.Storage.DataDir = v
	}
	if v := os.Getenv("VAULTS3_METADATA_DIR"); v != "" {
		cfg.Storage.MetadataDir = v
	}
	if v := os.Getenv("VAULTS3_ENCRYPTION_KEY"); v != "" {
		cfg.Encryption.Enabled = true
		cfg.Encryption.Key = v
	}
	if v := os.Getenv("VAULTS3_TLS_CERT"); v != "" {
		cfg.Server.TLS.CertFile = v
	}
	if v := os.Getenv("VAULTS3_TLS_KEY"); v != "" {
		cfg.Server.TLS.KeyFile = v
	}
	if os.Getenv("VAULTS3_TLS_CERT") != "" && os.Getenv("VAULTS3_TLS_KEY") != "" {
		cfg.Server.TLS.Enabled = true
	}
	if v := os.Getenv("VAULTS3_LOG_LEVEL"); v != "" {
		cfg.Logging.Level = v
	}
}

func (c *Config) ListenAddr() string {
	return fmt.Sprintf("%s:%d", c.Server.Address, c.Server.Port)
}
