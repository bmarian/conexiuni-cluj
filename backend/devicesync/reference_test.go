package devicesync

// The client side of protocol.md in Go: what produced testdata/vectors.json and checks testdata/cases.json.

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

var zeroSalt = make([]byte, 32)

func hkdfKey(ikm, salt []byte, info string, n int) []byte {
	if len(salt) == 0 {
		salt = zeroSalt
	}
	key, err := hkdf.Key(sha256.New, ikm, salt, info, n)
	if err != nil {
		panic(err)
	}
	return key
}

func spki(key *ecdh.PrivateKey) []byte {
	raw, err := x509.MarshalPKIXPublicKey(key.PublicKey())
	if err != nil {
		panic(err)
	}
	return raw
}

func ecdhWith(own *ecdh.PrivateKey, other []byte) []byte {
	parsed, err := x509.ParsePKIXPublicKey(other)
	if err != nil {
		panic(err)
	}
	pub, err := ecdhKey(parsed)
	if err != nil {
		panic(err)
	}
	shared, err := own.ECDH(pub)
	if err != nil {
		panic(err)
	}
	return shared
}

func gcm(key []byte) cipher.AEAD {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		panic(err)
	}
	return aead
}

func seal(key, nonce, plain []byte, aad string) []byte {
	return gcm(key).Seal(slices.Clone(nonce), nonce, plain, []byte(aad))
}

func open(key, sealed []byte, aad string) ([]byte, error) {
	if len(sealed) < sealing {
		return nil, errors.New("short")
	}
	return gcm(key).Open(nil, sealed[:12], sealed[12:], []byte(aad))
}

func pad(plain []byte, bucket int) []byte {
	out := make([]byte, bucket)
	binary.BigEndian.PutUint32(out, uint32(len(plain)))
	copy(out[4:], plain)
	return out
}

func unpad(padded []byte) ([]byte, error) {
	if len(padded) < 4 {
		return nil, errors.New("short")
	}
	n := int(binary.BigEndian.Uint32(padded))
	if n > len(padded)-4 {
		return nil, errors.New("length")
	}
	for _, b := range padded[4+n:] {
		if b != 0 {
			return nil, errors.New("padding")
		}
	}
	return padded[4 : 4+n], nil
}

func bucketFor(n int) int {
	for _, b := range recordBuckets {
		if n+4 <= b {
			return b
		}
	}
	return 0
}

func pairKey(shared, k []byte, code string, pe, pd []byte) []byte {
	info := append([]byte("cai-sync/v1/pair|"+code+"|"), pe...)
	return hkdfKey(shared, k, string(append(info, pd...)), 32)
}

func checkNumber(t []byte) string {
	return fmt.Sprintf("%06d", binary.BigEndian.Uint32(hkdfKey(t, nil, "cai-sync/v1/check", 4))%1_000_000)
}

type chainKeys struct{ join, data, names []byte }

func keysOf(s []byte) chainKeys {
	salt := []byte("cai-sync/v1")
	return chainKeys{
		join:  hkdfKey(s, salt, "cai-sync/v1/join", 32),
		data:  hkdfKey(s, salt, "cai-sync/v1/data", 32),
		names: hkdfKey(s, salt, "cai-sync/v1/names", 32),
	}
}

func recordName(names []byte, logical string) string {
	mac := hmac.New(sha256.New, names)
	mac.Write([]byte(logical))
	return b64.EncodeToString(mac.Sum(nil)[:idSize])
}

func recordAAD(chain string, epoch int, name string, rev int) string {
	return fmt.Sprintf("cai-sync/v1/record|%s|%d|%s|%d", chain, epoch, name, rev)
}

func wrapInfo(chain string, epoch int, device string) string {
	return fmt.Sprintf("cai-sync/v1/wrap|%s|%d|%s", chain, epoch, device)
}

func wrap(ephemeral *ecdh.PrivateKey, devicePub []byte, nonce, s []byte, info string) []byte {
	px := spki(ephemeral)
	w := hkdfKey(ecdhWith(ephemeral, devicePub), px, info, 32)
	return append(px, seal(w, nonce, s, info)...)
}

func unwrap(device *ecdh.PrivateKey, body []byte, info string) ([]byte, error) {
	px := body[:spkiSize]
	return open(hkdfKey(ecdhWith(device, px), px, info, 32), body[spkiSize:], info)
}

