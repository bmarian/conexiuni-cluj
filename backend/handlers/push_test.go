package handlers

import (
	"bytes"
	"conexiuni-cluj/database"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

type testPushClient struct {
	sub  pushSubscriber
	key  *ecdh.PrivateKey
	auth []byte

	mu       sync.Mutex
	messages []pushMessage
	vapid    []string
}

func newTestPushClient(t *testing.T, locale string, status int) *testPushClient {
	t.Helper()
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, 16)
	_, _ = rand.Read(auth)
	client := &testPushClient{key: key, auth: auth}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var msg pushMessage
		if err := json.Unmarshal(decryptTestPush(t, body, key, auth), &msg); err != nil {
			t.Errorf("payload is not a push message: %v", err)
		}
		client.mu.Lock()
		client.messages = append(client.messages, msg)
		client.vapid = append(client.vapid, r.Header.Get("Authorization"))
		client.mu.Unlock()
		w.WriteHeader(status)
	}))
	t.Cleanup(server.Close)

	client.sub = pushSubscriber{
		endpoint: server.URL + "/push/" + t.Name(),
		p256dh:   base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()),
		auth:     base64.RawURLEncoding.EncodeToString(auth),
		locale:   locale,
	}
	return client
}

func (c *testPushClient) received() []pushMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]pushMessage(nil), c.messages...)
}

// decryptTestPush undoes RFC 8291 aes128gcm the way a browser would.
func decryptTestPush(t *testing.T, body []byte, key *ecdh.PrivateKey, auth []byte) []byte {
	t.Helper()
	salt := body[:16]
	idLen := int(body[20])
	senderKey := body[21 : 21+idLen]
	sender, err := ecdh.P256().NewPublicKey(senderKey)
	if err != nil {
		t.Fatal(err)
	}
	shared, err := key.ECDH(sender)
	if err != nil {
		t.Fatal(err)
	}
	info := append(append([]byte("WebPush: info\x00"), key.PublicKey().Bytes()...), senderKey...)
	ikm, _ := hkdf.Key(sha256.New, shared, auth, string(info), 32)
	cek, _ := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: aes128gcm\x00", 16)
	nonce, _ := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: nonce\x00", 12)
	block, _ := aes.NewCipher(cek)
	gcm, _ := cipher.NewGCM(block)
	plain, err := gcm.Open(nil, nonce, body[21+idLen:], nil)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	plain = bytes.TrimRight(plain, "\x00")
	if len(plain) == 0 || plain[len(plain)-1] != 0x02 {
		t.Fatalf("missing padding delimiter")
	}
	return plain[:len(plain)-1]
}

func withTestPush(t *testing.T) {
	t.Helper()
	private, public, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	pushCfg = &pushConfig{publicKey: public, privateKey: private, subject: siteOrigin}
	pushWake = make(chan struct{}, 1)
	t.Cleanup(func() { pushCfg, pushWake = nil, nil })
}

func followLines(t *testing.T, sub pushSubscriber, routes ...string) {
	t.Helper()
	if err := savePushSubscription(sub, routes); err != nil {
		t.Fatal(err)
	}
}

func TestCheckVAPIDKeys(t *testing.T) {
	private, public, _ := webpush.GenerateVAPIDKeys()
	_, otherPublic, _ := webpush.GenerateVAPIDKeys()
	trim := func(s string) string { return strings.TrimRight(s, "=") }

	if err := checkVAPIDKeys(trim(public), trim(private)); err != nil {
		t.Fatalf("matching pair rejected: %v", err)
	}
	if err := checkVAPIDKeys(trim(otherPublic), trim(private)); err == nil {
		t.Fatal("mismatched pair accepted")
	}
}

