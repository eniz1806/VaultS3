package server

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/eniz1806/VaultS3/internal/api"
	"github.com/eniz1806/VaultS3/internal/config"
	"github.com/eniz1806/VaultS3/internal/dashboard"
	"github.com/eniz1806/VaultS3/internal/metadata"
	"github.com/eniz1806/VaultS3/internal/metrics"
	"github.com/eniz1806/VaultS3/internal/s3"
	"github.com/eniz1806/VaultS3/internal/storage"
)

type Server struct {
	cfg     *config.Config
	store   *metadata.Store
	engine  storage.Engine
	s3h     *s3.Handler
	metrics *metrics.Collector
}

func New(cfg *config.Config) (*Server, error) {
	// Initialize storage engine
	fs, err := storage.NewFileSystem(cfg.Storage.DataDir)
	if err != nil {
		return nil, fmt.Errorf("init storage: %w", err)
	}

	var engine storage.Engine = fs

	// Wrap with encryption if enabled
	if cfg.Encryption.Enabled {
		keyBytes, err := cfg.Encryption.KeyBytes()
		if err != nil {
			return nil, fmt.Errorf("encryption config: %w", err)
		}
		enc, err := storage.NewEncryptedEngine(fs, keyBytes)
		if err != nil {
			return nil, fmt.Errorf("init encryption: %w", err)
		}
		engine = enc
		log.Println("Encryption at rest: enabled (AES-256-GCM)")
	}

	// Initialize metadata store
	metaDir := cfg.Storage.MetadataDir
	if err := os.MkdirAll(metaDir, 0755); err != nil {
		return nil, fmt.Errorf("create metadata dir: %w", err)
	}
	store, err := metadata.NewStore(filepath.Join(metaDir, "vaults3.db"))
	if err != nil {
		return nil, fmt.Errorf("init metadata: %w", err)
	}

	// Initialize S3 authenticator
	auth := s3.NewAuthenticator(cfg.Auth.AdminAccessKey, cfg.Auth.AdminSecretKey, store)

	// Initialize metrics collector
	mc := metrics.NewCollector(store, engine)

	// Initialize S3 handler
	s3h := s3.NewHandler(store, engine, auth, cfg.Encryption.Enabled, cfg.Server.Domain, mc)

	return &Server{
		cfg:     cfg,
		store:   store,
		engine:  engine,
		s3h:     s3h,
		metrics: mc,
	}, nil
}

func (s *Server) Start() error {
	addr := s.cfg.ListenAddr()

	// Dashboard API
	apiHandler := api.NewAPIHandler(s.store, s.engine, s.metrics, s.cfg)

	mux := http.NewServeMux()
	mux.Handle("/api/v1/", apiHandler)
	mux.Handle("/dashboard/", dashboard.Handler())
	mux.Handle("/metrics", s.metrics)
	mux.Handle("/", s.s3h)

	log.Printf("VaultS3 starting on %s", addr)
	log.Printf("  Data dir:     %s", s.cfg.Storage.DataDir)
	log.Printf("  Metadata dir: %s", s.cfg.Storage.MetadataDir)
	log.Printf("  Access key:   %s", s.cfg.Auth.AdminAccessKey)
	log.Printf("  Dashboard:    http://%s/dashboard/", addr)
	if s.cfg.Encryption.Enabled {
		log.Printf("  Encryption:   AES-256-GCM")
	}
	if s.cfg.Server.Domain != "" {
		log.Printf("  Domain:       %s (virtual-hosted URLs enabled)", s.cfg.Server.Domain)
	}

	return http.ListenAndServe(addr, mux)
}

func (s *Server) Close() {
	if s.store != nil {
		s.store.Close()
	}
}