// Records: the rules of protocol.md §4, as a parser and a canonical writer.

var errNewer = errors.New("newer")

type refused string

func (r refused) Error() string { return string(r) }

func refuse(format string, args ...any) error { return refused(fmt.Sprintf(format, args...)) }

type origin struct {
	name     string
	lat, lon float64
}

type place struct {
	name, label string
	lat, lon    float64
	origin      *origin
}

type line struct {
	route int64
	dir   string
}

type cityRecord struct {
	stops    []int64
	lines    []line
	places   []place
	followed []string
}

type deviceEntry struct{ id, kind, label, pub string }

type settingsRecord struct {
	theme, locale                             string
	autoCenterOnMe, autoFitMap, vehicleExtras bool
	greenFriday                               bool
}

const (
	cityCap   = 50
	deviceCap = 10
	maxInt    = math.MaxInt32
)

func forbidden(r rune) bool {
	switch {
	case r <= 0x1F, r >= 0x7F && r <= 0x9F, r == 0x061C, r == 0x200B, r == 0x200E, r == 0x200F,
		r >= 0x2028 && r <= 0x202E, r == 0x2060, r >= 0x2066 && r <= 0x2069, r == 0xFEFF, r == 0xFFFE, r == 0xFFFF,
		r >= 0xD800 && r <= 0xDFFF, r == 0xFFFD:
		return true
	}
	return false
}

func blankEnd(r rune) bool {
	return r == 0x20 || r == 0xA0 || r == 0x1680 || (r >= 0x2000 && r <= 0x200A) || r == 0x202F || r == 0x205F || r == 0x3000
}

func text(v any, minLen, maxLen int) (string, error) {
	s, ok := v.(string)
	if !ok {
		return "", refuse("not text: %v", v)
	}
	n := utf8.RuneCountInString(s)
	if n < minLen || n > maxLen {
		return "", refuse("text length %d", n)
	}
	for _, r := range s {
		if forbidden(r) {
			return "", refuse("forbidden character U+%04X", r)
		}
	}
	if n > 0 {
		first, _ := utf8.DecodeRuneInString(s)
		last, _ := utf8.DecodeLastRuneInString(s)
		if blankEnd(first) || blankEnd(last) {
			return "", refuse("blank at an end")
		}
	}
	return s, nil
}

func number(v any) (float64, error) {
	n, ok := v.(json.Number)
	if !ok {
		return 0, refuse("not a number: %v", v)
	}
	f, err := strconv.ParseFloat(string(n), 64)
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		return 0, refuse("bad number %s", n)
	}
	return f, nil
}

func integer(v any, lo, hi int64) (int64, error) {
	f, err := number(v)
	if err != nil {
		return 0, err
	}
	if f != math.Trunc(f) || f < float64(lo) || f > float64(hi) {
		return 0, refuse("integer out of range: %v", v)
	}
	return int64(f), nil
}

func coordinate(v any, lo, hi float64) (float64, error) {
	f, err := number(v)
	if err != nil {
		return 0, err
	}
	if f < lo || f > hi || math.Floor(f*1e6+0.5)/1e6 != f {
		return 0, refuse("bad coordinate %v", v)
	}
	return f, nil
}

func boolean(v any) (bool, error) {
	b, ok := v.(bool)
	if !ok {
		return false, refuse("not a boolean: %v", v)
	}
	return b, nil
}

func list(v any, limit int) ([]any, error) {
	items, ok := v.([]any)
	if !ok {
		return nil, refuse("not a list")
	}
	if len(items) > limit {
		return nil, refuse("%d items, cap %d", len(items), limit)
	}
	return items, nil
}

func object(v any, keys ...string) (map[string]any, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, refuse("not an object")
	}
	if len(m) != len(keys) {
		return nil, refuse("keys %v", slices.Sorted(maps.Keys(m)))
	}
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			return nil, refuse("missing %s", k)
		}
	}
	return m, nil
}

func parseJSON(raw string) (any, error) {
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, refuse("json: %v", err)
	}
	if dec.More() {
		return nil, refuse("trailing data")
	}
	return v, nil
}

// version answers errNewer for a later version, so the caller leaves the record alone.
func version(v any) error {
	m, ok := v.(map[string]any)
	if !ok {
		return refuse("not an object")
	}
	n, err := integer(m["v"], 1, maxInt)
	if err != nil {
		return err
	}
	if n > 1 {
		return errNewer
	}
	return nil
}