func TestValidPushEndpoint(t *testing.T) {
	cases := map[string]bool{
		"https://fcm.googleapis.com/fcm/send/abc":                 true,
		"https://updates.push.services.mozilla.com/wpush/v2/abc":  true,
		"https://wns2-par02p.notify.windows.com/w/?token=abc":     true,
		"https://web.push.apple.com/abc":                          true,
		"http://fcm.googleapis.com/fcm/send/abc":                  false,
		"https://fcm.googleapis.com:8443/fcm/send/abc":            false,
		"https://evilfcm.googleapis.com.example.com/x":            false,
		"https://notfcm.googleapis.com/x":                         false,
		"https://127.0.0.1/push":                                  false,
		"https://user@fcm.googleapis.com/fcm/send/abc":            false,
		"https://fcm.googleapis.com/" + strings.Repeat("a", 1100): false,
	}
	for endpoint, want := range cases {
		if got := validPushEndpoint(endpoint); got != want {
			t.Errorf("validPushEndpoint(%.60q) = %v, want %v", endpoint, got, want)
		}
	}
}

func TestSendRouteChangePushReachesOnlyFollowers(t *testing.T) {
	withTestDB(t)
	withTestPush(t)
	if _, err := database.DB.Exec(`INSERT INTO routes VALUES (12, 2, '25', 'A - B', 3, '', 'ff0000')`); err != nil {
		t.Fatal(err)
	}

	english := newTestPushClient(t, "en", http.StatusCreated)
	romanian := newTestPushClient(t, "ro", http.StatusCreated)
	other := newTestPushClient(t, "ro", http.StatusCreated)
	followLines(t, english.sub, "25", "35")
	followLines(t, romanian.sub, "25")
	followLines(t, other.sub, "35")

	sendRouteChangePush(RouteChange{
		ID: 7, RouteShortName: "25", Source: routeChangeSourceTimetable, DetectedAt: 1700000000000,
		Changes: []RouteChangeItem{
			{Kind: changeLater, Direction: "1", Days: []string{"weekdays", "sunday"}},
			{Kind: changeTripsAdded, Direction: "0", Days: []string{"saturday"}},
		},
	})

	got := english.received()
	want := pushMessage{
		Title:     "Line 25 · Timetable changed",
		Body:      "There are changes to the weekday, Saturday and Sunday timetables.",
		URL:       "/route/12/1?change=7",
		Tag:       "route-change-25",
		Timestamp: 1700000000000,
	}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("english follower got %+v", got)
	}
	if !strings.HasPrefix(english.vapid[0], "vapid t=") || !strings.HasSuffix(english.vapid[0], "k="+pushCfg.publicKey) {
		t.Fatalf("missing VAPID authorization: %q", english.vapid[0])
	}
	if got := romanian.received(); len(got) != 1 || got[0].Title != "Linia 25 · Orar modificat" ||
		got[0].Body != "Sunt schimbări la orarul din timpul săptămânii, de sâmbătă și de duminică." {
		t.Fatalf("romanian follower got %+v", got)
	}
	if got := other.received(); len(got) != 0 {
		t.Fatalf("line 35 follower should not hear about line 25, got %+v", got)
	}
}

func TestSendRouteChangePushDropsExpiredSubscriptions(t *testing.T) {
	withTestDB(t)
	withTestPush(t)

	gone := newTestPushClient(t, "en", http.StatusGone)
	followLines(t, gone.sub, "25")

	sendRouteChangePush(RouteChange{
		ID: 1, RouteShortName: "25", Source: routeChangeSourceStops,
		Changes: []RouteChangeItem{{Kind: changeStopsRemoved, Direction: "0", Stops: []string{"S3"}}},
	})

	var subs, routes int
	_ = database.DB.QueryRow(`SELECT COUNT(*) FROM push_subscriptions`).Scan(&subs)
	_ = database.DB.QueryRow(`SELECT COUNT(*) FROM push_subscription_routes`).Scan(&routes)
	if len(gone.received()) != 1 || subs != 0 || routes != 0 {
		t.Fatalf("want one attempt then cleanup, got %d attempts, %d subscriptions, %d routes", len(gone.received()), subs, routes)
	}
}

func pendingPushes(t *testing.T) []int64 {
	t.Helper()
	ids, err := queryRows(`SELECT id FROM route_changes WHERE push_pending = 1 ORDER BY id`, nil,
		func(rows *sql.Rows) (int64, error) {
			var id int64
			return id, rows.Scan(&id)
		})
	if err != nil {
		t.Fatal(err)
	}
	return ids
}

