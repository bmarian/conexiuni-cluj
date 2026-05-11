package handlers

import (
	"bufio"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"conexiuni-cluj/database"
	"conexiuni-cluj/services/tranzy"

	"github.com/gofiber/fiber/v3"
)

const (
	maxLogTail         = 1000
	defaultLogTail     = 200
	adminLogPrefix     = "access"
	logScanMaxLineSize = 1 << 20 // Defensive cap for very long log lines.
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

func setAdminSessionCookie(c fiber.Ctx, token string, secure bool) {
	now := time.Now()
	c.Cookie(&fiber.Cookie{
		Name:     adminSessionCookie,
		Value:    newAdminSession(token, now),
		Path:     "/api/admin",
		Expires:  now.Add(adminSessionMaxAge),
		MaxAge:   int(adminSessionMaxAge.Seconds()),
		Secure:   secure,
		HTTPOnly: true,
		SameSite: "Strict",
	})
}

func clearAdminSessionCookie(c fiber.Ctx, secure bool) {
	c.Cookie(&fiber.Cookie{
		Name:     adminSessionCookie,
		Value:    "",
		Path:     "/api/admin",
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

type adminStatsResponse struct {
	Visitors    database.VisitorTotals       `json:"visitors"`
	DailyVisits []database.DailyVisitorPoint `json:"daily_visits"`
	ActiveNow   int                          `json:"active_now"`
	TopRoutes   []database.TopEntry          `json:"top_routes"`
	TopStops    []database.TopEntry          `json:"top_stops"`
	TopAPI      []database.TopEntry          `json:"top_api"`
	PWAInstalls int                          `json:"pwa_installs"`
	TranzyQuota tranzyQuotaSnapshot          `json:"tranzy_quota"`
	GeneratedAt string                       `json:"generated_at"`
}

type tranzyQuotaSnapshot struct {
	VehiclesRemaining int `json:"vehicles_remaining"`
	VehiclesLimit     int `json:"vehicles_limit"`
	DefaultRemaining  int `json:"default_remaining"`
	DefaultLimit      int `json:"default_limit"`
}

func normalizeTopRouteKeys(entries []database.TopEntry) []database.TopEntry {
	out := make([]database.TopEntry, len(entries))
	copy(out, entries)
	for i := range out {
		id, err := strconv.Atoi(out[i].Key)
		if err != nil {
			continue
		}
		var shortName string
		err = database.DB.QueryRow(`SELECT route_short_name FROM routes WHERE route_id = ?`, id).Scan(&shortName)
		if err == nil && strings.TrimSpace(shortName) != "" {
			out[i].Key = shortName
		}
	}
	return out
}

func RegisterAdminRoutes(api fiber.Router, token, logDir string, tranzyClient *tranzy.Client, secureCookies bool) {
	if token == "" {
		return
	}
	admin := api.Group("/admin")

	admin.Post("/login", func(c fiber.Ctx) error {
		var req adminLoginRequest
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
		}
		if !adminTokenMatches(token, req.Token) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		setAdminSessionCookie(c, token, secureCookies)
		c.Set("Cache-Control", "no-store")
		return c.JSON(fiber.Map{"ok": true})
	})

	admin.Post("/logout", func(c fiber.Ctx) error {
		clearAdminSessionCookie(c, secureCookies)
		c.Set("Cache-Control", "no-store")
		return c.JSON(fiber.Map{"ok": true})
	})

	admin.Get("/stats", RequireAdminSession(token), func(c fiber.Ctx) error {
		visitors, err := database.GetVisitorTotals()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		daily, err := database.GetDailyVisitors(30)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		topRoutes, err := database.GetTopMetric("route_view", 25)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		topRoutes = normalizeTopRouteKeys(topRoutes)
		topStops, err := database.GetTopMetric("stop_view", 25)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		topAPI, err := database.GetTopMetric("api_call", 25)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		pwaInstalls, err := database.GetMetricTotal("pwa_install", "appinstalled")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		resp := adminStatsResponse{
			Visitors:    visitors,
			DailyVisits: daily,
			ActiveNow:   VehicleHub.SubscriberCount(),
			TopRoutes:   topRoutes,
			TopStops:    topStops,
			TopAPI:      topAPI,
			PWAInstalls: pwaInstalls,
			TranzyQuota: tranzyQuotaSnapshot{
				VehiclesRemaining: tranzyClient.VehiclesQuotaRemaining(),
				VehiclesLimit:     tranzyClient.VehiclesQuotaLimit(),
				DefaultRemaining:  tranzyClient.DefaultQuotaRemaining(),
				DefaultLimit:      tranzyClient.DefaultQuotaLimit(),
			},
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		}
		c.Set("Cache-Control", "no-store")
		return c.JSON(resp)
	})

	admin.Get("/logs", RequireAdminSession(token), func(c fiber.Ctx) error {
		tail := defaultLogTail
		if s := c.Query("tail"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				tail = n
			}
		}
		if tail > maxLogTail {
			tail = maxLogTail
		}
		lines, err := readAccessLogTail(logDir, tail)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		c.Set("Cache-Control", "no-store")
		return c.JSON(fiber.Map{"lines": lines, "count": len(lines)})
	})
}

func readAccessLogTail(logDir string, tail int) ([]string, error) {
	loc, err := time.LoadLocation("Europe/Bucharest")
	if err != nil {
		loc = time.UTC
	}
	today := time.Now().In(loc)

	collected := make([]string, 0, tail)
	for offset := 0; offset < 7 && len(collected) < tail; offset++ {
		date := today.AddDate(0, 0, -offset).Format("2006-01-02")
		path := filepath.Join(logDir, fmt.Sprintf("%s-%s.log", adminLogPrefix, date))
		dayLines, err := readFileTail(path, tail-len(collected))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		// Prepend so the final slice ends with the newest day's last line.
		collected = append(dayLines, collected...)
	}
	return collected, nil
}

func readFileTail(path string, max int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), logScanMaxLineSize)

	ring := make([]string, 0, max)
	for scanner.Scan() {
		line := scanner.Text()
		if len(ring) < max {
			ring = append(ring, line)
		} else {
			copy(ring, ring[1:])
			ring[max-1] = line
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return ring, nil
}
