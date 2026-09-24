package maxapi_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dommax/internal/storage/maxapi"
)

// selfSignedPEM — корневой сертификат для проверки загрузки CA из файла (как сертификат Минцифры).
func selfSignedPEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Test Root CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func transport(t *testing.T, c *http.Client) *http.Transport {
	t.Helper()
	tr, ok := c.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport = %T", c.Transport)
	}
	return tr
}

func TestHTTPClientTrustsCAFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(path, selfSignedPEM(t), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := maxapi.HTTPClient(path, false)
	if err != nil {
		t.Fatalf("HTTPClient: %v", err)
	}
	cfg := transport(t, c).TLSClientConfig
	if cfg.RootCAs == nil || cfg.InsecureSkipVerify {
		t.Fatalf("tls config = %+v", cfg)
	}
}

func TestHTTPClientRejectsBadCAFile(t *testing.T) {
	empty := filepath.Join(t.TempDir(), "empty.pem")
	if err := os.WriteFile(empty, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := maxapi.HTTPClient(empty, false); err == nil || !strings.Contains(err.Error(), "no PEM") {
		t.Fatalf("empty file err = %v", err)
	}
	if _, err := maxapi.HTTPClient(filepath.Join(t.TempDir(), "missing.pem"), false); err == nil {
		t.Fatal("missing file must be an error")
	}
}

func TestHTTPClientModes(t *testing.T) {
	insecure, err := maxapi.HTTPClient("", true)
	if err != nil || !transport(t, insecure).TLSClientConfig.InsecureSkipVerify {
		t.Fatalf("insecure mode: %v", err)
	}
	system, err := maxapi.HTTPClient("", false)
	cfg := transport(t, system).TLSClientConfig
	if err != nil || cfg.InsecureSkipVerify || cfg.RootCAs != nil || cfg.MinVersion == 0 {
		t.Fatalf("system trust mode: %+v, %v", cfg, err)
	}
}