func stopsAdded(stop string) []RouteChangeItem {
	return []RouteChangeItem{{Kind: changeStopsAdded, Direction: "0", Stops: []string{stop}}}
}

func TestRecordRouteChangesLeavesPushPending(t *testing.T) {
	withTestDB(t)
	withTestPush(t)

	recordRouteChanges("25", routeChangeSourceStops, stopsAdded("S9"))
	if got := pendingPushes(t); len(got) != 1 {
		t.Fatalf("want the new change pending, got %v", got)
	}
	select {
	case <-pushWake:
	default:
		t.Fatal("recording a change should wake the sender")
	}

	recordRouteChanges("25", routeChangeSourceStops, stopsAdded("S9"))
	if got := pendingPushes(t); len(got) != 1 || len(pushWake) != 0 {
		t.Fatalf("a repeated change must not notify again, pending=%v", got)
	}
}

func TestRecordRouteChangesWithoutPushLeavesNothingPending(t *testing.T) {
	withTestDB(t)

	recordRouteChanges("25", routeChangeSourceStops, stopsAdded("S9"))
	if got := pendingPushes(t); len(got) != 0 {
		t.Fatalf("enabling push later must not replay old changes, pending=%v", got)
	}
}

func TestFlushPendingPushesSendsNewestChangePerLine(t *testing.T) {
	withTestDB(t)
	withTestPush(t)

	client := newTestPushClient(t, "en", http.StatusCreated)
	followLines(t, client.sub, "25", "35")

	recordRouteChanges("25", routeChangeSourceStops, stopsAdded("S1"))
	recordRouteChanges("35", routeChangeSourceStops, stopsAdded("S2"))
	recordRouteChanges("25", routeChangeSourceStops, []RouteChangeItem{
		{Kind: changeStopsRemoved, Direction: "1", Stops: []string{"S3", "S4"}},
	})
	if len(client.received()) != 0 {
		t.Fatal("recording must not send by itself")
	}

	flushPendingPushes()
	got := client.received()
	bodies := map[string]string{}
	for _, msg := range got {
		bodies[msg.Tag] = msg.Body
	}
	if len(got) != 2 || bodies["route-change-25"] != "Some stops were removed from the route." ||
		bodies["route-change-35"] != "A new stop was added to the route." {
		t.Fatalf("want the newest change for each line, got %+v", got)
	}
	if pending := pendingPushes(t); len(pending) != 0 {
		t.Fatalf("flushed changes still pending: %v", pending)
	}

	flushPendingPushes()
	if len(client.received()) != 2 {
		t.Fatal("a second flush must not resend")
	}
}

func TestPushWindow(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Bucharest")
	if err != nil {
		t.Skip(err)
	}
	at := func(day, hour, minute int) time.Time { return time.Date(2026, 10, day, hour, minute, 0, 0, loc) }

	for _, c := range []struct {
		t    time.Time
		want bool
	}{
		{at(20, 4, 0), false},
		{at(20, 7, 59), false},
		{at(20, 8, 0), true},
		{at(20, 14, 30), true},
		{at(20, 19, 59), true},
		{at(20, 20, 0), false},
		{at(20, 20, 1), false},
		{at(20, 22, 0), false},
	} {
		if got := inPushWindow(c.t); got != c.want {
			t.Errorf("inPushWindow(%s) = %v, want %v", c.t.Format("15:04"), got, c.want)
		}
	}

	if got := nextPushWindow(at(20, 4, 0)); !got.Equal(at(20, 8, 0)) {
		t.Errorf("after the 04:00 warmup, want 08:00 the same day, got %s", got)
	}
	if got := nextPushWindow(at(20, 23, 0)); !got.Equal(at(21, 8, 0)) {
		t.Errorf("late evening, want 08:00 the next day, got %s", got)
	}
	// Clocks go back on 25 October 2026.
	if got := nextPushWindow(at(24, 23, 30)); !got.Equal(at(25, 8, 0)) || got.Sub(at(24, 23, 30)) != 9*time.Hour+30*time.Minute {
		t.Errorf("across the DST change, want 08:00 local, got %s", got)
	}
}
