package gtfsrt

import (
	"encoding/binary"
	"errors"
	"math"
)

// Read straight off the protobuf wire: the vehiclePositions schema is small and fixed.
type field struct {
	num    int
	wire   int
	varint uint64
	bytes  []byte
}

var errWire = errors.New("gtfsrt: malformed protobuf")

func fields(m []byte) ([]field, error) {
	var out []field
	for i := 0; i < len(m); {
		key, n := binary.Uvarint(m[i:])
		if n <= 0 {
			return nil, errWire
		}
		i += n
		f := field{num: int(key >> 3), wire: int(key & 7)}
		switch f.wire {
		case 0:
			v, n := binary.Uvarint(m[i:])
			if n <= 0 {
				return nil, errWire
			}
			f.varint, i = v, i+n
		case 1, 5:
			size := 8
			if f.wire == 5 {
				size = 4
			}
			if i+size > len(m) {
				return nil, errWire
			}
			f.bytes, i = m[i:i+size], i+size
		case 2:
			l, n := binary.Uvarint(m[i:])
			if n <= 0 || i+n+int(l) > len(m) {
				return nil, errWire
			}
			f.bytes, i = m[i+n:i+n+int(l)], i+n+int(l)
		default:
			return nil, errWire
		}
		out = append(out, f)
	}
	return out, nil
}

func (f field) float32() float64 {
	if len(f.bytes) != 4 {
		return 0
	}
	return float64(math.Float32frombits(binary.LittleEndian.Uint32(f.bytes)))
}

type position struct {
	routeID, stopID, vehicleID, label string
	direction, stopSequence           int // -1 when absent
	lat, lon, speed                   float64
	hasSpeed                          bool
	timestamp                         int64
}

type feedMessage struct {
	timestamp int64
	positions []position
}

// decode reads FeedMessage → FeedEntity.vehicle (VehiclePosition) with its trip, position, stop and vehicle.
func decode(data []byte) (feedMessage, error) {
	var out feedMessage
	top, err := fields(data)
	if err != nil {
		return out, err
	}
	for _, f := range top {
		switch f.num {
		case 1:
			header, err := fields(f.bytes)
			if err != nil {
				return out, err
			}
			for _, h := range header {
				if h.num == 3 {
					out.timestamp = int64(h.varint)
				}
			}
		case 2:
			entity, err := fields(f.bytes)
			if err != nil {
				return out, err
			}
			for _, e := range entity {
				if e.num != 4 {
					continue
				}
				p, err := decodePosition(e.bytes)
				if err != nil {
					return out, err
				}
				out.positions = append(out.positions, p)
			}
		}
	}
	return out, nil
}

func decodePosition(data []byte) (position, error) {
	p := position{direction: -1, stopSequence: -1}
	vp, err := fields(data)
	if err != nil {
		return p, err
	}
	for _, f := range vp {
		switch f.num {
		case 1:
			trip, err := fields(f.bytes)
			if err != nil {
				return p, err
			}
			for _, t := range trip {
				switch t.num {
				case 5:
					p.routeID = string(t.bytes)
				case 6:
					p.direction = int(t.varint)
				}
			}
		case 2:
			pos, err := fields(f.bytes)
			if err != nil {
				return p, err
			}
			for _, c := range pos {
				switch c.num {
				case 1:
					p.lat = c.float32()
				case 2:
					p.lon = c.float32()
				case 5:
					p.speed, p.hasSpeed = c.float32(), true
				}
			}
		case 3:
			p.stopSequence = int(f.varint)
		case 5:
			p.timestamp = int64(f.varint)
		case 7:
			p.stopID = string(f.bytes)
		case 8:
			vehicle, err := fields(f.bytes)
			if err != nil {
				return p, err
			}
			for _, v := range vehicle {
				switch v.num {
				case 1:
					p.vehicleID = string(v.bytes)
				case 2:
					p.label = string(v.bytes)
				}
			}
		}
	}
	return p, nil
}
