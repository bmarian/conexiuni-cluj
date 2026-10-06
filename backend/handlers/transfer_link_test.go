package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

const testClientHeader = "X-Test-Client"

func newTransferApp(t *testing.T) *fiber.App {
	t.Helper()
	transferLinksMu.Lock()
	transferLinks = map[string]*transferLink{}
	transferClientMisses = map[string]*transferCounter{}
	transferCreates = map[string]*transferCounter{}
	transferMisses = transferCounter{}
	transferLinksMu.Unlock()
	app := fiber.New()
	// Stands in for StatsMiddleware, which tells clients apart in production.
	app.Use(func(c fiber.Ctx) error {
		c.Locals(clientHashLocalKey, c.Get(testClientHeader))
		return c.Next()
	})
	app.Post("/transfer", CreateTransferLink)
	app.Put("/transfer/:code", SendToTransferLink)
	app.Get("/transfer/:code", ReadTransferLink)
	app.Delete("/transfer/:code", DeleteTransferLink)
	return app
}

type transferCall struct {
	method, path, secret, client, body string
}

func transferRequest(t *testing.T, app *fiber.App, call transferCall) (int, string) {
	t.Helper()
	req := httptest.NewRequest(call.method, call.path, strings.NewReader(call.body))
	if call.body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if call.secret != "" {
		req.Header.Set(transferLinkSecretHeader, call.secret)
	}
	if call.client != "" {
		req.Header.Set(testClientHeader, call.client)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(data)
}

func createTransferLink(t *testing.T, app *fiber.App, client string) (code, secret string) {
	t.Helper()
	status, body := transferRequest(t, app, transferCall{method: http.MethodPost, path: "/transfer", client: client})
	if status != http.StatusOK {
		t.Fatalf("create: status %d", status)
	}
	var created struct {
		Code   string `json:"code"`
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal([]byte(body), &created); err != nil || !validTransferCode(created.Code) || created.Secret == "" {
		t.Fatalf("create: bad reply %q", body)
	}
	return created.Code, created.Secret
}

func wrongCode(code string) string {
	if code == "000000" {
		return "000001"
	}
	return "000000"
}

func TestTransferLinkHandsTheExportToTheReceiver(t *testing.T) {
	app := newTransferApp(t)
	code, secret := createTransferLink(t, app, "")
	path := "/transfer/" + code

	if status, _ := transferRequest(t, app, transferCall{method: http.MethodGet, path: path, secret: secret}); status != http.StatusNoContent {
		t.Fatalf("before sending: status %d, want 204", status)
	}

	sent := `{"version":1,"settings":{"locale":"en"},"favorites":{"stops":[391,12],"routes":[{"routeId":7,"direction":"1"}]},"followedLines":["25"]}`
	if status, _ := transferRequest(t, app, transferCall{method: http.MethodPut, path: path, body: "  " + sent + "\n"}); status != http.StatusNoContent {
		t.Fatalf("send: status %d", status)
	}

	status, body := transferRequest(t, app, transferCall{method: http.MethodGet, path: path, secret: secret})
	if status != http.StatusOK || body != sent {
		t.Fatalf("read: status %d, body %s", status, body)
	}

	if status, _ := transferRequest(t, app, transferCall{method: http.MethodGet, path: path, secret: "wrong"}); status != http.StatusNotFound {
		t.Fatalf("wrong secret: status %d, want 404", status)
	}
	if status, _ := transferRequest(t, app, transferCall{method: http.MethodDelete, path: path, secret: secret}); status != http.StatusNoContent {
		t.Fatalf("delete: status %d", status)
	}
	if status, _ := transferRequest(t, app, transferCall{method: http.MethodGet, path: path, secret: secret}); status != http.StatusNotFound {
		t.Fatalf("after delete: status %d, want 404", status)
	}
}

func TestTransferLinkTakesOnlySmallJSONObjects(t *testing.T) {
	app := newTransferApp(t)
	code, _ := createTransferLink(t, app, "")
	path := "/transfer/" + code
	for _, body := range []string{`[1,2]`, `{"a":`, `"text"`} {
		if status, _ := transferRequest(t, app, transferCall{method: http.MethodPut, path: path, body: body}); status != http.StatusBadRequest {
			t.Fatalf("send %s: status %d, want 400", body, status)
		}
	}
	big := `{"a":"` + strings.Repeat("x", maxTransferBody) + `"}`
	if status, _ := transferRequest(t, app, transferCall{method: http.MethodPut, path: path, body: big}); status != http.StatusRequestEntityTooLarge {
		t.Fatalf("send too big: status %d, want 413", status)
	}
}

func TestTransferLinkTurnsAwayAClientThatKeepsGuessing(t *testing.T) {
	app := newTransferApp(t)
	code, _ := createTransferLink(t, app, "")
	for range maxTransferMissesPerClient {
		call := transferCall{method: http.MethodPut, path: "/transfer/" + wrongCode(code), client: "bot", body: `{}`}
		if status, _ := transferRequest(t, app, call); status != http.StatusNotFound {
			t.Fatalf("guess: status %d, want 404", status)
		}
	}
	if status, _ := transferRequest(t, app, transferCall{method: http.MethodPut, path: "/transfer/" + code, client: "bot", body: `{}`}); status != http.StatusTooManyRequests {
		t.Fatalf("bot after guessing: status %d, want 429", status)
	}
	if status, _ := transferRequest(t, app, transferCall{method: http.MethodPut, path: "/transfer/" + code, client: "phone", body: `{}`}); status != http.StatusNoContent {
		t.Fatalf("someone else: status %d, want 204", status)
	}
}

func TestTransferLinkBudgetsGuessesFromEveryoneTogether(t *testing.T) {
	app := newTransferApp(t)
	code, _ := createTransferLink(t, app, "")
	for i := range maxTransferMissesPerMinute {
		call := transferCall{method: http.MethodPut, path: "/transfer/" + wrongCode(code), client: fmt.Sprintf("bot-%d", i), body: `{}`}
		if status, _ := transferRequest(t, app, call); status != http.StatusNotFound {
			t.Fatalf("guess %d: status %d, want 404", i, status)
		}
	}
	if status, _ := transferRequest(t, app, transferCall{method: http.MethodPut, path: "/transfer/" + code, client: "phone", body: `{}`}); status != http.StatusTooManyRequests {
		t.Fatalf("right code during the flood: status %d, want 429", status)
	}

	transferLinksMu.Lock()
	transferMisses.reset = time.Now().Add(-time.Second)
	transferLinksMu.Unlock()
	if status, _ := transferRequest(t, app, transferCall{method: http.MethodPut, path: "/transfer/" + code, client: "phone", body: `{}`}); status != http.StatusNoContent {
		t.Fatalf("right code a minute later: status %d, want 204", status)
	}
}

func TestTransferLinkLimitsCodesPerClient(t *testing.T) {
	app := newTransferApp(t)
	for range maxTransferCreatesPerClient {
		createTransferLink(t, app, "bot")
	}
	if status, _ := transferRequest(t, app, transferCall{method: http.MethodPost, path: "/transfer", client: "bot"}); status != http.StatusTooManyRequests {
		t.Fatalf("one code too many: status %d, want 429", status)
	}
	createTransferLink(t, app, "phone")
}

func TestTransferLinkForgetsOldCounters(t *testing.T) {
	app := newTransferApp(t)
	past := time.Now().Add(-time.Second)
	transferLinksMu.Lock()
	for i := range 50 {
		transferClientMisses[fmt.Sprintf("old-%d", i)] = &transferCounter{count: 1, reset: past}
		transferCreates[fmt.Sprintf("old-%d", i)] = &transferCounter{count: 1, reset: past}
	}
	transferMisses.reset = past
	transferLinksMu.Unlock()

	transferRequest(t, app, transferCall{method: http.MethodPut, path: "/transfer/123456", client: "new", body: `{}`})

	transferLinksMu.Lock()
	defer transferLinksMu.Unlock()
	if len(transferClientMisses) != 1 || len(transferCreates) != 0 {
		t.Fatalf("kept %d miss and %d create counters, want 1 and 0", len(transferClientMisses), len(transferCreates))
	}
}

func TestClientIPTrustsOnlyTheEntryNginxAdded(t *testing.T) {
	loopback := net.ParseIP("127.0.0.1")
	cases := []struct {
		name         string
		peer         net.IP
		forwardedFor string
		want         string
	}{
		{"through nginx", loopback, "203.0.113.7", "203.0.113.7"},
		{"made-up entries before nginx's", loopback, "1.2.3.4, 198.51.100.9, 203.0.113.7", "203.0.113.7"},
		{"ipv6 client", loopback, "2001:db8::1", "2001:db8::1"},
		{"nothing forwarded", loopback, "", "127.0.0.1"},
		{"garbage forwarded", loopback, "not-an-ip", "127.0.0.1"},
		{"straight to the backend", net.ParseIP("198.51.100.20"), "203.0.113.7", "198.51.100.20"},
	}
	for _, tc := range cases {
		if got := clientIPFrom(tc.peer, tc.forwardedFor); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}
