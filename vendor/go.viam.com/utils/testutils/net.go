package testutils

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"testing"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/test"
)

var waitDur = 5 * time.Second

// ReserveRandomListener returns a new TCP listener at a random port.
func ReserveRandomListener(tb testing.TB) *net.TCPListener {
	tb.Helper()
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{Port: 0})
	test.That(tb, err, test.ShouldBeNil)
	return listener
}

// WaitSuccessfulDial waits for a dial attempt to succeed.
func WaitSuccessfulDial(address string) error {
	ctx, cancel := context.WithTimeout(context.Background(), waitDur)
	var lastErr error
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return multierr.Combine(ctx.Err(), lastErr)
		default:
		}
		var conn net.Conn
		conn, lastErr = net.Dial("tcp", address)
		if lastErr == nil {
			return conn.Close()
		}
		lastErr = errors.WithStack(lastErr)
	}
}

// GenerateSelfSignedCertificate generates a self signed certificate with the given names.
func GenerateSelfSignedCertificate(commonName string, altNames ...string) (tls.Certificate, string, string, *x509.CertPool, error) {
	// Create Root Certificate Authority.
	caCert := &x509.Certificate{
		SerialNumber: big.NewInt(2022),
		Subject: pkix.Name{
			Organization:  []string{"Base"},
			Country:       []string{"Space"},
			Locality:      []string{"Moon"},
			StreetAddress: []string{"123 Main Street"},
			PostalCode:    []string{"123456"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caCertPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return tls.Certificate{}, "", "", nil, err
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, caCert, caCert, &caCertPrivKey.PublicKey, caCertPrivKey)
	if err != nil {
		return tls.Certificate{}, "", "", nil, err
	}

	// Create Certificate issued by the CA.
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(2023),
		Subject: pkix.Name{
			CommonName:    commonName,
			Organization:  []string{"Base"},
			Country:       []string{"Space"},
			Locality:      []string{"Moon"},
			StreetAddress: []string{"124 Main Street"},
			PostalCode:    []string{"123456"},
		},
		DNSNames:     append([]string{commonName}, altNames...),
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(1, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	certPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return tls.Certificate{}, "", "", nil, err
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, caCert, &certPrivKey.PublicKey, caCertPrivKey)
	if err != nil {
		return tls.Certificate{}, "", "", nil, err
	}

	var certPEM bytes.Buffer
	if err := pem.Encode(&certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	}); err != nil {
		return tls.Certificate{}, "", "", nil, err
	}

	var certPrivKeyPEM bytes.Buffer
	if err := pem.Encode(&certPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
	}); err != nil {
		return tls.Certificate{}, "", "", nil, err
	}

	tlsCert, err := tls.X509KeyPair(certPEM.Bytes(), certPrivKeyPEM.Bytes())
	if err != nil {
		return tls.Certificate{}, "", "", nil, err
	}

	certPool := x509.NewCertPool()
	var caPEM bytes.Buffer
	if err := pem.Encode(&caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	}); err != nil {
		return tls.Certificate{}, "", "", nil, err
	}
	if !certPool.AppendCertsFromPEM(caPEM.Bytes()) {
		return tls.Certificate{}, "", "", nil, errors.New("error adding CA to pool")
	}

	certFile, err := os.CreateTemp("", "cert")
	if err != nil {
		return tls.Certificate{}, "", "", nil, err
	}
	if _, err := certFile.WriteString(certPEM.String()); err != nil {
		return tls.Certificate{}, "", "", nil, err
	}

	keyFile, err := os.CreateTemp("", "key")
	if err != nil {
		return tls.Certificate{}, "", "", nil, err
	}
	if _, err := keyFile.WriteString(certPrivKeyPEM.String()); err != nil {
		return tls.Certificate{}, "", "", nil, err
	}

	return tlsCert, certFile.Name(), keyFile.Name(), certPool, nil
}
