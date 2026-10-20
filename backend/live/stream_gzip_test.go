package live

import (
	"compress/gzip"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

func TestStreamGzipDecodesEachEventAtOnce(t *testing.T) {
	var rows []string
	for i := range 400 {
		rows = append(rows, fmt.Sprintf(`{"stop":%d,"line":"%d","eta":%d}`, i, i%40, i*7))
	}
	events := []string{"event: snapshot\ndata: [" + strings.Join(rows, ",") + "]\n\n"}
	for i := range 50 {
		events = append(events, fmt.Sprintf("event: delta\ndata: {\"stop\":%d,\"eta\":%d}\n\n", i%7, i))
		if i%10 == 0 {
			events = append(events, ": ping\n\n")
		}
	}

	pr, pw := io.Pipe()
	got := make(chan string)
	end := make(chan error, 1)
	go func() {
		zr, err := gzip.NewReader(pr)
		if err != nil {
			end <- err
			return
		}
		for _, e := range events {
			buf := make([]byte, len(e))
			if _, err := io.ReadFull(zr, buf); err != nil {
				end <- err
				return
			}
			got <- string(buf)
		}
		// Reading to the end checks the CRC and length.
		rest, err := io.ReadAll(zr)
		if err == nil && len(rest) > 0 {
			err = fmt.Errorf("%d bytes after the last event", len(rest))
		}
		end <- err
	}()

	g := newStreamGzip(pw)
	for i, e := range events {
		if err := g.write([]byte(e)); err != nil {
			t.Fatal(err)
		}
		select {
		case s := <-got:
			if s != e {
				t.Fatalf("event %d decoded as %q, want %q", i, s, e)
			}
		case err := <-end:
			t.Fatalf("event %d: %v", i, err)
		case <-time.After(5 * time.Second):
			t.Fatalf("event %d not decodable before the next one", i)
		}
	}
	if err := g.close(); err != nil {
		t.Fatal(err)
	}
	pw.Close()
	if err := <-end; err != nil {
		t.Fatal(err)
	}
}
