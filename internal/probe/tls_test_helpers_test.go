package probe

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"testing"
	"time"
)

func newTLSTestServer(t *testing.T, serverName string, notBefore time.Time, notAfter time.Time) (*httptest.Server, *url.URL, []netip.Addr, *x509.CertPool, *x509.Certificate) {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa.GenerateKey() error = %v, want nil", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "SignalDock Test CA",
		},
		NotBefore:             notBefore.Add(-24 * time.Hour),
		NotAfter:              notAfter.Add(365 * 24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}

	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("x509.CreateCertificate(CA) error = %v, want nil", err)
	}

	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("x509.ParseCertificate(CA) error = %v, want nil", err)
	}

	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa.GenerateKey() error = %v, want nil", err)
	}

	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName: serverName,
		},
		DNSNames:  []string{serverName},
		NotBefore: notBefore,
		NotAfter:  notAfter,
		KeyUsage:  x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
	}

	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("x509.CreateCertificate(server) error = %v, want nil", err)
	}

	serverCert, err := x509.ParseCertificate(serverDER)
	if err != nil {
		t.Fatalf("x509.ParseCertificate(server) error = %v, want nil", err)
	}

	serverKeyDER, err := x509.MarshalECPrivateKey(serverKey)
	if err != nil {
		t.Fatalf("x509.MarshalECPrivateKey() error = %v, want nil", err)
	}

	certificate, err := tls.X509KeyPair(
		pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: serverDER,
		}),
		pem.EncodeToMemory(&pem.Block{
			Type:  "EC PRIVATE KEY",
			Bytes: serverKeyDER,
		}),
	)
	if err != nil {
		t.Fatalf("tls.X509KeyPair() error = %v, want nil", err)
	}

	server := httptest.NewUnstartedServer(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	)

	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{certificate},
		MinVersion:   tls.VersionTLS12,
	}

	server.StartTLS()
	t.Cleanup(server.Close)

	serverURL := mustParseURL(t, server.URL)

	parsedURL := mustParseURL(t, "https://"+serverName+":"+serverURL.Port())

	roots := x509.NewCertPool()
	roots.AddCert(caCert)

	ips := []netip.Addr{
		netip.MustParseAddr("127.0.0.1"),
	}

	return server, parsedURL, ips, roots, serverCert
}
