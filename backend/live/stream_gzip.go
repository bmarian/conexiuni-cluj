package live

import (
	"encoding/binary"
	"hash/crc32"
	"io"

	"github.com/klauspost/compress/flate"
)

// The most flate.StatelessDeflate looks back.
const streamGzipDict = 8 << 10

// streamGzip keeps no compressor between events: each is deflated alone against the last bytes sent,
// so an open stream holds kilobytes instead of the megabyte a gzip.Writer keeps.
type streamGzip struct {
	w       io.Writer
	started bool
	dict    []byte
	crc     uint32
	size    uint32
}

func newStreamGzip(w io.Writer) *streamGzip {
	return &streamGzip{w: w, dict: make([]byte, 0, streamGzipDict)}
}

func (g *streamGzip) header() error {
	if g.started {
		return nil
	}
	g.started = true
	_, err := g.w.Write([]byte{0x1f, 0x8b, 8, 0, 0, 0, 0, 0, 0, 255})
	return err
}

// write ends on a byte boundary, so the client can decode the event at once.
func (g *streamGzip) write(p []byte) error {
	if err := g.header(); err != nil {
		return err
	}
	if err := flate.StatelessDeflate(g.w, p, false, g.dict); err != nil {
		return err
	}
	g.crc = crc32.Update(g.crc, crc32.IEEETable, p)
	g.size += uint32(len(p))
	if len(p) >= streamGzipDict {
		g.dict = append(g.dict[:0], p[len(p)-streamGzipDict:]...)
		return nil
	}
	if keep := streamGzipDict - len(p); len(g.dict) > keep {
		g.dict = append(g.dict[:0], g.dict[len(g.dict)-keep:]...)
	}
	g.dict = append(g.dict, p...)
	return nil
}

func (g *streamGzip) close() error {
	if err := g.header(); err != nil {
		return err
	}
	if err := flate.StatelessDeflate(g.w, nil, true, nil); err != nil {
		return err
	}
	var trailer [8]byte
	binary.LittleEndian.PutUint32(trailer[:4], g.crc)
	binary.LittleEndian.PutUint32(trailer[4:], g.size)
	_, err := g.w.Write(trailer[:])
	return err
}
