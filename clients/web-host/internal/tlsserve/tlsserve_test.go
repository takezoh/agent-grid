package tlsserve

import (
	"crypto/tls"
	"testing"
)

func TestSelfSignedCertificateIsUsable(t *testing.T) {
	certificate, err := selfSignedCertificate()
	if err != nil {
		t.Fatal(err)
	}
	config := &tls.Config{
		Certificates: []tls.Certificate{certificate},
		MinVersion:   tls.VersionTLS12,
	}
	if len(config.Certificates) != 1 || config.Certificates[0].PrivateKey == nil {
		t.Fatal("self-signed certificate is incomplete")
	}
}
