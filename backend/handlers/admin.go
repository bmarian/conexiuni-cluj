package handlers

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

const (
	adminSessionCookie = "conexiuni_admin"
	adminSessionMaxAge = 12 * time.Hour
)

type adminLoginRequest struct {
	Token string `json:"token"`
}

func adminTokenMatches(token, candidate string) bool {
	expected := []byte(token)
	got := []byte(strings.TrimSpace(candidate))
	return subtle.ConstantTimeEq(int32(len(got)), int32(len(expected))) == 1 &&
		subtle.ConstantTimeCompare(got, expected) == 1
}

func adminSessionSignature(token, expires, nonce string) string {
	mac := hmac.New(sha256.New, []byte(token))
	_, _ = mac.Write([]byte(expires))
	_, _ = mac.Write([]byte("|"))
	_, _ = mac.Write([]byte(nonce))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func newAdminSession(token string, now time.Time) string {
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		nonceBytes = []byte(strconv.FormatInt(now.UnixNano(), 36))
	}
	expires := strconv.FormatInt(now.Add(adminSessionMaxAge).Unix(), 10)
	nonce := base64.RawURLEncoding.EncodeToString(nonceBytes)
	return expires + "." + nonce + "." + adminSessionSignature(token, expires, nonce)
}

func validAdminSession(token, value string, now time.Time) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return false
	}
	expUnix, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || !now.Before(time.Unix(expUnix, 0)) {
		return false
	}
	expected := []byte(adminSessionSignature(token, parts[0], parts[1]))
	got := []byte(parts[2])
	return subtle.ConstantTimeEq(int32(len(got)), int32(len(expected))) == 1 &&
		subtle.ConstantTimeCompare(got, expected) == 1
}

func setAdminSessionCookie(c fiber.Ctx, path, token string, secure bool) {
	now := time.Now()
	c.Cookie(&fiber.Cookie{
		Name:     adminSessionCookie,
		Value:    newAdminSession(token, now),
		Path:     path,
		Expires:  now.Add(adminSessionMaxAge),
		MaxAge:   int(adminSessionMaxAge.Seconds()),
		Secure:   secure,
		HTTPOnly: true,
		SameSite: "Strict",
	})
}

func clearAdminSessionCookie(c fiber.Ctx, path string, secure bool) {
	c.Cookie(&fiber.Cookie{
		Name:     adminSessionCookie,
		Value:    "",
		Path:     path,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		Secure:   secure,
		HTTPOnly: true,
		SameSite: "Strict",
	})
}

func RequireAdminSession(token string) fiber.Handler {
	return func(c fiber.Ctx) error {
		if token == "" {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "admin disabled"})
		}
		if validAdminSession(token, c.Cookies(adminSessionCookie), time.Now()) {
			return c.Next()
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
}

type adminCity struct {
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

func adminCityList(cities *Cities) []adminCity {
	out := make([]adminCity, 0, len(cities.All()))
	for _, city := range cities.All() {
		out = append(out, adminCity{Slug: city.Slug, Name: city.Name, Enabled: city.Enabled()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func RegisterAdminRoutes(api fiber.Router, cities *Cities, token string, secureCookies bool) {
	if token == "" || len(cities.All()) == 0 {
		return
	}
	admin := api.Group("/admin")
	cookiePath := "/api/admin"
	// SetEnabled never turns the last city off, so there is always one.
	cityOf := func(c fiber.Ctx) *City {
		if city := cities.Find(c.Query("city")); city != nil {
			return city
		}
		return cities.Enabled()[0]
	}

	admin.Post("/login", func(c fiber.Ctx) error {
		var req adminLoginRequest
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
		}
		if !adminTokenMatches(token, req.Token) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		setAdminSessionCookie(c, cookiePath, token, secureCookies)
		c.Set("Cache-Control", "no-store")
		return c.JSON(fiber.Map{"ok": true})
	})

	admin.Post("/logout", func(c fiber.Ctx) error {
		clearAdminSessionCookie(c, cookiePath, secureCookies)
		c.Set("Cache-Control", "no-store")
		return c.JSON(fiber.Map{"ok": true})
	})

	admin.Get("/overview", RequireAdminSession(token), func(c fiber.Ctx) error {
		overview, err := cityOf(c).buildAdminOverview(time.Now())
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		c.Set("Cache-Control", "no-store")
		return c.JSON(overview)
	})

	admin.Get("/cities", RequireAdminSession(token), func(c fiber.Ctx) error {
		c.Set("Cache-Control", "no-store")
		return c.JSON(adminCityList(cities))
	})

	admin.Put("/cities/:slug", RequireAdminSession(token), func(c fiber.Ctx) error {
		var req struct {
			Enabled *bool `json:"enabled"`
		}
		if err := c.Bind().Body(&req); err != nil || req.Enabled == nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
		}
		switch err := cities.SetEnabled(c.Params("slug"), *req.Enabled); {
		case errors.Is(err, ErrUnknownCity):
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		case errors.Is(err, ErrLastCity):
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
		case err != nil:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		c.Set("Cache-Control", "no-store")
		return c.JSON(adminCityList(cities))
	})

	registerAdminCalendarRoutes(admin, cityOf, token)
}
