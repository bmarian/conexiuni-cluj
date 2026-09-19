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
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

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
	t.Cleanup(func() { pushCfg, pushQueue = nil, nil })
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

func TestRecordRouteChangesQueuesPush(t *testing.T) {
	withTestDB(t)
	withTestPush(t)
	pushQueue = make(chan RouteChange, 1)

	items := []RouteChangeItem{{Kind: changeStopsAdded, Direction: "0", Stops: []string{"S9"}}}
	recordRouteChanges("25", routeChangeSourceStops, items)

	select {
	case change := <-pushQueue:
		if change.ID == 0 || change.RouteShortName != "25" || len(change.Changes) != 1 {
			t.Fatalf("queued %+v", change)
		}
	default:
		t.Fatal("new change was not queued for push")
	}

	recordRouteChanges("25", routeChangeSourceStops, items)
	if len(pushQueue) != 0 {
		t.Fatal("a repeated change must not notify again")
	}
}
