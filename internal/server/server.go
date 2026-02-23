package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/eniz1806/VaultS3/internal/accesslog"
	"github.com/eniz1806/VaultS3/internal/api"
	"github.com/eniz1806/VaultS3/internal/config"
	"github.com/eniz1806/VaultS3/internal/dashboard"
	"github.com/eniz1806/VaultS3/internal/lifecycle"
	"github.com/eniz1806/VaultS3/internal/metadata"
	"github.com/eniz1806/VaultS3/internal/metrics"
	"github.com/eniz1806/VaultS3/internal/s3"
	"github.com/eniz1806/VaultS3/internal/storage"
)

type Server struct {
	cfg       *config.Config
	store     *metadata.Store
	engine    storage.Engine
	s3h       *s3.Handler
	metrics   *metrics.Collector
	activity  *api.ActivityLog
	accessLog *accesslog.AccessLogger
}

func New(cfg *config.Config) (*Server, error) {
	// Initialize storage engine
	fs, err := storage.NewFileSystem(cfg.Storage.DataDir)
	if err != nil {
		return nil, fmt.Errorf("init storage: %w", err)
	}

	var engine storage.Engine = fs

	// Wrap with compression if enabled (compress before encrypt)
	if cfg.Compression.Enabled {
		engine = storage.NewCompressedEngine(engine)
		log.Println("Compression: enabled (gzip)")
	}

	// Wrap with encryption if enabled
	if cfg.Encryption.Enabled {
		keyBytes, err := cfg.Encryption.KeyBytes()
		if err != nil {
			return nil, fmt.Errorf("encryption config: %w", err)
		}
		enc, err := storage.NewEncryptedEngine(engine, keyBytes)
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

	// Initialize activity log
	activityLog := api.NewActivityLog()

	// Initialize S3 handler
	s3h := s3.NewHandler(store, engine, auth, cfg.Encryption.Enabled, cfg.Server.Domain, mc)

	// Initialize access logger if enabled
	var accessLogger *accesslog.AccessLogger
	if cfg.Logging.Enabled {
		var err error
		accessLogger, err = accesslog.NewAccessLogger(cfg.Logging.FilePath)
		if err != nil {
			store.Close()
			return nil, fmt.Errorf("init access logger: %w", err)
		}
		log.Printf("Access logging: enabled (%s)", cfg.Logging.FilePath)
	}

	// Wire activity recording from S3 handler to activity log + access logger
	s3h.SetActivityFunc(func(method, bucket, key string, status int, size int64, clientIP string) {
		now := time.Now().UTC()
		activityLog.Record(api.ActivityEntry{
			Time:     now,
			Method:   method,
			Bucket:   bucket,
			Key:      key,
			Status:   status,
			Size:     size,
			ClientIP: clientIP,
		})
		if accessLogger != nil {
			accessLogger.Log(accesslog.AccessEntry{
				Time:     now,
				Method:   method,
				Bucket:   bucket,
				Key:      key,
				Status:   status,
				Bytes:    size,
				ClientIP: clientIP,
			})
		}
	})

	return &Server{
		cfg:       cfg,
		store:     store,
		engine:    engine,
		s3h:       s3h,
		metrics:   mc,
		activity:  activityLog,
		accessLog: accessLogger,
	}, nil
}

// Run starts the server and blocks until shutdown signal is received.
// It handles graceful shutdown with a configurable timeout.
func (s *Server) Run() error {
	addr := s.cfg.ListenAddr()

	// Dashboard API
	apiHandler := api.NewAPIHandler(s.store, s.engine, s.metrics, s.cfg, s.activity)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler(s.metrics.StartTime()))
	mux.HandleFunc("/ready", readyHandler(s.store))
	mux.Handle("/api/v1/", apiHandler)
	mux.Handle("/dashboard/", dashboard.Handler())
	mux.Handle("/metrics", s.metrics)
	mux.Handle("/", s.s3h)

	httpServer := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Log startup info
	scheme := "http"
	if s.cfg.Server.TLS.Enabled {
		scheme = "https"
	}
	log.Printf("VaultS3 starting on %s", addr)
	log.Printf("  Data dir:     %s", s.cfg.Storage.DataDir)
	log.Printf("  Metadata dir: %s", s.cfg.Storage.MetadataDir)
	log.Printf("  Access key:   %s", s.cfg.Auth.AdminAccessKey)
	log.Printf("  Dashboard:    %s://%s/dashboard/", scheme, addr)
	log.Printf("  Health:       %s://%s/health", scheme, addr)
	if s.cfg.Encryption.Enabled {
		log.Printf("  Encryption:   AES-256-GCM")
	}
	if s.cfg.Server.Domain != "" {
		log.Printf("  Domain:       %s (virtual-hosted URLs enabled)", s.cfg.Server.Domain)
	}
	if s.cfg.Server.TLS.Enabled {
		log.Printf("  TLS:          enabled (%s, %s)", s.cfg.Server.TLS.CertFile, s.cfg.Server.TLS.KeyFile)
	}

	// Start lifecycle worker
	lcCtx, lcCancel := context.WithCancel(context.Background())
	defer lcCancel()
	lcWorker := lifecycle.NewWorker(s.store, s.engine, s.cfg.Lifecycle.ScanIntervalSecs)
	go lcWorker.Run(lcCtx)
	log.Printf("  Lifecycle:    scan every %ds", s.cfg.Lifecycle.ScanIntervalSecs)

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		if s.cfg.Server.TLS.Enabled {
			errCh <- httpServer.ListenAndServeTLS(s.cfg.Server.TLS.CertFile, s.cfg.Server.TLS.KeyFile)
		} else {
			errCh <- httpServer.ListenAndServe()
		}
	}()

	// Wait for signal or server error
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case sig := <-sigCh:
		log.Printf("Received %v, shutting down gracefully...", sig)
	}

	// Graceful shutdown
	timeout := time.Duration(s.cfg.Server.ShutdownTimeoutSecs) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("Graceful shutdown timed out after %v: %v", timeout, err)
		return err
	}

	log.Println("Server stopped gracefully")
	return nil
}

func (s *Server) Close() {
	if s.accessLog != nil {
		s.accessLog.Close()
	}
	if s.store != nil {
		s.store.Close()
	}
}
