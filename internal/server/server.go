package server

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/eniz1806/VaultS3/internal/config"
	"github.com/eniz1806/VaultS3/internal/metadata"
	"github.com/eniz1806/VaultS3/internal/s3"
	"github.com/eniz1806/VaultS3/internal/storage"
)

type Server struct {
	cfg    *config.Config
	store  *metadata.Store
	engine storage.Engine
	s3h    *s3.Handler
}

func New(cfg *config.Config) (*Server, error) {
	// Initialize storage engine
	engine, err := storage.NewFileSystem(cfg.Storage.DataDir)
	if err != nil {
		return nil, fmt.Errorf("init storage: %w", err)
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

	// Initialize S3 handler
	s3h := s3.NewHandler(store, engine, auth)

	return &Server{
		cfg:    cfg,
		store:  store,
		engine: engine,
		s3h:    s3h,
	}, nil
}

func (s *Server) Start() error {
	addr := s.cfg.ListenAddr()

	mux := http.NewServeMux()

	// S3 API — catch all
	mux.Handle("/", s.s3h)

	log.Printf("VaultS3 starting on %s", addr)
	log.Printf("  Data dir:     %s", s.cfg.Storage.DataDir)
	log.Printf("  Metadata dir: %s", s.cfg.Storage.MetadataDir)
	log.Printf("  Access key:   %s", s.cfg.Auth.AdminAccessKey)

	return http.ListenAndServe(addr, mux)
}

func (s *Server) Close() {
	if s.store != nil {
		s.store.Close()
	}
}
