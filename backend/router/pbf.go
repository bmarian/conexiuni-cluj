package router

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sync"
)

// Expects nodes before ways, as Geofabrik writes them.

type pbfFilter struct {
	node func(lat, lon float64) bool
	way  func(tag func(key string) string) bool
	// Tagged nodes in the area and ways that are places; nil keeps none.
	place func(tag func(key string) string, way bool) (placeTags, bool)
}

type pbfNode struct {
	id       int64
	lat, lon float64
}

type pbfBlock struct {
	nodes  []pbfNode
	ways   [][]int64
	places []pbfPlace
	err    error
}

// readPBF decodes blocks on every core and hands them back in file order.
func readPBF(r io.Reader, keep pbfFilter, each func(pbfBlock)) error {
	workers := runtime.GOMAXPROCS(0)
	type job struct {
		seq  int
		blob []byte
	}
	jobs := make(chan job, workers)
	results := make(chan struct {
		seq int
		pbfBlock
	}, workers)
	inflight := make(chan struct{}, 2*workers)
	var readErr error
	done := make(chan struct{})

	go func() {
		defer close(jobs)
		for seq := 0; ; seq++ {
			blob, err := nextBlob(r)
			if err != nil || blob == nil {
				readErr = err
				return
			}
			select {
			case inflight <- struct{}{}:
			case <-done:
				return
			}
			jobs <- job{seq, blob}
		}
	}()
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				results <- struct {
					seq int
					pbfBlock
				}{j.seq, decodeBlock(j.blob, keep)}
			}
		}()
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	pending := make(map[int]pbfBlock)
	next := 0
	var firstErr error
	for res := range results {
		pending[res.seq] = res.pbfBlock
		for b, ok := pending[next]; ok; b, ok = pending[next] {
			delete(pending, next)
			next++
			<-inflight
			if b.err != nil && firstErr == nil {
				firstErr = b.err
				close(done)
			}
			if firstErr == nil {
				each(b)
			}
		}
	}
	if firstErr != nil {
		return firstErr
	}
	return readErr
}

func nextBlob(r io.Reader) ([]byte, error) {
	var size [4]byte
	for {
		if _, err := io.ReadFull(r, size[:]); err != nil {
			if errors.Is(err, io.EOF) {
				return nil, nil
			}
			return nil, err
		}
		header := make([]byte, binary.BigEndian.Uint32(size[:]))
		if _, err := io.ReadFull(r, header); err != nil {
			return nil, err
		}
		var kind string
		var dataSize int
		for p := (pbuf{b: header}); p.more(); {
			switch f, w := p.key(); f {
			case 1:
				kind = string(p.bytes())
			case 3:
				dataSize = int(p.varint())
			default:
				p.skip(w)
			}
		}
		blob := make([]byte, dataSize)
		if _, err := io.ReadFull(r, blob); err != nil {
			return nil, err
		}
		if kind == "OSMData" {
			return blob, nil
		}
	}
}

func blobData(blob []byte) ([]byte, error) {
	for p := (pbuf{b: blob}); p.more(); {
		switch f, w := p.key(); f {
		case 1:
			return p.bytes(), nil
		case 3:
			z, err := zlib.NewReader(bytes.NewReader(p.bytes()))
			if err != nil {
				return nil, err
			}
			defer z.Close()
			return io.ReadAll(z)
		default:
			p.skip(w)
		}
	}
	return nil, fmt.Errorf("pbf: blob without raw or zlib data")
}

