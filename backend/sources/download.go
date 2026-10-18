package sources

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// The API clients' timeout; a file of any size fails only when it stops arriving for this long.
const StallTimeout = 30 * time.Second

func Download(url string) ([]byte, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stalled := time.AfterFunc(StallTimeout, cancel)
	defer stalled.Stop()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, stallError(ctx, url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: status %d", url, resp.StatusCode)
	}
	data, err := io.ReadAll(progress{resp.Body, func() { stalled.Reset(StallTimeout) }})
	if err != nil {
		return nil, stallError(ctx, url, err)
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
