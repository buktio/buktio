package app

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"time"

	"golang.org/x/crypto/acme/autocert"

	"github.com/buktio/buktio/internal/config"
)

// serveWithTLS starts srv according to the configured TLS mode and blocks until it
// returns. This enables the single-binary "lite" deployment (no edge proxy) to
// serve HTTPS directly. "off" keeps plaintext HTTP for setups that terminate TLS
// upstream (the "full" Caddy profile, or any reverse proxy / k8s ingress).
func serveWithTLS(srv *http.Server, c config.TLSConfig, logger *slog.Logger) error {
	switch c.Mode {
	case "", "off":
		return srv.ListenAndServe()

	case "self":
		cert, err := selfSignedCert(c.Domains)
		if err != nil {
			return fmt.Errorf("tls self: %w", err)
		}
		srv.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
		logger.Warn("serving HTTPS with a self-signed certificate (browsers will warn); set BUKTIO_TLS=auto for a real certificate")
		return srv.ListenAndServeTLS("", "")

	case "auto":
		m := &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(c.Domains...),
			Cache:      autocert.DirCache(c.CacheDir),
			Email:      c.Email,
		}
		srv.TLSConfig = m.TLSConfig()
		// ACME HTTP-01 challenge + plain-HTTP → HTTPS redirect on :80.
		go func() {
			if err := http.ListenAndServe(":80", m.HTTPHandler(nil)); err != nil {
				logger.Error("ACME http-01 listener stopped", slog.Any("error", err))
			}
		}()
		logger.Info("serving HTTPS via Let's Encrypt (ACME)", slog.Any("domains", c.Domains))
		return srv.ListenAndServeTLS("", "")
	}
	return fmt.Errorf("unknown TLS mode %q", c.Mode)
}

// selfSignedCert mints an in-memory ECDSA self-signed certificate valid for
// loopback plus any configured domains. Regenerated on every start — it is meant
// for quick private/homelab use, not trust.
func selfSignedCert(domains []string) (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, err
	}
	tmpl := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "buktio"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              append([]string{"localhost"}, domains...),
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, nil
}
