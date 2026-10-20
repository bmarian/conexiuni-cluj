package router

import (
	"encoding/binary"
	"testing"
)

func pbBytes(b []byte, field int, data []byte) []byte {
	b = binary.AppendUvarint(b, uint64(field)<<3|2)
	return append(binary.AppendUvarint(b, uint64(len(data))), data...)
}

func pbVarint(b []byte, field int, v uint64) []byte {
	return binary.AppendUvarint(binary.AppendUvarint(b, uint64(field)<<3), v)
}

func TestPlainNodesKeepTheirTags(t *testing.T) {
	strs := pbBytes(nil, 1, nil)
	for _, s := range []string{"amenity", "cafe", "name", "Cafeneaua"} {
		strs = pbBytes(strs, 1, []byte(s))
	}
	var node []byte
	node = pbVarint(node, 1, 2*42)
	node = pbBytes(node, 2, []byte{1, 3})
	node = pbBytes(node, 3, []byte{2, 4})
	node = pbVarint(node, 8, 2*460000000)
	node = pbVarint(node, 9, 2*240000000)
	data := pbBytes(nil, 1, strs)
	data = pbBytes(data, 2, pbBytes(nil, 1, node))

	block := decodeBlock(pbBytes(nil, 1, data), pbfFilter{
		node:  func(int64, float64, float64) (bool, bool) { return true, true },
		way:   func(int64, func(string) string) (string, bool) { return "", false },
		place: placeOf,
	})
	if block.err != nil {
		t.Fatal(block.err)
	}
	if len(block.places) != 1 || block.places[0].name != "Cafeneaua" || block.places[0].node != 42 {
		t.Fatalf("places = %+v", block.places)
	}
}
