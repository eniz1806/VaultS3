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
	"github.com/eniz1806/VaultS3/internal/backup"
	"github.com/eniz1806/VaultS3/internal/config"
	"github.com/eniz1806/VaultS3/internal/dashboard"
	"github.com/eniz1806/VaultS3/internal/lifecycle"
	"github.com/eniz1806/VaultS3/internal/metadata"
	"github.com/eniz1806/VaultS3/internal/metrics"
	"github.com/eniz1806/VaultS3/internal/notify"
	"github.com/eniz1806/VaultS3/internal/ratelimit"
	"github.com/eniz1806/VaultS3/internal/replication"
	"github.com/eniz1806/VaultS3/internal/s3"
	"github.com/eniz1806/VaultS3/internal/scanner"
	"github.com/eniz1806/VaultS3/internal/search"
	"github.com/eniz1806/VaultS3/internal/storage"
	"github.com/eniz1806/VaultS3/internal/tiering"
)

type Server struct {
	cfg       *config.Config
	store     *metadata.Store
	engine    storage.Engine
	s3h       *s3.Handler
	metrics   *metrics.Collector
	activity  *api.ActivityLog
	accessLog    *accesslog.AccessLogger
	notifyDisp   *notify.Dispatcher
	replWorker   *replication.Worker
	searchIndex  *search.Index
	scanWorker   *scanner.Scanner
	tieringMgr   *tiering.Manager
	backupSched  *backup.Scheduler
	rateLimiter  *ratelimit.Limiter
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
	auth := s3.NewAuthenticator(cfg.Auth.AdminAccessKey, cfg.Auth.AdminSecretKey, store,
		cfg.Security.IPAllowlist, cfg.Security.IPBlocklist)

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

	// Wire audit trail recording
	s3h.SetAuditFunc(func(principal, userID, action, resource, effect, sourceIP string, statusCode int) {
		store.PutAuditEntry(metadata.AuditEntry{
			Time:       time.Now().UnixNano(),
			Principal:  principal,
			UserID:     userID,
			Action:     action,
			Resource:   resource,
			Effect:     effect,
			SourceIP:   sourceIP,
			StatusCode: statusCode,
		})
	})

	// Initialize notification dispatcher
	nc := cfg.Notifications
	notifyDispatcher := notify.NewDispatcher(store, nc.MaxWorkers, nc.QueueSize, nc.TimeoutSecs, nc.MaxRetries)

	// Register notification backends
	if nc.Kafka.Enabled && len(nc.Kafka.Brokers) > 0 && nc.Kafka.Topic != "" {
		notifyDispatcher.AddBackend(notify.NewKafkaBackend(nc.Kafka.Brokers, nc.Kafka.Topic))
	}
	if nc.NATS.Enabled && nc.NATS.URL != "" && nc.NATS.Subject != "" {
		natsBackend, err := notify.NewNATSBackend(nc.NATS.URL, nc.NATS.Subject)
		if err != nil {
			log.Printf("Warning: NATS backend failed to connect: %v", err)
		} else {
			notifyDispatcher.AddBackend(natsBackend)
		}
	}
	if nc.Redis.Enabled && nc.Redis.Addr != "" {
		notifyDispatcher.AddBackend(notify.NewRedisBackend(nc.Redis.Addr, nc.Redis.Channel, nc.Redis.ListKey))
	}

	s3h.SetNotificationFunc(func(eventType, bucket, key string, size int64, etag, versionID string) {
		notifyDispatcher.Dispatch(bucket, key, eventType, size, etag, versionID)
	})

	// Initialize replication worker if enabled
	var replWorker *replication.Worker
	if cfg.Replication.Enabled && len(cfg.Replication.Peers) > 0 {
		replWorker = replication.NewWorker(store, engine, cfg.Replication)
		s3h.SetReplicationFunc(func(eventType, bucket, key string, size int64, etag, versionID string) {
			evtType := "put"
			if eventType == "s3:ObjectRemoved:Delete" {
				evtType = "delete"
			}
			for _, peer := range cfg.Replication.Peers {
				store.EnqueueReplication(metadata.ReplicationEvent{
					Type:   evtType,
					Bucket: bucket,
					Key:    key,
					ETag:   etag,
					Peer:   peer.Name,
					Size:   size,
				})
			}
		})
		log.Printf("  Replication:  enabled (%d peers, scan every %ds)", len(cfg.Replication.Peers), cfg.Replication.ScanIntervalSecs)
	}

	// Build search index
	searchIdx := search.NewIndex(store)
	if err := searchIdx.Build(); err != nil {
		log.Printf("Warning: search index build failed: %v", err)
	}
	s3h.SetSearchUpdateFunc(func(eventType, bucket, key string) {
		if eventType == "delete" {
			searchIdx.Remove(bucket, key)
		} else {
			if meta, err := store.GetObjectMeta(bucket, key); err == nil {
				searchIdx.Update(bucket, key, *meta)
			}
		}
	})

	// Initialize scanner if enabled
	var scanWorker *scanner.Scanner
	if cfg.Scanner.Enabled && cfg.Scanner.WebhookURL != "" {
		scanWorker = scanner.NewScanner(store, engine,
			cfg.Scanner.WebhookURL, cfg.Scanner.Workers,
			cfg.Scanner.TimeoutSecs, cfg.Scanner.QuarantineBucket,
			cfg.Scanner.FailClosed, cfg.Scanner.MaxScanSizeBytes, 256)
		s3h.SetScanFunc(func(bucket, key string, size int64) {
			scanWorker.Scan(bucket, key, size)
		})
	}

	// Initialize tiering if enabled
	var tieringMgr *tiering.Manager
	if cfg.Tiering.Enabled && cfg.Tiering.ColdDataDir != "" {
		coldFS, err := storage.NewFileSystem(cfg.Tiering.ColdDataDir)
		if err != nil {
			store.Close()
			return nil, fmt.Errorf("init cold storage: %w", err)
		}
		tieringMgr = tiering.NewManager(store, fs, coldFS, cfg.Tiering.MigrateAfterDays, cfg.Tiering.ScanIntervalSecs)
		log.Printf("  Tiering:      enabled (cold=%s, migrate after %dd)", cfg.Tiering.ColdDataDir, cfg.Tiering.MigrateAfterDays)
	}

	// Initialize backup scheduler if enabled
	var backupSched *backup.Scheduler
	if cfg.Backup.Enabled && len(cfg.Backup.Targets) > 0 {
		backupSched = backup.NewScheduler(store, engine, cfg.Backup)
		log.Printf("  Backup:       enabled (%d targets, schedule=%s)", len(cfg.Backup.Targets), cfg.Backup.ScheduleCron)
	}

	// Initialize rate limiter if enabled
	var rateLimiter *ratelimit.Limiter
	if cfg.RateLimit.Enabled {
		rateLimiter = ratelimit.NewLimiter(
			cfg.RateLimit.RequestsPerSec, cfg.RateLimit.BurstSize,
			cfg.RateLimit.PerKeyRPS, cfg.RateLimit.PerKeyBurst,
		)
		s3h.SetRateLimiter(rateLimiter)
		log.Printf("  Rate limit:   enabled (IP: %.0f rps/%d burst, Key: %.0f rps/%d burst)",
			cfg.RateLimit.RequestsPerSec, cfg.RateLimit.BurstSize,
			cfg.RateLimit.PerKeyRPS, cfg.RateLimit.PerKeyBurst)
	}

	// Initialize built-in IAM policies
	initBuiltinPolicies(store)

	return &Server{
		cfg:         cfg,
		store:       store,
		engine:      engine,
		s3h:         s3h,
		metrics:     mc,
		activity:    activityLog,
		accessLog:   accessLogger,
		notifyDisp:  notifyDispatcher,
		replWorker:  replWorker,
		searchIndex: searchIdx,
		scanWorker:  scanWorker,
		tieringMgr:  tieringMgr,
		backupSched: backupSched,
		rateLimiter: rateLimiter,
	}, nil
}

