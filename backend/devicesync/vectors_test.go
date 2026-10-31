package devicesync

import (
	"bytes"
	"crypto/ecdh"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/vectors.json")

func seed(label string) []byte {
	sum := sha256.Sum256([]byte("cai-sync vectors|" + label))
	return sum[:]
}

func fixedKey(label string) *ecdh.PrivateKey {
	key, err := ecdh.P256().NewPrivateKey(seed(label))
	if err != nil {
		panic(err)
	}
	return key
}

type keyPair struct {
	Scalar string `json:"scalar"`
	PKCS8  string `json:"pkcs8"`
	SPKI   string `json:"spki"`
}

func pairOf(key *ecdh.PrivateKey) keyPair {
	pkcs8, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		panic(err)
	}
	return keyPair{b64.EncodeToString(key.Bytes()), b64.EncodeToString(pkcs8), b64.EncodeToString(spki(key))}
}

type sealedVector struct {
	Sender string `json:"sender,omitempty"`
	Name   string `json:"name,omitempty"`
	Stored string `json:"stored,omitempty"`
	Rev    int    `json:"rev,omitempty"`
	JSON   string `json:"json"`
	Bucket int    `json:"bucket"`
	Nonce  string `json:"nonce"`
	AAD    string `json:"aad"`
	Body   string `json:"body"`
}

type pairMode struct {
	T        string         `json:"t"`
	Check    string         `json:"check"`
	Messages []sealedVector `json:"messages"`
}

type vectorFile struct {
	About string `json:"about"`
	Pair  struct {
		Code    string   `json:"code"`
		K       string   `json:"k"`
		D       keyPair  `json:"d"`
		E       keyPair  `json:"e"`
		Commit  string   `json:"commit"`
		Shared  string   `json:"shared"`
		Scanned pairMode `json:"scanned"`
		Typed   pairMode `json:"typed"`
	} `json:"pair"`
	Chain struct {
		ID          string            `json:"id"`
		Epoch       int               `json:"epoch"`
		S           string            `json:"s"`
		Join        string            `json:"join"`
		JoinHash    string            `json:"joinHash"`
		Data        string            `json:"data"`
		Names       string            `json:"names"`
		RecordNames map[string]string `json:"recordNames"`
		Records     []sealedVector    `json:"records"`
	} `json:"chain"`
	Wrap struct {
		Chain     string  `json:"chain"`
		Epoch     int     `json:"epoch"`
		Device    string  `json:"device"`
		DeviceKey keyPair `json:"deviceKey"`
		Ephemeral keyPair `json:"ephemeral"`
		Nonce     string  `json:"nonce"`
		S         string  `json:"s"`
		Info      string  `json:"info"`
		Body      string  `json:"body"`
	} `json:"wrap"`
	Sizes struct {
		Buckets     []int `json:"buckets"`
		Bodies      []int `json:"bodies"`
		PairMessage int   `json:"pairMessage"`
		Wrap        int   `json:"wrap"`
		SPKI        int   `json:"spki"`
		WorstCity   int   `json:"worstCity"`
	} `json:"sizes"`
}

func sealPair(t []byte, code, sender, message, nonceLabel string) sealedVector {
	aad := "cai-sync/v1/pair-message|" + code + "|" + sender
	nonce := seed(nonceLabel)[:12]
	body := seal(t, nonce, pad([]byte(message), pairPadded), aad)
	return sealedVector{Sender: sender, JSON: message, Bucket: pairPadded, Nonce: b64.EncodeToString(nonce), AAD: aad, Body: b64.EncodeToString(body)}
}

func sealRecord(keys chainKeys, chain string, epoch int, logical string, rev int, record, nonceLabel string) sealedVector {
	name := recordName(keys.names, logical)
	aad := recordAAD(chain, epoch, name, rev)
	nonce := seed(nonceLabel)[:12]
	bucket := bucketFor(len(record))
	body := seal(keys.data, nonce, pad([]byte(record), bucket), aad)
	return sealedVector{Name: logical, Stored: name, Rev: rev, JSON: record, Bucket: bucket, Nonce: b64.EncodeToString(nonce), AAD: aad, Body: b64.EncodeToString(body)}
}

