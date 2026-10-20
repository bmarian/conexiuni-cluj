package feeds

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type issued struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

func issue(t *testing.T, template *x509.Certificate, parent *issued) *issued {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template.SerialNumber = big.NewInt(time.Now().UnixNano())
	template.NotBefore = time.Now().Add(-time.Hour)
	template.NotAfter = time.Now().Add(time.Hour)
	parentCert, parentKey := template, key
	if parent != nil {
		parentCert, parentKey = parent.cert, parent.key
	}
	der, err := x509.CreateCertificate(rand.Reader, template, parentCert, &key.PublicKey, parentKey)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return &issued{cert, key}
}

// A server that sends only its own certificate, whose issuer is published elsewhere.
func leafOnlyServer(t *testing.T, dnsName string) (url string, roots *x509.CertPool) {
	ca := func(name string) *x509.Certificate {
		return &x509.Certificate{Subject: pkix.Name{CommonName: name}, IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	}
	root := issue(t, ca("root"), nil)
	intermediate := issue(t, ca("intermediate"), root)
	issuers := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(intermediate.cert.Raw)
	}))
	t.Cleanup(issuers.Close)
	leaf := issue(t, &x509.Certificate{
		Subject:               pkix.Name{CommonName: dnsName},
		DNSNames:              []string{dnsName},
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IssuingCertificateURL: []string{issuers.URL + "/intermediate.cer"},
	}, intermediate)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("timetable"))
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{leaf.cert.Raw}, PrivateKey: leaf.key}}}
	server.StartTLS()
	t.Cleanup(server.Close)
	roots = x509.NewCertPool()
	roots.AddCert(root.cert)
	return strings.Replace(server.URL, "127.0.0.1", "localhost", 1), roots
}

func TestFetchesMissingIssuer(t *testing.T) {
	url, roots := leafOnlyServer(t, "localhost")
	data, err := fetch(&http.Client{Transport: issuerFetching(roots)}, url)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "timetable" {
		t.Fatalf("got %q", data)
	}
}

func TestStillChecksHostname(t *testing.T) {
	url, roots := leafOnlyServer(t, "elsewhere.test")
	if _, err := fetch(&http.Client{Transport: issuerFetching(roots)}, url); err == nil {
		t.Fatal("accepted a certificate for another host")
	}
}

func TestStillChecksRoot(t *testing.T) {
	url, _ := leafOnlyServer(t, "localhost")
	if _, err := fetch(&http.Client{Transport: issuerFetching(x509.NewCertPool())}, url); err == nil {
		t.Fatal("accepted an untrusted root")
	}
}
