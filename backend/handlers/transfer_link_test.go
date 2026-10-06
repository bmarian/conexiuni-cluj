package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func newTransferApp(t *testing.T) *fiber.App {
	t.Helper()
	transferLinksMu.Lock()
	transferLinks = map[string]*transferLink{}
	transferLinkMissed = map[string]*transferLinkMisses{}
	transferLinksMu.Unlock()
	app := fiber.New()
	app.Post("/transfer", CreateTransferLink)
	app.Put("/transfer/:code", SendToTransferLink)
	app.Get("/transfer/:code", ReadTransferLink)
	app.Delete("/transfer/:code", DeleteTransferLink)
	return app
}

func transferRequest(t *testing.T, app *fiber.App, method, path, secret, body string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if secret != "" {
		req.Header.Set(transferLinkSecretHeader, secret)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(data)
}

func createTransferLink(t *testing.T, app *fiber.App) (code, secret string) {
	t.Helper()
	status, body := transferRequest(t, app, http.MethodPost, "/transfer", "", "")
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

func TestTransferLinkHandsTheExportToTheReceiver(t *testing.T) {
	app := newTransferApp(t)
	code, secret := createTransferLink(t, app)
	path := "/transfer/" + code

	if status, _ := transferRequest(t, app, http.MethodGet, path, secret, ""); status != http.StatusNoContent {
		t.Fatalf("before sending: status %d, want 204", status)
	}

	sent := `{"version":1,"settings":{"locale":"en"},"favorites":{"stops":[391,12],"routes":[{"routeId":7,"direction":"1"}]},"followedLines":["25"]}`
	if status, _ := transferRequest(t, app, http.MethodPut, path, "", "  "+sent+"\n"); status != http.StatusNoContent {
		t.Fatalf("send: status %d", status)
	}

	status, body := transferRequest(t, app, http.MethodGet, path, secret, "")
	if status != http.StatusOK || body != sent {
		t.Fatalf("read: status %d, body %s", status, body)
	}

	if status, _ := transferRequest(t, app, http.MethodGet, path, "wrong", ""); status != http.StatusNotFound {
		t.Fatalf("wrong secret: status %d, want 404", status)
	}
	if status, _ := transferRequest(t, app, http.MethodDelete, path, secret, ""); status != http.StatusNoContent {
		t.Fatalf("delete: status %d", status)
	}
	if status, _ := transferRequest(t, app, http.MethodGet, path, secret, ""); status != http.StatusNotFound {
		t.Fatalf("after delete: status %d, want 404", status)
	}
}

func TestTransferLinkTakesOnlySmallJSONObjects(t *testing.T) {
	app := newTransferApp(t)
	code, _ := createTransferLink(t, app)
	path := "/transfer/" + code
	for _, body := range []string{`[1,2]`, `{"a":`, `"text"`} {
		if status, _ := transferRequest(t, app, http.MethodPut, path, "", body); status != http.StatusBadRequest {
			t.Fatalf("send %s: status %d, want 400", body, status)
		}
	}
	big := `{"a":"` + strings.Repeat("x", maxTransferBody) + `"}`
	if status, _ := transferRequest(t, app, http.MethodPut, path, "", big); status != http.StatusRequestEntityTooLarge {
		t.Fatalf("send too big: status %d, want 413", status)
	}
}

func TestTransferLinkTurnsAwayGuessing(t *testing.T) {
	app := newTransferApp(t)
	code, _ := createTransferLink(t, app)
	wrong := "000000"
	if code == wrong {
		wrong = "000001"
	}
	for range maxTransferLinkMisses {
		if status, _ := transferRequest(t, app, http.MethodPut, "/transfer/"+wrong, "", `{}`); status != http.StatusNotFound {
			t.Fatalf("guess: status %d, want 404", status)
		}
	}
	if status, _ := transferRequest(t, app, http.MethodPut, "/transfer/"+code, "", `{}`); status != http.StatusTooManyRequests {
		t.Fatalf("after guessing: status %d, want 429", status)
	}
}