// Run starts the server and blocks until shutdown signal is received.
// It handles graceful shutdown with a configurable timeout.
func (s *Server) Run() error {
	addr := s.cfg.ListenAddr()

	// Dashboard API
	apiHandler := api.NewAPIHandler(s.store, s.engine, s.metrics, s.cfg, s.activity)
	apiHandler.SetSearchIndex(s.searchIndex)
	if s.scanWorker != nil {
		apiHandler.SetScanner(s.scanWorker)
	}
	if s.tieringMgr != nil {
		apiHandler.SetTieringManager(s.tieringMgr)
	}
	if s.backupSched != nil {
		apiHandler.SetBackupScheduler(s.backupSched)
	}
	if s.rateLimiter != nil {
		apiHandler.SetRateLimiter(s.rateLimiter)
	}

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
	lcWorker := lifecycle.NewWorker(s.store, s.engine, s.cfg.Lifecycle.ScanIntervalSecs, s.cfg.Security.AuditRetentionDays)
	go lcWorker.Run(lcCtx)
	log.Printf("  Lifecycle:    scan every %ds", s.cfg.Lifecycle.ScanIntervalSecs)

	// Start notification dispatcher
	notifyCtx, notifyCancel := context.WithCancel(context.Background())
	defer notifyCancel()
	s.notifyDisp.Start(notifyCtx)
	log.Printf("  Notifications: %d workers, queue size %d", s.cfg.Notifications.MaxWorkers, s.cfg.Notifications.QueueSize)

	// Start replication worker if enabled
	if s.replWorker != nil {
		replCtx, replCancel := context.WithCancel(context.Background())
		defer replCancel()
		go s.replWorker.Run(replCtx)
	}

	// Start scanner workers if enabled
	if s.scanWorker != nil {
		scanCtx, scanCancel := context.WithCancel(context.Background())
		defer scanCancel()
		s.scanWorker.Start(scanCtx, s.cfg.Scanner.Workers)
	}

	// Start tiering manager if enabled
	if s.tieringMgr != nil {
		tierCtx, tierCancel := context.WithCancel(context.Background())
		defer tierCancel()
		go s.tieringMgr.Run(tierCtx)
	}

	// Start backup scheduler if enabled
	if s.backupSched != nil {
		backupCtx, backupCancel := context.WithCancel(context.Background())
		defer backupCancel()
		go s.backupSched.Run(backupCtx)
	}

	log.Printf("  Search:       %d objects indexed", s.searchIndex.Count())

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

func initBuiltinPolicies(store *metadata.Store) {
	builtins := []metadata.IAMPolicy{
		{
			Name:     "ReadOnlyAccess",
			Document: `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["s3:GetObject","s3:ListBucket","s3:ListAllMyBuckets","s3:GetBucketPolicy"],"Resource":["*"]}]}`,
		},
		{
			Name:     "ReadWriteAccess",
			Document: `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["s3:GetObject","s3:PutObject","s3:DeleteObject","s3:ListBucket","s3:ListAllMyBuckets"],"Resource":["*"]}]}`,
		},
		{
			Name:     "FullAccess",
			Document: `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["s3:*"],"Resource":["*"]}]}`,
		},
	}

	for _, p := range builtins {
		p.CreatedAt = time.Now().UTC()
		// Use CreateIAMPolicy which is a no-op if already exists
		store.CreateIAMPolicy(p)
	}
}

func (s *Server) Close() {
	if s.rateLimiter != nil {
		s.rateLimiter.Stop()
	}
	if s.accessLog != nil {
		s.accessLog.Close()
	}
	if s.store != nil {
		s.store.Close()
	}
}