func unique[T any](items []T, id func(T) string) error {
	seen := map[string]bool{}
	for _, item := range items {
		key := id(item)
		if seen[key] {
			return refuse("duplicate %s", key)
		}
		seen[key] = true
	}
	return nil
}

func parseCity(raw string) (cityRecord, error) {
	var rec cityRecord
	v, err := parseJSON(raw)
	if err != nil {
		return rec, err
	}
	if err := version(v); err != nil {
		return rec, err
	}
	m, err := object(v, "v", "stops", "lines", "places", "followed")
	if err != nil {
		return rec, err
	}
	stops, err := list(m["stops"], cityCap)
	if err != nil {
		return rec, err
	}
	for _, s := range stops {
		id, err := integer(s, 1, maxInt)
		if err != nil {
			return rec, err
		}
		rec.stops = append(rec.stops, id)
	}
	lines, err := list(m["lines"], cityCap)
	if err != nil {
		return rec, err
	}
	for _, l := range lines {
		pair, err := list(l, 2)
		if err != nil || len(pair) != 2 {
			return rec, refuse("line is not a pair")
		}
		route, err := integer(pair[0], 1, maxInt)
		if err != nil {
			return rec, err
		}
		dir, ok := pair[1].(string)
		if !ok || (dir != "0" && dir != "1") {
			return rec, refuse("bad direction")
		}
		rec.lines = append(rec.lines, line{route, dir})
	}
	places, err := list(m["places"], cityCap)
	if err != nil {
		return rec, err
	}
	for _, p := range places {
		fields, err := list(p, 7)
		if err != nil || (len(fields) != 4 && len(fields) != 7) {
			return rec, refuse("place is not 4 or 7 fields")
		}
		var pl place
		if pl.name, err = text(fields[0], 1, 100); err != nil {
			return rec, err
		}
		if pl.label, err = text(fields[1], 0, 40); err != nil {
			return rec, err
		}
		if pl.lat, err = coordinate(fields[2], 43.6, 48.3); err != nil {
			return rec, err
		}
		if pl.lon, err = coordinate(fields[3], 20.2, 29.8); err != nil {
			return rec, err
		}
		if len(fields) == 7 {
			o := &origin{}
			if o.name, err = text(fields[4], 1, 100); err != nil {
				return rec, err
			}
			if o.lat, err = coordinate(fields[5], 43.6, 48.3); err != nil {
				return rec, err
			}
			if o.lon, err = coordinate(fields[6], 20.2, 29.8); err != nil {
				return rec, err
			}
			pl.origin = o
		}
		rec.places = append(rec.places, pl)
	}
	followed, err := list(m["followed"], cityCap)
	if err != nil {
		return rec, err
	}
	for _, f := range followed {
		name, err := text(f, 1, 16)
		if err != nil {
			return rec, err
		}
		rec.followed = append(rec.followed, name)
	}
	if err := errors.Join(unique(rec.stops, stopID), unique(rec.lines, lineID), unique(rec.places, placeID), unique(rec.followed, followedID)); err != nil {
		return rec, err
	}
	return rec, canonical(raw, rec.write())
}

func parseDevices(raw string) ([]deviceEntry, error) {
	v, err := parseJSON(raw)
	if err != nil {
		return nil, err
	}
	if err := version(v); err != nil {
		return nil, err
	}
	m, err := object(v, "v", "devices")
	if err != nil {
		return nil, err
	}
	items, err := list(m["devices"], deviceCap)
	if err != nil {
		return nil, err
	}
	var out []deviceEntry
	for _, item := range items {
		d, err := object(item, "id", "kind", "label", "pub")
		if err != nil {
			return nil, err
		}
		var e deviceEntry
		var ok bool
		if e.id, ok = d["id"].(string); !ok || !validID(e.id) {
			return nil, refuse("bad device id")
		}
		if e.kind, ok = d["kind"].(string); !ok || !slices.Contains(deviceKinds, e.kind) {
			return nil, refuse("bad kind")
		}
		if e.label, err = text(d["label"], 1, 40); err != nil {
			return nil, err
		}
		if e.pub, ok = d["pub"].(string); !ok {
			return nil, refuse("bad key")
		}
		if _, ok := p256(e.pub); !ok {
			return nil, refuse("bad key")
		}
		out = append(out, e)
	}
	if err := unique(out, deviceID); err != nil {
		return nil, err
	}
	return out, canonical(raw, writeDevices(out))
}

