package backup

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/eniz1806/VaultS3/internal/config"
)

type Target interface {
	Write(bucket, key string, reader io.Reader, size int64) error
	Close() error
}

func NewTarget(cfg config.BackupTarget) (Target, error) {
	switch cfg.Type {
	case "local", "":
		return NewLocalTarget(cfg.Path)
	default:
		return nil, fmt.Errorf("unsupported backup target type: %s", cfg.Type)
	}
}

type LocalTarget struct {
	basePath string
}

func NewLocalTarget(basePath string) (*LocalTarget, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("create backup dir: %w", err)
	}
	return &LocalTarget{basePath: basePath}, nil
}

func (t *LocalTarget) Write(bucket, key string, reader io.Reader, size int64) error {
	path := filepath.Join(t.basePath, bucket, key)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, reader)
	return err
}

func (t *LocalTarget) Close() error {
	return nil
}
