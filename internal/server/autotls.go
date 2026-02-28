package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

// AutoTLSConfig holds auto-TLS settings.
type AutoTLSConfig struct {
	Enabled    bool     `json:"enabled" yaml:"enabled"`
	Domains    []string `json:"domains" yaml:"domains"`
	CacheDir   string   `json:"cache_dir" yaml:"cache_dir"`
	SelfSigned bool     `json:"self_signed" yaml:"self_signed"`
}

// NewAutoTLS creates a TLS config using Let's Encrypt auto-cert.
func NewAutoTLS(cfg AutoTLSConfig) (*tls.Config, http.Handler) {
	if cfg.SelfSigned {
		tlsCfg, err := generateSelfSigned()
		if err != nil {
			panic(fmt.Sprintf("failed to generate self-signed cert: %v", err))
		}
		return tlsCfg, nil
	}

	cacheDir := cfg.CacheDir
	if cacheDir == "" {
		cacheDir = "autocert-cache"
	}

	m := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Cache:      autocert.DirCache(cacheDir),
		HostPolicy: autocert.HostWhitelist(cfg.Domains...),
	}

	return m.TLSConfig(), m.HTTPHandler(nil)
}

// generateSelfSigned generates a self-signed TLS certificate.
func generateSelfSigned() (*tls.Config, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	template := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{Organization: []string{"VaultS3"}},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}

	return &tls.Config{Certificates: []tls.Certificate{cert}}, nil
}