func buildVectors() vectorFile {
	var v vectorFile
	v.About = "Worked values for protocol.md, made by vectors_test.go from fixed seeds. Binary values are base64url without padding."

	code := "482913"
	k := seed("k")[:16]
	d, e := fixedKey("d"), fixedKey("e")
	pd, pe := spki(d), spki(e)
	commit := sha256.Sum256(pd)
	shared := ecdhWith(d, pe)
	v.Pair.Code, v.Pair.K = code, b64.EncodeToString(k)
	v.Pair.D, v.Pair.E = pairOf(d), pairOf(e)
	v.Pair.Commit, v.Pair.Shared = b64.EncodeToString(commit[:]), b64.EncodeToString(shared)

	s := seed("s")
	chain := b64.EncodeToString(seed("chain")[:idSize])
	scanned := pairKey(shared, k, code, pe, pd)
	v.Pair.Scanned = pairMode{T: b64.EncodeToString(scanned), Check: checkNumber(scanned), Messages: []sealedVector{
		sealPair(scanned, code, "E", `{"t":"hello","kind":"android","label":"Pixel 8","syncs":true,"devices":2}`, "pair-1"),
		sealPair(scanned, code, "D", `{"t":"hello","kind":"web","label":"Chrome on Windows","syncs":false,"devices":0}`, "pair-2"),
		sealPair(scanned, code, "D", `{"t":"ready"}`, "pair-3"),
		sealPair(scanned, code, "E", fmt.Sprintf(`{"t":"secret","chain":%q,"epoch":1,"s":%q}`, chain, b64.EncodeToString(s)), "pair-4"),
	}}
	typed := pairKey(shared, nil, code, pe, pd)
	v.Pair.Typed = pairMode{T: b64.EncodeToString(typed), Check: checkNumber(typed), Messages: []sealedVector{
		sealPair(typed, code, "E", `{"t":"hello","kind":"web","label":"Firefox on Linux","syncs":false,"devices":0}`, "pair-5"),
	}}

	keys := keysOf(s)
	joinHash := sha256.Sum256(keys.join)
	v.Chain.ID, v.Chain.Epoch, v.Chain.S = chain, 1, b64.EncodeToString(s)
	v.Chain.Join, v.Chain.JoinHash = b64.EncodeToString(keys.join), b64.EncodeToString(joinHash[:])
	v.Chain.Data, v.Chain.Names = b64.EncodeToString(keys.data), b64.EncodeToString(keys.names)
	v.Chain.RecordNames = map[string]string{}
	for _, logical := range []string{"settings", "devices", "city:cluj", "city:iasi"} {
		v.Chain.RecordNames[logical] = recordName(keys.names, logical)
	}
	device := b64.EncodeToString(seed("device")[:idSize])
	deviceKey := fixedKey("device-key")
	v.Chain.Records = []sealedVector{
		sealRecord(keys, chain, 1, "settings", 1, `{"v":1,"theme":"dark","locale":"ro","autoCenterOnMe":true,"autoFitMap":false,"showVehicleExtras":true,"regions":{"cluj":{"showGreenFriday":true}}}`, "record-1"),
		sealRecord(keys, chain, 1, "city:cluj", 3, `{"v":1,"stops":[391,12],"lines":[[24,"0"]],"places":[["Acasă","",46.77,23.6]],"followed":["24"]}`, "record-2"),
		sealRecord(keys, chain, 1, "devices", 2, fmt.Sprintf(`{"v":1,"devices":[{"id":%q,"kind":"android","label":"Pixel 8","pub":%q}]}`, device, b64.EncodeToString(spki(deviceKey))), "record-3"),
		sealRecord(keys, chain, 1, "city:iasi", 1, worstCity().write(), "record-4"),
	}

	ephemeral := fixedKey("ephemeral")
	next := seed("s2")
	nonce := seed("wrap-nonce")[:12]
	info := wrapInfo(chain, 2, device)
	v.Wrap.Chain, v.Wrap.Epoch, v.Wrap.Device = chain, 2, device
	v.Wrap.DeviceKey, v.Wrap.Ephemeral = pairOf(deviceKey), pairOf(ephemeral)
	v.Wrap.Nonce, v.Wrap.S, v.Wrap.Info = b64.EncodeToString(nonce), b64.EncodeToString(next), info
	v.Wrap.Body = b64.EncodeToString(wrap(ephemeral, spki(deviceKey), nonce, next, info))

	v.Sizes.Buckets = recordBuckets
	for _, b := range recordBuckets {
		v.Sizes.Bodies = append(v.Sizes.Bodies, b+sealing)
	}
	v.Sizes.PairMessage, v.Sizes.Wrap, v.Sizes.SPKI = pairMessage, wrapSize, spkiSize
	v.Sizes.WorstCity = len(worstCity().write())
	return v
}