func parseSettings(raw string) (settingsRecord, error) {
	var rec settingsRecord
	v, err := parseJSON(raw)
	if err != nil {
		return rec, err
	}
	if err := version(v); err != nil {
		return rec, err
	}
	m, err := object(v, "v", "theme", "locale", "autoCenterOnMe", "autoFitMap", "showVehicleExtras", "regions")
	if err != nil {
		return rec, err
	}
	var ok bool
	if rec.theme, ok = m["theme"].(string); !ok || !slices.Contains([]string{"light", "dark", "system"}, rec.theme) {
		return rec, refuse("bad theme")
	}
	if rec.locale, ok = m["locale"].(string); !ok || (rec.locale != "ro" && rec.locale != "en") {
		return rec, refuse("bad locale")
	}
	if rec.autoCenterOnMe, err = boolean(m["autoCenterOnMe"]); err != nil {
		return rec, err
	}
	if rec.autoFitMap, err = boolean(m["autoFitMap"]); err != nil {
		return rec, err
	}
	if rec.vehicleExtras, err = boolean(m["showVehicleExtras"]); err != nil {
		return rec, err
	}
	regions, err := object(m["regions"], "cluj")
	if err != nil {
		return rec, err
	}
	cluj, err := object(regions["cluj"], "showGreenFriday")
	if err != nil {
		return rec, err
	}
	if rec.greenFriday, err = boolean(cluj["showGreenFriday"]); err != nil {
		return rec, err
	}
	return rec, canonical(raw, rec.write())
}

func canonical(raw, written string) error {
	if raw != written {
		return refuse("not canonical: %s", written)
	}
	return nil
}

func stopID(id int64) string        { return strconv.FormatInt(id, 10) }
func lineID(l line) string          { return stopID(l.route) + "/" + l.dir }
func followedID(name string) string { return name }
func deviceID(d deviceEntry) string { return d.id }

func placeID(p place) string {
	if p.origin == nil {
		return coord(p.lat) + "," + coord(p.lon)
	}
	return coord(p.lat) + "," + coord(p.lon) + "," + coord(p.origin.lat) + "," + coord(p.origin.lon)
}

