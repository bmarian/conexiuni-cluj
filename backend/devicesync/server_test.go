package devicesync

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

type harness struct {
	t   *testing.T
	s   *Service
	app *fiber.App
}

func newHarness(t *testing.T, opts Options) *harness {
	t.Helper()
	if opts.MaxBytes == 0 {
		opts.MaxBytes = 1 << 30
	}
	s, err := Open(filepath.Join(t.TempDir(), "sync.db"), opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	app := fiber.New()
	s.Register(app.Group("/api"))
	return &harness{t: t, s: s, app: app}
}

func (h *harness) do(method, path, body string, header ...string) (int, map[string]any) {
	h.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	resp, err := h.app.Test(req, fiber.TestConfig{Timeout: 0})
	if err != nil {
		h.t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, out
}

func (h *harness) want(status int, method, path, body string, header ...string) map[string]any {
	h.t.Helper()
	got, out := h.do(method, path, body, header...)
	if got != status {
		h.t.Fatalf("%s %s: %d %v, want %d", method, path, got, out, status)
	}
	return out
}

func enc(raw []byte) string { return b64.EncodeToString(raw) }

func (h *harness) openSlot(pd []byte) (string, string) {
	commit := sha256.Sum256(pd)
	out := h.want(200, "POST", "/api/sync/pair", fmt.Sprintf(`{"commit":%q}`, enc(commit[:])))
	return out["code"].(string), out["secret"].(string)
}

func TestPairingFlow(t *testing.T) {
	h := newHarness(t, Options{Cities: 1})
	d, e := fixedKey("d"), fixedKey("e")
	pd, pe := spki(d), spki(e)
	code, dSecret := h.openSlot(pd)
	path := "/api/sync/pair/" + code
	asD, claim := []string{pairSecretHeader, dSecret}, fmt.Sprintf(`{"pub":%q,"scanned":true}`, enc(pe))

	h.want(409, "PUT", path+"/pub", fmt.Sprintf(`{"pub":%q}`, enc(pd)), asD...)
	if out := h.want(200, "GET", path, "", asD...); out["pub"] != nil {
		t.Fatalf("a key before anyone claimed: %v", out)
	}
	out := h.want(200, "POST", path+"/claim", claim)
	asE := []string{pairSecretHeader, out["secret"].(string)}
	commit := sha256.Sum256(pd)
	if out["commit"] != enc(commit[:]) {
		t.Fatalf("commit %v", out["commit"])
	}
	h.want(404, "POST", path+"/claim", claim)

	if out := h.want(200, "GET", path, "", asD...); out["pub"] != enc(pe) || out["scanned"] != true {
		t.Fatalf("shower reads %v", out)
	}
	h.want(409, "POST", path+"/messages", fmt.Sprintf(`{"message":%q}`, enc(make([]byte, pairMessage))), asE...)
	h.want(400, "PUT", path+"/pub", fmt.Sprintf(`{"pub":%q}`, enc(pe)), asD...)
	h.want(409, "PUT", path+"/pub", fmt.Sprintf(`{"pub":%q}`, enc(pd)), asE...)
	h.want(204, "PUT", path+"/pub", fmt.Sprintf(`{"pub":%q}`, enc(pd)), asD...)
	h.want(409, "PUT", path+"/pub", fmt.Sprintf(`{"pub":%q}`, enc(pd)), asD...)
	if out := h.want(200, "GET", path, "", asE...); out["pub"] != enc(pd) {
		t.Fatalf("enterer reads %v", out)
	}

	key := pairKey(ecdhWith(e, pd), seed("k")[:16], code, pe, pd)
	hello := `{"t":"hello","kind":"web","label":"Firefox on Linux","syncs":false,"devices":0}`
	sealed := seal(key, randomBytes(12), pad([]byte(hello), pairPadded), "cai-sync/v1/pair-message|"+code+"|E")
	message := fmt.Sprintf(`{"message":%q}`, enc(sealed))
	h.want(204, "POST", path+"/messages", message, asE...)
	h.want(204, "POST", path+"/messages", message, asE...)
	h.want(409, "POST", path+"/messages", message, asE...)
	h.want(400, "POST", path+"/messages", fmt.Sprintf(`{"message":%q}`, enc(sealed[1:])), asD...)

	got := h.want(200, "GET", path, "", asD...)["messages"].([]any)
	if len(got) != 2 {
		t.Fatalf("%d messages", len(got))
	}
	raw, _ := b64.DecodeString(got[0].(string))
	theirs := pairKey(ecdhWith(d, pe), seed("k")[:16], code, pe, pd)
	padded, err := open(theirs, raw, "cai-sync/v1/pair-message|"+code+"|E")
	if err != nil {
		t.Fatal(err)
	}
	if plain, _ := unpad(padded); string(plain) != hello {
		t.Fatalf("hello %q", plain)
	}
	if out := h.want(200, "GET", path, "", asE...); len(out["messages"].([]any)) != 0 {
		t.Fatal("the enterer reads its own messages")
	}
	h.want(404, "GET", path, "", pairSecretHeader, "wrong")
	h.want(204, "DELETE", path, "", asD...)
	h.want(404, "GET", path, "", asE...)
}

func TestPairingExpires(t *testing.T) {
	h := newHarness(t, Options{Cities: 1})
	code, secret := h.openSlot(spki(fixedKey("d")))
	h.s.mu.Lock()
	h.s.slots[code].expires = time.Now().Add(-time.Second)
	h.s.mu.Unlock()
	h.want(404, "GET", "/api/sync/pair/"+code, "", pairSecretHeader, secret)
	h.want(404, "POST", "/api/sync/pair/"+code+"/claim", fmt.Sprintf(`{"pub":%q,"scanned":false}`, enc(spki(fixedKey("e")))))
}

func TestPairingBudgets(t *testing.T) {
	h := newHarness(t, Options{Cities: 1})
	pd := spki(fixedKey("d"))
	for range slotsPerClient {
		h.openSlot(pd)
	}
	commit := sha256.Sum256(pd)
	h.want(429, "POST", "/api/sync/pair", fmt.Sprintf(`{"commit":%q}`, enc(commit[:])))

	claim := fmt.Sprintf(`{"pub":%q,"scanned":false}`, enc(spki(fixedKey("e"))))
	for range missesPerClient {
		h.want(404, "POST", "/api/sync/pair/999999/claim", claim)
	}
	h.want(429, "POST", "/api/sync/pair/999999/claim", claim)

	h.s.mu.Lock()
	h.s.misses = newTiered(missesPerClient, clientWindow)
	h.s.globalMisses.spend("", missesPerMinute, time.Now())
	h.s.slotsMade = newTiered(slotsPerClient, clientWindow)
	for i := range maxOpenSlots - len(h.s.slots) {
		h.s.slots[fmt.Sprintf("x%d", i)] = &slot{expires: time.Now().Add(time.Hour)}
	}
	h.s.mu.Unlock()
	h.want(429, "POST", "/api/sync/pair/999999/claim", claim)
	h.want(503, "POST", "/api/sync/pair", fmt.Sprintf(`{"commit":%q}`, enc(commit[:])))
}

func TestStrictBodies(t *testing.T) {
	h := newHarness(t, Options{Cities: 1})
	commit := enc(make([]byte, 32))
	for _, body := range []string{
		``, `[]`, `{}`, `{"commit":null}`,
		fmt.Sprintf(`{"commit":%q,"x":1}`, commit),
		fmt.Sprintf(`{"commit":%q}{}`, commit),
		fmt.Sprintf(`{"commit":%q}`, commit+"=="),
		fmt.Sprintf(`{"commit":%q}`, enc(make([]byte, 31))),
		fmt.Sprintf(`{"commit":%q}`, strings.Repeat(" ", 200)),
	} {
		h.want(400, "POST", "/api/sync/pair", body)
	}
	h.want(200, "POST", "/api/sync/pair", fmt.Sprintf(` {"commit":%q} `, commit))
	notCurve := enc(spki(fixedKey("e")))
	notCurve = notCurve[:len(notCurve)-4] + "AAAA"
	h.want(400, "POST", "/api/sync/pair/123456/claim", fmt.Sprintf(`{"pub":%q,"scanned":false}`, notCurve))
	h.want(400, "POST", "/api/sync/pair/123456/claim", fmt.Sprintf(`{"pub":%q}`, enc(spki(fixedKey("e")))))
	if got, _ := h.do("POST", "/api/sync/pair", fmt.Sprintf(`{"commit":%q}`, commit), "Origin", "https://evil.example"); got != 403 {
		t.Fatalf("another site's page: %d", got)
	}
}

type device struct{ chain, id, token string }

func (h *harness) createChain(join []byte) device {
	out := h.want(200, "POST", "/api/sync/chains", fmt.Sprintf(`{"join":%q,"kind":"web"}`, enc(join)))
	return device{out["chain"].(string), out["device"].(string), out["token"].(string)}
}

func (h *harness) joinChain(chain string, join []byte) device {
	out := h.want(200, "POST", "/api/sync/chains/"+chain+"/devices", fmt.Sprintf(`{"join":%q,"kind":"android"}`, enc(join)))
	return device{chain, out["device"].(string), out["token"].(string)}
}

func (d device) auth() []string { return []string{"Authorization", "Bearer " + d.token} }

func (d device) path(rest string) string { return "/api/sync/chains/" + d.chain + rest }

func body(bucket int) string { return enc(make([]byte, bucket+sealing)) }

func TestChainDevices(t *testing.T) {
	h := newHarness(t, Options{Cities: 1})
	join := seed("join")
	first := h.createChain(join)
	h.want(401, "POST", first.path("/devices"), fmt.Sprintf(`{"join":%q,"kind":"web"}`, enc(seed("other"))))
	h.want(401, "POST", "/api/sync/chains/"+enc(seed("x")[:16])+"/devices", fmt.Sprintf(`{"join":%q,"kind":"web"}`, enc(join)))
	h.want(400, "POST", first.path("/devices"), fmt.Sprintf(`{"join":%q,"kind":"ios"}`, enc(join)))
	for range maxDevices - 1 {
		h.joinChain(first.chain, join)
	}
	if out := h.want(409, "POST", first.path("/devices"), fmt.Sprintf(`{"join":%q,"kind":"web"}`, enc(join))); out["error"] != "full" {
		t.Fatalf("%v", out)
	}
	state := h.want(200, "GET", first.path(""), "", first.auth()...)
	if devices := state["devices"].([]any); len(devices) != maxDevices || devices[0].(map[string]any)["id"] != first.id {
		t.Fatalf("devices %v", devices)
	}

	other := h.createChain(seed("join2"))
	h.want(401, "GET", first.path(""), "", other.auth()...)
	h.want(401, "GET", first.path(""), "", "Authorization", "Bearer "+enc(seed("nope")))
	h.want(401, "GET", first.path(""), "", "Authorization", first.token)
}

func TestAuthBudget(t *testing.T) {
	h := newHarness(t, Options{Cities: 1})
	first := h.createChain(seed("join"))
	for range authMissesPerClient {
		h.want(401, "GET", first.path(""), "", "Authorization", "Bearer "+enc(seed("nope")))
	}
	h.want(429, "GET", first.path(""), "", first.auth()...)
	h.want(429, "POST", first.path("/devices"), fmt.Sprintf(`{"join":%q,"kind":"web"}`, enc(seed("join"))))
}

func TestChainCreationLimits(t *testing.T) {
	h := newHarness(t, Options{Cities: 1})
	for i := range chainsPerClient {
		h.createChain(seed(fmt.Sprint(i)))
	}
	h.want(429, "POST", "/api/sync/chains", fmt.Sprintf(`{"join":%q,"kind":"web"}`, enc(seed("more"))))

	small := newHarness(t, Options{Cities: 1, MaxBytes: 1})
	small.want(503, "POST", "/api/sync/chains", fmt.Sprintf(`{"join":%q,"kind":"web"}`, enc(seed("join"))))
}

func TestRecords(t *testing.T) {
	h := newHarness(t, Options{Cities: 1})
	a := h.createChain(seed("join"))
	b := h.joinChain(a.chain, seed("join"))
	name := func(i int) string { return a.path("/records/" + enc(seed(fmt.Sprint("name", i))[:16])) }
	write := func(epoch, base int, bucket int) string {
		return fmt.Sprintf(`{"epoch":%d,"base":%d,"body":%q}`, epoch, base, body(bucket))
	}

	if out := h.want(200, "PUT", name(0), write(1, 0, 1024), a.auth()...); out["rev"] != 1.0 {
		t.Fatalf("%v", out)
	}
	if out := h.want(409, "PUT", name(0), write(1, 0, 4096), b.auth()...); out["error"] != "stale" || out["rev"] != 1.0 || out["epoch"] != 1.0 {
		t.Fatalf("%v", out)
	}
	h.want(409, "PUT", name(0), write(2, 1, 4096), b.auth()...)
	h.want(200, "PUT", name(0), write(1, 1, 65536), b.auth()...)
	if out := h.want(200, "GET", name(0), "", a.auth()...); out["rev"] != 2.0 || len(out["body"].(string)) != b64.EncodedLen(65536+sealing) {
		t.Fatalf("%v", out)
	}
	h.want(404, "GET", name(9), "", a.auth()...)
	for _, bad := range []string{
		fmt.Sprintf(`{"epoch":1,"base":2,"body":%q}`, enc(make([]byte, 1000))),
		fmt.Sprintf(`{"epoch":1,"base":-1,"body":%q}`, body(1024)),
		fmt.Sprintf(`{"epoch":1,"body":%q}`, body(1024)),
		fmt.Sprintf(`{"epoch":1,"base":2,"body":%q,"x":1}`, body(1024)),
	} {
		h.want(400, "PUT", name(0), bad, a.auth()...)
	}
	h.want(400, "PUT", a.path("/records/short"), write(1, 0, 1024), a.auth()...)

	h.want(200, "PUT", name(1), write(1, 0, 1024), a.auth()...)
	h.want(200, "PUT", name(2), write(1, 0, 1024), a.auth()...)
	if out := h.want(409, "PUT", name(3), write(1, 0, 1024), a.auth()...); out["error"] != "full" {
		t.Fatalf("%v", out)
	}
	if records := h.want(200, "GET", a.path(""), "", b.auth()...)["records"].([]any); len(records) != 3 {
		t.Fatalf("records %v", records)
	}
}

func TestWriteBudget(t *testing.T) {
	h := newHarness(t, Options{Cities: 1})
	a := h.createChain(seed("join"))
	path := a.path("/records/" + enc(seed("name")[:16]))
	for i := range writesPerDevice {
		h.want(200, "PUT", path, fmt.Sprintf(`{"epoch":1,"base":%d,"body":%q}`, i, body(1024)), a.auth()...)
	}
	h.want(429, "PUT", path, fmt.Sprintf(`{"epoch":1,"base":%d,"body":%q}`, writesPerDevice, body(1024)), a.auth()...)
}

func TestRotation(t *testing.T) {
	h := newHarness(t, Options{Cities: 1})
	a := h.createChain(seed("join"))
	b := h.joinChain(a.chain, seed("join"))
	c := h.joinChain(a.chain, seed("join"))
	oldName := enc(seed("old")[:16])
	old := a.path("/records/" + oldName)
	h.want(200, "PUT", old, fmt.Sprintf(`{"epoch":1,"base":0,"body":%q}`, body(1024)), a.auth()...)

	newName := enc(seed("new")[:16])
	wrapBody := wrap(fixedKey("ephemeral"), spki(fixedKey("device-key")), randomBytes(12), seed("s2"), wrapInfo(a.chain, 2, b.id))
	seenRev := 1
	rotation := func(epoch int, remove []string, wraps map[string][]byte) string {
		var w []string
		for id, body := range wraps {
			w = append(w, fmt.Sprintf(`{"device":%q,"body":%q}`, id, enc(body)))
		}
		removed, _ := json.Marshal(remove)
		return fmt.Sprintf(`{"epoch":%d,"seen":[{"name":%q,"rev":%d}],"join":%q,"remove":%s,"records":[{"name":%q,"body":%q}],"wraps":[%s]}`,
			epoch, oldName, seenRev, enc(seed("join-2")), removed, newName, body(4096), strings.Join(w, ","))
	}
	if out := h.want(409, "POST", a.path("/rotate"), rotation(2, []string{c.id}, map[string][]byte{b.id: wrapBody}), a.auth()...); out["error"] != "stale" {
		t.Fatalf("%v", out)
	}
	for _, bad := range []string{
		rotation(1, []string{a.id}, map[string][]byte{b.id: wrapBody, c.id: wrapBody}),
		rotation(1, []string{c.id}, map[string][]byte{}),
		rotation(1, []string{c.id}, map[string][]byte{b.id: wrapBody, c.id: wrapBody}),
		rotation(1, []string{c.id}, map[string][]byte{b.id: wrapBody, a.id: wrapBody}),
		rotation(1, []string{enc(seed("ghost")[:16])}, map[string][]byte{b.id: wrapBody, c.id: wrapBody}),
	} {
		if out := h.want(409, "POST", a.path("/rotate"), bad, a.auth()...); out["error"] != "devices" {
			t.Fatalf("%v", out)
		}
	}
	h.want(400, "POST", a.path("/rotate"), rotation(1, []string{c.id, c.id}, map[string][]byte{b.id: wrapBody}), a.auth()...)
	h.want(200, "PUT", old, fmt.Sprintf(`{"epoch":1,"base":1,"body":%q}`, body(1024)), b.auth()...)
	if out := h.want(409, "POST", a.path("/rotate"), rotation(1, []string{c.id}, map[string][]byte{b.id: wrapBody}), a.auth()...); out["error"] != "stale" {
		t.Fatalf("a record written since the caller read it: %v", out)
	}
	h.want(200, "PUT", old, fmt.Sprintf(`{"epoch":1,"base":2,"body":%q}`, body(1024)), b.auth()...)
	seenRev = 3
	missing := strings.Replace(rotation(1, []string{c.id}, map[string][]byte{b.id: wrapBody}), fmt.Sprintf(`{"name":%q,"rev":3}`, oldName), "", 1)
	h.want(409, "POST", a.path("/rotate"), missing, a.auth()...)
	h.want(400, "POST", a.path("/rotate"), rotation(1, []string{c.id}, map[string][]byte{b.id: wrapBody[1:]}), a.auth()...)

	if out := h.want(200, "POST", a.path("/rotate"), rotation(1, []string{c.id}, map[string][]byte{b.id: wrapBody}), a.auth()...); out["epoch"] != 2.0 {
		t.Fatalf("%v", out)
	}
	h.want(401, "GET", a.path(""), "", c.auth()...)
	h.want(401, "POST", a.path("/devices"), fmt.Sprintf(`{"join":%q,"kind":"web"}`, enc(seed("join"))))
	state := h.want(200, "GET", a.path(""), "", b.auth()...)
	if state["epoch"] != 2.0 || len(state["devices"].([]any)) != 2 {
		t.Fatalf("%v", state)
	}
	records := state["records"].([]any)
	if len(records) != 1 || records[0].(map[string]any)["name"] != newName || records[0].(map[string]any)["rev"] != 1.0 {
		t.Fatalf("records %v", records)
	}
	h.want(404, "GET", old, "", a.auth()...)
	out := h.want(200, "GET", a.path("/wrap"), "", b.auth()...)
	raw, _ := b64.DecodeString(out["body"].(string))
	if s, err := unwrap(fixedKey("device-key"), raw, wrapInfo(a.chain, 2, b.id)); err != nil || !bytes.Equal(s, seed("s2")) || out["epoch"] != 2.0 {
		t.Fatalf("wrap: %v", err)
	}
	h.want(404, "GET", a.path("/wrap"), "", a.auth()...)
	h.joinChain(a.chain, seed("join-2"))
}

func TestLeaveAndDelete(t *testing.T) {
	h := newHarness(t, Options{Cities: 1})
	a := h.createChain(seed("join"))
	b := h.joinChain(a.chain, seed("join"))
	h.want(204, "DELETE", a.path("/devices/self"), "", a.auth()...)
	h.want(401, "GET", a.path(""), "", a.auth()...)
	h.want(200, "GET", a.path(""), "", b.auth()...)
	h.want(204, "DELETE", a.path("/devices/self"), "", b.auth()...)
	h.want(401, "POST", a.path("/devices"), fmt.Sprintf(`{"join":%q,"kind":"web"}`, enc(seed("join"))))

	c := h.createChain(seed("join"))
	d := h.joinChain(c.chain, seed("join"))
	h.want(204, "DELETE", c.path(""), "", c.auth()...)
	h.want(401, "GET", c.path(""), "", d.auth()...)
}

func TestPruneUnused(t *testing.T) {
	h := newHarness(t, Options{Cities: 1})
	a := h.createChain(seed("join"))
	if err := h.s.pruneChains(time.Now().Add(unusedFor)); err != nil {
		t.Fatal(err)
	}
	h.want(200, "GET", a.path(""), "", a.auth()...)
	if err := h.s.pruneChains(time.Now().Add(unusedFor + 48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	h.want(401, "GET", a.path(""), "", a.auth()...)
}

func TestLogPath(t *testing.T) {
	for path, want := range map[string]string{
		"/api/sync/chains/abc":              "/api/sync/chains/-",
		"/api/sync/chains/abc/records/def":  "/api/sync/chains/-/records/-",
		"/api/sync/chains/abc/devices/self": "/api/sync/chains/-/devices/self",
		"/api/sync/chains":                  "/api/sync/chains",
		"/api/sync/pair/123456":             "/api/sync/pair/123456",
		"/api/cluj/live?stops=1":            "/api/cluj/live?stops=1",
		"/api/sync/chains/abc/wrap":         "/api/sync/chains/-/wrap",
	} {
		if got := LogPath(path); got != want {
			t.Errorf("%s: %s, want %s", path, got, want)
		}
	}
}

func TestPairingBudgetsCountTheNetworkToo(t *testing.T) {
	b := newTiered(2, time.Minute)
	now := time.Now()
	for i := range 2 * clientsPerNetwork {
		who := requester{client: fmt.Sprint("c", i/2), network: "2001:db8::"}
		if b.spent(who, now) {
			t.Fatalf("spent after %d", i)
		}
		b.spend(who, 1, now)
	}
	if !b.spent(requester{client: "fresh", network: "2001:db8::"}, now) {
		t.Fatal("a new /64 in a spent /48 still got through")
	}
	if b.spent(requester{client: "fresh", network: "2001:db9::"}, now) {
		t.Fatal("another network was refused")
	}
}

func TestAFullDatabaseStopsGrowth(t *testing.T) {
	h := newHarness(t, Options{Cities: 1})
	a := h.createChain(seed("join"))
	b := h.joinChain(a.chain, seed("join"))
	name := enc(seed("name0")[:16])
	path := a.path("/records/" + name)
	write := func(base, bucket int) string {
		return fmt.Sprintf(`{"epoch":1,"base":%d,"body":%q}`, base, body(bucket))
	}
	h.want(200, "PUT", path, write(0, 4096), a.auth()...)
	h.s.maxBytes = 1

	if out := h.want(409, "PUT", a.path("/records/"+enc(seed("name1")[:16])), write(0, 1024), a.auth()...); out["error"] != "full" {
		t.Fatalf("a new record: %v", out)
	}
	if out := h.want(409, "PUT", path, write(1, 16384), a.auth()...); out["error"] != "full" {
		t.Fatalf("a bigger record: %v", out)
	}
	h.want(200, "PUT", path, write(1, 4096), a.auth()...)
	h.want(200, "PUT", path, write(2, 1024), a.auth()...)
	h.want(503, "POST", "/api/sync/chains", fmt.Sprintf(`{"join":%q,"kind":"web"}`, enc(seed("other"))))

	rotate := func(bucket int) string {
		return fmt.Sprintf(`{"epoch":1,"seen":[{"name":%q,"rev":3}],"join":%q,"remove":[%q],"records":[{"name":%q,"body":%q}],"wraps":[]}`,
			name, enc(seed("join-2")), b.id, name, body(bucket))
	}
	if out := h.want(409, "POST", a.path("/rotate"), rotate(4096), a.auth()...); out["error"] != "full" {
		t.Fatalf("a growing rotation: %v", out)
	}
	h.want(200, "POST", a.path("/rotate"), rotate(1024), a.auth()...)
	h.want(401, "GET", a.path(""), "", b.auth()...)
}