// Every list full, every text at its limit in four-byte characters, every coordinate at full length.
func worstCity() cityRecord {
	wide := func(n int) string { return strings.Repeat("𝓐", n) }
	at := func(lo float64, i int) float64 { return math.Floor((lo+0.111111+float64(i)*0.010101)*1e6+0.5) / 1e6 }
	var rec cityRecord
	for i := range cityCap {
		rec.stops = append(rec.stops, int64(maxInt-i))
		rec.lines = append(rec.lines, line{int64(maxInt - i), "1"})
		rec.places = append(rec.places, place{name: wide(100), label: wide(40), lat: at(43.6, i), lon: at(20.2, i),
			origin: &origin{name: wide(100), lat: at(44.6, i), lon: at(21.2, i)}})
		rec.followed = append(rec.followed, wide(14)+fmt.Sprintf("%02d", i))
	}
	return rec
}

func TestVectors(t *testing.T) {
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(buildVectors()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", "vectors.json")
	if *update {
		if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(saved, out.Bytes()) {
		t.Fatal("testdata/vectors.json differs from what the reference makes; go test -run TestVectors -update rewrites it once the change is meant")
	}
}

func mustB64(t *testing.T, text string) []byte {
	t.Helper()
	raw, err := b64.DecodeString(text)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// The vectors read back: what a client does with them.
func TestVectorsOpen(t *testing.T) {
	v := buildVectors()
	d, e := fixedKey("d"), fixedKey("e")
	if !bytes.Equal(ecdhWith(e, spki(d)), mustB64(t, v.Pair.Shared)) {
		t.Fatal("ECDH differs by side")
	}
	for _, mode := range []pairMode{v.Pair.Scanned, v.Pair.Typed} {
		for _, m := range mode.Messages {
			body := mustB64(t, m.Body)
			if len(body) != pairMessage {
				t.Fatalf("message is %d bytes", len(body))
			}
			padded, err := open(mustB64(t, mode.T), body, m.AAD)
			if err != nil {
				t.Fatal(err)
			}
			if plain, err := unpad(padded); err != nil || string(plain) != m.JSON {
				t.Fatalf("message %q: %v", plain, err)
			}
		}
	}
	if v.Pair.Scanned.Check == v.Pair.Typed.Check {
		t.Fatal("the QR key does not reach the check number")
	}
	data := mustB64(t, v.Chain.Data)
	for _, r := range v.Chain.Records {
		body := mustB64(t, r.Body)
		if !validRecordBody(body) {
			t.Fatalf("%s: %d bytes", r.Name, len(body))
		}
		padded, err := open(data, body, r.AAD)
		if err != nil {
			t.Fatal(err)
		}
		if plain, err := unpad(padded); err != nil || string(plain) != r.JSON {
			t.Fatalf("%s: %v", r.Name, err)
		}
		if _, err := open(data, body, recordAAD(v.Chain.ID, v.Chain.Epoch, r.Stored, r.Rev-1)); err == nil {
			t.Fatalf("%s opens as an older revision", r.Name)
		}
	}
	body := mustB64(t, v.Wrap.Body)
	if len(body) != wrapSize {
		t.Fatalf("wrap is %d bytes", len(body))
	}
	if s, err := unwrap(fixedKey("device-key"), body, v.Wrap.Info); err != nil || !bytes.Equal(s, mustB64(t, v.Wrap.S)) {
		t.Fatalf("unwrap: %v", err)
	}
	if _, err := unwrap(fixedKey("d"), body, v.Wrap.Info); err == nil {
		t.Fatal("another device opens the wrap")
	}
}

func TestWorstCityFits(t *testing.T) {
	written := worstCity().write()
	if _, err := parseCity(written); err != nil {
		t.Fatal(err)
	}
	top := recordBuckets[len(recordBuckets)-1]
	if bucketFor(len(written)) != top {
		t.Fatalf("worst city is %d bytes, bucket %d", len(written), bucketFor(len(written)))
	}
}

type casesFile struct {
	About   string `json:"about"`
	Records []struct {
		Kind    string `json:"kind"`
		About   string `json:"about"`
		JSON    string `json:"json"`
		Outcome string `json:"outcome"`
	} `json:"records"`
	Merges []struct {
		Kind    string                     `json:"kind"`
		About   string                     `json:"about"`
		Base    *string                    `json:"base"`
		Local   string                     `json:"local"`
		Remote  string                     `json:"remote"`
		Merged  string                     `json:"merged"`
		Dropped map[string]json.RawMessage `json:"dropped"`
	} `json:"merges"`
}

func parseAny(kind, raw string) error {
	var err error
	switch kind {
	case "city":
		_, err = parseCity(raw)
	case "news":
		_, err = parseNews(raw)
	case "devices":
		_, err = parseDevices(raw)
	case "settings":
		_, err = parseSettings(raw)
	case "recents":
		_, err = parseRecents(raw)
	case "options":
		_, err = parseOptions(raw)
	default:
		err = fmt.Errorf("unknown kind %q", kind)
	}
	return err
}

func readCases(t *testing.T) casesFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases casesFile
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

func TestRecordCases(t *testing.T) {
	for _, c := range readCases(t).Records {
		err := parseAny(c.Kind, c.JSON)
		got := "ok"
		var r refused
		switch {
		case errors.Is(err, errNewer):
			got = "newer"
		case errors.As(err, &r):
			got = "refuse"
		case err != nil:
			t.Fatalf("%s: %v", c.About, err)
		}
		if got != c.Outcome {
			t.Errorf("%s %s: got %s (%v), want %s", c.Kind, c.About, got, err, c.Outcome)
		}
	}
}

func droppedJSON(t *testing.T, d cityDropped) map[string]json.RawMessage {
	t.Helper()
	out := map[string]json.RawMessage{}
	add := func(key, text string, n int) {
		if n > 0 {
			out[key] = json.RawMessage(text)
		}
	}
	add("stops", joined(d.stops, stopID), len(d.stops))
	add("lines", joined(d.lines, func(l line) string { return "[" + stopID(l.route) + "," + quote(l.dir) + "]" }), len(d.lines))
	add("places", joined(d.places, writePlace), len(d.places))
	add("followed", joined(d.followed, quote), len(d.followed))
	return out
}

func compact(t *testing.T, m map[string]json.RawMessage) map[string]string {
	t.Helper()
	out := map[string]string{}
	for k, v := range m {
		var buf bytes.Buffer
		if err := json.Compact(&buf, v); err != nil {
			t.Fatal(err)
		}
		out[k] = buf.String()
	}
	return out
}

func TestMergeCases(t *testing.T) {
	for _, c := range readCases(t).Merges {
		for _, raw := range []*string{c.Base, &c.Local, &c.Remote, &c.Merged} {
			if raw != nil {
				if err := parseAny(c.Kind, *raw); err != nil {
					t.Fatalf("%s: a record in the case is refused: %v", c.About, err)
				}
			}
		}
		var got string
		dropped := map[string]json.RawMessage{}
		switch c.Kind {
		case "city":
			var base cityRecord
			if c.Base != nil {
				base, _ = parseCity(*c.Base)
			}
			local, _ := parseCity(c.Local)
			remote, _ := parseCity(c.Remote)
			merged, d := mergeCity(base, local, remote)
			got, dropped = merged.write(), droppedJSON(t, d)
		case "news":
			var base newsRecord
			if c.Base != nil {
				base, _ = parseNews(*c.Base)
			}
			local, _ := parseNews(c.Local)
			remote, _ := parseNews(c.Remote)
			got = mergeNews(base, local, remote).write()
		case "settings":
			var base *settingsRecord
			if c.Base != nil {
				b, _ := parseSettings(*c.Base)
				base = &b
			}
			local, _ := parseSettings(c.Local)
			remote, _ := parseSettings(c.Remote)
			got = mergeSettings(base, local, remote).write()
		case "recents":
			var base recentsRecord
			if c.Base != nil {
				base, _ = parseRecents(*c.Base)
			}
			local, _ := parseRecents(c.Local)
			remote, _ := parseRecents(c.Remote)
			got = mergeRecents(base, local, remote).write()
		case "options":
			var base *optionsRecord
			if c.Base != nil {
				b, _ := parseOptions(*c.Base)
				base = &b
			}
			local, _ := parseOptions(c.Local)
			remote, _ := parseOptions(c.Remote)
			got = mergeOptions(base, local, remote).write()
		case "devices":
			var base []deviceEntry
			if c.Base != nil {
				base, _ = parseDevices(*c.Base)
			}
			local, _ := parseDevices(c.Local)
			remote, _ := parseDevices(c.Remote)
			merged, _ := mergeList(base, local, remote, deviceID, mergeDevice, deviceCap)
			got = writeDevices(merged)
		}
		if got != c.Merged {
			t.Errorf("%s %s:\n got  %s\n want %s", c.Kind, c.About, got, c.Merged)
		}
		if want := compact(t, c.Dropped); !reflect.DeepEqual(compact(t, dropped), want) {
			t.Errorf("%s %s: dropped %v, want %v", c.Kind, c.About, compact(t, dropped), want)
		}
	}
}
