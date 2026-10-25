package feeds

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"conexiuni-cluj/web"
)

const StallTimeout = 30 * time.Second

const (
	maxIssuerFetches = 4
	maxIssuerBytes   = 64 << 10
)

var client = &http.Client{Transport: issuerFetching(nil)}

func Download(url string) ([]byte, error) {
	return fetch(client, url)
}

type StatusError struct {
	URL  string
	Code int
}

func (e *StatusError) Error() string { return fmt.Sprintf("%s: status %d", e.URL, e.Code) }

// Some servers send only their own certificate. Like browsers, fetch the
// missing issuers from the address each certificate names.
func issuerFetching(roots *x509.CertPool) *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.TLSClientConfig = &tls.Config{
		// Verified in full by VerifyConnection.
		InsecureSkipVerify: true,
		VerifyConnection: func(cs tls.ConnectionState) error {
			return verify(cs, roots)
		},
	}
	return t
}

func verify(cs tls.ConnectionState, roots *x509.CertPool) error {
	// An IP host sends no server name, which would skip the hostname check.
	if cs.ServerName == "" {
		return errors.New("tls: no server name to verify")
	}
	opts := x509.VerifyOptions{DNSName: cs.ServerName, Roots: roots, Intermediates: x509.NewCertPool()}
	for _, cert := range cs.PeerCertificates[1:] {
		opts.Intermediates.AddCert(cert)
	}
	fetched := map[string]bool{}
	for {
		_, err := cs.PeerCertificates[0].Verify(opts)
		var unknown x509.UnknownAuthorityError
		if !errors.As(err, &unknown) || unknown.Cert == nil || len(fetched) >= maxIssuerFetches {
			return err
		}
		issuer := fetchIssuer(unknown.Cert, fetched)
		if issuer == nil {
			return err
		}
		opts.Intermediates.AddCert(issuer)
	}
}

func fetchIssuer(cert *x509.Certificate, fetched map[string]bool) *x509.Certificate {
	for _, url := range cert.IssuingCertificateURL {
		if fetched[url] {
			continue
		}
		fetched[url] = true
		data, err := fetchAtMost(http.DefaultClient, url, maxIssuerBytes)
		if err != nil {
			continue
		}
		if block, _ := pem.Decode(data); block != nil {
			data = block.Bytes
		}
		if issuer, err := x509.ParseCertificate(data); err == nil {
			return issuer
		}
	}
	return nil
}

// Some hosts turn away Go's own agent (gtfs.opendev.ro resets it and then the address for a while).
var userAgent = strings.TrimPrefix(web.SiteOrigin, "https://") + " (+" + web.SiteOrigin + "/)"

func fetch(c *http.Client, url string) ([]byte, error) { return fetchAtMost(c, url, 0) }

func fetchAtMost(c *http.Client, url string, limit int64) ([]byte, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stalled := time.AfterFunc(StallTimeout, cancel)
	defer stalled.Stop()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.Do(req)
	if err != nil {
		return nil, stallError(ctx, url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &StatusError{URL: url, Code: resp.StatusCode}
	}
	var body io.Reader = progress{resp.Body, func() { stalled.Reset(StallTimeout) }}
	if limit > 0 {
		body = io.LimitReader(body, limit+1)
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, stallError(ctx, url, err)
	}
	if limit > 0 && int64(len(data)) > limit {
		return nil, fmt.Errorf("%s: over %d bytes", url, limit)
	}
	return data, nil
}

func stallError(ctx context.Context, url string, err error) error {
	if ctx.Err() != nil {
		return fmt.Errorf("%s: nothing received for %s", url, StallTimeout)
	}
	return err
}

type progress struct {
	r      io.Reader
	onRead func()
}

func (p progress) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	if n > 0 {
		p.onRead()
	}
	return n, err
}
