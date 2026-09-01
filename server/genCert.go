package server

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

// Generate a self-signed certificate as long as the server is running.
func serial_number() *big.Int {
	serialNumLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial_num, serial_err := rand.Int(rand.Reader, serialNumLimit)
	if serial_err != nil {
		return big.NewInt(time.Now().UnixNano())
	}
	return serial_num
}

func gen_cert() (*tls.Config, error) {
	ca := &x509.Certificate{
		SerialNumber: serial_number(),
		Subject: pkix.Name{
			Country:      []string{"US"},
			Organization: []string{"NVDARemote Server"},
			CommonName:   "Root CA",
		},
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:             time.Now().Add(-10 * time.Second),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	// elliptic.P521() curve constant is not deprecated — only
	// elliptic.GenerateKey() is. We use ecdsa.GenerateKey() with
	// the elliptic curve constant, which is the recommended pattern.
	priv, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating ECDSA P-521 key: %w", err)
	}
	// SubjectKeyId and AuthorityKeyId are intentionally left empty.
	// Since Go 1.25, CreateCertificate populates SubjectKeyId
	// automatically using truncated SHA-256, which is more secure
	// than the previous SHA-1 approach.
	caBytes, cerr := x509.CreateCertificate(rand.Reader, ca, ca, &priv.PublicKey, priv)
	if cerr != nil {
		return nil, fmt.Errorf("creating X.509 certificate: %w", cerr)
	}

	certPEM := new(bytes.Buffer)
	err = pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("encoding certificate PEM: %w", err)
	}

	tmpk, merr := x509.MarshalPKCS8PrivateKey(priv)
	if merr != nil {
		return nil, fmt.Errorf("marshaling PKCS8 private key: %w", merr)
	}

	certPrivKeyPEM := new(bytes.Buffer)
	err = pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: tmpk,
	})
	if err != nil {
		return nil, fmt.Errorf("encoding private key PEM: %w", err)
	}

	serverCert, serr := tls.X509KeyPair(certPEM.Bytes(), certPrivKeyPEM.Bytes())
	if serr != nil {
		return nil, fmt.Errorf("parsing X509 key pair: %w", serr)
	}

	serverTLSConf := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
	}

	return serverTLSConf, nil
}