func coord(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

func quote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

func joined[T any](items []T, write func(T) string) string {
	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = write(item)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func (r cityRecord) write() string {
	return `{"v":1,"stops":` + joined(r.stops, stopID) +
		`,"lines":` + joined(r.lines, func(l line) string { return "[" + stopID(l.route) + "," + quote(l.dir) + "]" }) +
		`,"places":` + joined(r.places, writePlace) +
		`,"followed":` + joined(r.followed, quote) + "}"
}

func writePlace(p place) string {
	out := "[" + quote(p.name) + "," + quote(p.label) + "," + coord(p.lat) + "," + coord(p.lon)
	if p.origin != nil {
		out += "," + quote(p.origin.name) + "," + coord(p.origin.lat) + "," + coord(p.origin.lon)
	}
	return out + "]"
}

func writeDevices(devices []deviceEntry) string {
	return `{"v":1,"devices":` + joined(devices, func(d deviceEntry) string {
		return `{"id":` + quote(d.id) + `,"kind":` + quote(d.kind) + `,"label":` + quote(d.label) + `,"pub":` + quote(d.pub) + "}"
	}) + "}"
}

func (r settingsRecord) write() string {
	return `{"v":1,"theme":` + quote(r.theme) + `,"locale":` + quote(r.locale) +
		`,"autoCenterOnMe":` + strconv.FormatBool(r.autoCenterOnMe) + `,"autoFitMap":` + strconv.FormatBool(r.autoFitMap) +
		`,"showVehicleExtras":` + strconv.FormatBool(r.vehicleExtras) +
		`,"regions":{"cluj":{"showGreenFriday":` + strconv.FormatBool(r.greenFriday) + "}}}"
}

// Merging: protocol.md §6.

// pick is the three-way rule for one field; base is nil for an item new on both sides.
func pick[T comparable](base *T, local, remote T) T {
	if base != nil && local != *base && remote == *base {
		return local
	}
	return remote
}

func mergeList[T any](base, local, remote []T, id func(T) string, fields func(base *T, local, remote T) T, limit int) (merged, dropped []T) {
	index := func(items []T) map[string]int {
		m := make(map[string]int, len(items))
		for i, item := range items {
			m[id(item)] = i
		}
		return m
	}
	b, l, r := index(base), index(local), index(remote)
	kept := func(key string) bool {
		_, inB := b[key]
		_, inL := l[key]
		_, inR := r[key]
		return (inL && inR) || (inL && !inB) || (inR && !inB)
	}
	orderOf := func(items []T, other map[string]int) []string {
		var out []string
		for _, item := range items {
			if _, ok := other[id(item)]; ok {
				out = append(out, id(item))
			}
		}
		return out
	}
	localMoved := !slices.Equal(orderOf(local, b), orderOf(base, l))
	remoteMoved := !slices.Equal(orderOf(remote, b), orderOf(base, r))
	win, other := remote, local
	if localMoved && !remoteMoved {
		win, other = local, remote
	}
	var keys []string
	placed := map[string]bool{}
	for _, item := range win {
		if key := id(item); kept(key) {
			keys = append(keys, key)
			placed[key] = true
		}
	}
	for _, item := range other {
		if key := id(item); kept(key) && !placed[key] {
			keys = append(keys, key)
		}
	}
	for _, key := range keys {
		li, inL := l[key]
		ri, inR := r[key]
		bi, inB := b[key]
		var item T
		switch {
		case inL && inR && inB:
			item = fields(&base[bi], local[li], remote[ri])
		case inL && inR:
			item = fields(nil, local[li], remote[ri])
		case inL:
			item = local[li]
		default:
			item = remote[ri]
		}
		merged = append(merged, item)
	}
	for i := len(merged) - 1; i >= 0 && len(merged) > limit; i-- {
		key := id(merged[i])
		if _, inR := r[key]; !inR {
			dropped = append([]T{merged[i]}, dropped...)
			merged = slices.Delete(merged, i, i+1)
		}
	}
	return merged, dropped
}

func same[T any](_ *T, _, remote T) T { return remote }

func mergePlace(base *place, local, remote place) place {
	out := remote
	var bName, bLabel *string
	if base != nil {
		bName, bLabel = &base.name, &base.label
	}
	out.name = pick(bName, local.name, remote.name)
	out.label = pick(bLabel, local.label, remote.label)
	return out
}

func mergeDevice(base *deviceEntry, local, remote deviceEntry) deviceEntry {
	if base == nil {
		return remote
	}
	return deviceEntry{
		id:    remote.id,
		kind:  pick(&base.kind, local.kind, remote.kind),
		label: pick(&base.label, local.label, remote.label),
		pub:   pick(&base.pub, local.pub, remote.pub),
	}
}

type cityDropped struct {
	stops    []int64
	lines    []line
	places   []place
	followed []string
}

func mergeCity(base, local, remote cityRecord) (cityRecord, cityDropped) {
	var out cityRecord
	var dropped cityDropped
	out.stops, dropped.stops = mergeList(base.stops, local.stops, remote.stops, stopID, same, cityCap)
	out.lines, dropped.lines = mergeList(base.lines, local.lines, remote.lines, lineID, same, cityCap)
	out.places, dropped.places = mergeList(base.places, local.places, remote.places, placeID, mergePlace, cityCap)
	out.followed, dropped.followed = mergeList(base.followed, local.followed, remote.followed, followedID, same, cityCap)
	return out, dropped
}

func mergeSettings(base *settingsRecord, local, remote settingsRecord) settingsRecord {
	if base == nil {
		return remote
	}
	return settingsRecord{
		theme:          pick(&base.theme, local.theme, remote.theme),
		locale:         pick(&base.locale, local.locale, remote.locale),
		autoCenterOnMe: pick(&base.autoCenterOnMe, local.autoCenterOnMe, remote.autoCenterOnMe),
		autoFitMap:     pick(&base.autoFitMap, local.autoFitMap, remote.autoFitMap),
		vehicleExtras:  pick(&base.vehicleExtras, local.vehicleExtras, remote.vehicleExtras),
		greenFriday:    pick(&base.greenFriday, local.greenFriday, remote.greenFriday),
	}
}