func decodeBlock(blob []byte, keep pbfFilter) (out pbfBlock) {
	data, err := blobData(blob)
	if err != nil {
		return pbfBlock{err: err}
	}
	var strs [][]byte
	var groups [][]byte
	granularity, latOffset, lonOffset := int64(100), int64(0), int64(0)
	for p := (pbuf{b: data}); p.more(); {
		switch f, w := p.key(); f {
		case 1:
			for s := (pbuf{b: p.bytes()}); s.more(); {
				if f, w := s.key(); f == 1 {
					strs = append(strs, s.bytes())
				} else {
					s.skip(w)
				}
			}
		case 2:
			groups = append(groups, p.bytes())
		case 17:
			granularity = int64(p.varint())
		case 19:
			latOffset = int64(p.varint())
		case 20:
			lonOffset = int64(p.varint())
		default:
			p.skip(w)
		}
	}
	coord := func(offset, v int64) float64 { return 1e-9 * float64(offset+granularity*v) }
	var keys, vals, kv []uint64
	var refs, ids, lats, lons []int64
	tag := func(k string) string {
		for i := range min(len(keys), len(vals)) {
			if keys[i] < uint64(len(strs)) && vals[i] < uint64(len(strs)) && string(strs[keys[i]]) == k {
				return string(strs[vals[i]])
			}
		}
		return ""
	}
	node := func(id, lat, lon int64) {
		la, lo := coord(latOffset, lat), coord(lonOffset, lon)
		if !keep.node(la, lo) {
			return
		}
		out.nodes = append(out.nodes, pbfNode{id, la, lo})
		if keep.place != nil && len(keys) > 0 {
			if p, ok := keep.place(tag, false); ok {
				out.places = append(out.places, pbfPlace{placeTags: p, node: id, lat: la, lon: lo, placed: true})
			}
		}
	}
	for _, g := range groups {
		for p := (pbuf{b: g}); p.more(); {
			switch f, w := p.key(); f {
			case 1:
				var id, lat, lon int64
				keys, vals = keys[:0], vals[:0]
				for n := (pbuf{b: p.bytes()}); n.more(); {
					switch f, w := n.key(); f {
					case 1:
						id = zigzag(n.varint())
					case 2:
						keys = n.uints(w, keys)
					case 4:
						vals = n.uints(w, vals)
					case 8:
						lat = zigzag(n.varint())
					case 9:
						lon = zigzag(n.varint())
					default:
						n.skip(w)
					}
				}
				node(id, lat, lon)
			case 2:
				ids, lats, lons, kv = ids[:0], lats[:0], lons[:0], kv[:0]
				for d := (pbuf{b: p.bytes()}); d.more(); {
					switch f, w := d.key(); f {
					case 1:
						ids = d.sints(w, ids)
					case 8:
						lats = d.sints(w, lats)
					case 9:
						lons = d.sints(w, lons)
					case 10:
						if keep.place != nil {
							kv = d.uints(w, kv)
						} else {
							d.skip(w)
						}
					default:
						d.skip(w)
					}
				}
				if len(lats) != len(ids) || len(lons) != len(ids) {
					return pbfBlock{err: fmt.Errorf("pbf: dense nodes with %d ids, %d lats, %d lons", len(ids), len(lats), len(lons))}
				}
				// Tags come as key, value pairs per node, each node's ended by a 0.
				var id, lat, lon int64
				at := 0
				for i := range ids {
					id, lat, lon = id+ids[i], lat+lats[i], lon+lons[i]
					keys, vals = keys[:0], vals[:0]
					for ; at+1 < len(kv) && kv[at] != 0; at += 2 {
						keys, vals = append(keys, kv[at]), append(vals, kv[at+1])
					}
					at++
					node(id, lat, lon)
				}
			case 3:
				keys, vals, refs = keys[:0], vals[:0], refs[:0]
				for wp := (pbuf{b: p.bytes()}); wp.more(); {
					switch f, w := wp.key(); f {
					case 2:
						keys = wp.uints(w, keys)
					case 3:
						vals = wp.uints(w, vals)
					case 8:
						refs = wp.sints(w, refs)
					default:
						wp.skip(w)
					}
				}
				if keep.place != nil && len(refs) > 0 {
					if p, ok := keep.place(tag, true); ok {
						out.places = append(out.places, pbfPlace{placeTags: p, node: refs[0]})
					}
				}
				if !keep.way(tag) {
					continue
				}
				way := make([]int64, len(refs))
				for i, d := range refs {
					if i == 0 {
						way[i] = d
					} else {
						way[i] = way[i-1] + d
					}
				}
				out.ways = append(out.ways, way)
			default:
				p.skip(w)
			}
		}
	}
	return out
}

type pbuf struct {
	b   []byte
	i   int
	err bool
}

func (p *pbuf) more() bool { return !p.err && p.i < len(p.b) }

func (p *pbuf) key() (int, int) {
	k := p.varint()
	return int(k >> 3), int(k & 7)
}

func (p *pbuf) varint() uint64 {
	v, n := binary.Uvarint(p.b[p.i:])
	if n <= 0 {
		p.err = true
		return 0
	}
	p.i += n
	return v
}

func (p *pbuf) bytes() []byte {
	n := int(p.varint())
	if p.err || n < 0 || p.i+n > len(p.b) {
		p.err = true
		return nil
	}
	out := p.b[p.i : p.i+n]
	p.i += n
	return out
}

func (p *pbuf) skip(wire int) {
	switch wire {
	case 0:
		p.varint()
	case 1:
		p.i += 8
	case 2:
		p.bytes()
	case 5:
		p.i += 4
	default:
		p.err = true
	}
}

func (p *pbuf) uints(wire int, out []uint64) []uint64 {
	if wire != 2 {
		return append(out, p.varint())
	}
	for packed := (pbuf{b: p.bytes()}); packed.more(); {
		out = append(out, packed.varint())
	}
	return out
}

func (p *pbuf) sints(wire int, out []int64) []int64 {
	if wire != 2 {
		return append(out, zigzag(p.varint()))
	}
	for packed := (pbuf{b: p.bytes()}); packed.more(); {
		out = append(out, zigzag(packed.varint()))
	}
	return out
}

func zigzag(v uint64) int64 { return int64(v>>1) ^ -int64(v&1) }
