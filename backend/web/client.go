// Package web holds what every endpoint shares: who is asking, and the headers answers carry.
package web

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// nginx on this host appends the peer to X-Forwarded-For, so only the last entry is trusted.
func ClientIP(c fiber.Ctx) string {
	return ClientIPFrom(c.RequestCtx().RemoteIP(), c.Get(fiber.HeaderXForwardedFor))
}

// One IPv6 subscriber holds a whole /64, so that is what counts as one client.
func ClientKey(ip string) string { return ipv6Prefix(ip, 64) }

// One person can hold a whole /48.
func NetworkKey(ip string) string { return ipv6Prefix(ip, 48) }

func ipv6Prefix(ip string, bits int) string {
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.To4() != nil {
		return ip
	}
	return parsed.Mask(net.CIDRMask(bits, 128)).String()
}

func ClientIPFrom(peer net.IP, forwardedFor string) string {
	if peer.IsLoopback() || peer.IsPrivate() {
		last := forwardedFor[strings.LastIndex(forwardedFor, ",")+1:]
		if ip := net.ParseIP(strings.TrimSpace(last)); ip != nil {
			return ip.String()
		}
	}
	if peer == nil {
		return ""
	}
	return peer.String()
}

const (
	clientHashLocalKey = "client_hash"
	clientKindLocalKey = "client_kind"
)

// Same scheme as the access log id; IPv6 clients by /64.
func ComputeClientHash(c fiber.Ctx, salt string) string {
	ip := ClientIP(c)
	if ip == "" {
		return ""
	}
	h := hmac.New(sha256.New, []byte(salt))
	_, _ = h.Write([]byte(ClientKey(ip)))
	return hex.EncodeToString(h.Sum(nil)[:8])
}

// ClientHash is the request's pseudonymous client id, set by Identify.
func ClientHash(c fiber.Ctx) string {
	if v, ok := c.Locals(clientHashLocalKey).(string); ok {
		return v
	}
	return ""
}

var appUserAgents = map[string]string{
	"ConexiuniClujMobile/": "android",
	"ConexiuniClujWear/":   "wear",
}

var botUserAgentWords = []string{"bot", "crawl", "spider", "slurp", "preview", "lighthouse", "headless"}

// clientKind is "web", "android/1.4.0" or "wear/1.0.2"; empty for robots and scripts.
func ClientKind(ua, fetchSite string) string {
	for prefix, platform := range appUserAgents {
		if version, ok := strings.CutPrefix(ua, prefix); ok {
			if validAppVersion(version) {
				return platform + "/" + version
			}
			return platform
		}
	}
	lower := strings.ToLower(ua)
	for _, w := range botUserAgentWords {
		if strings.Contains(lower, w) {
			return ""
		}
	}
	// Browsers mark the site's own fetches; scripts that borrow a browser user agent don't.
	if strings.HasPrefix(ua, "Mozilla/") && fetchSite == "same-origin" {
		return "web"
	}
	return ""
}

// IsPerson tells the website and the apps apart from robots and scripts.
func IsPerson(c fiber.Ctx) bool {
	kind, _ := c.Locals(clientKindLocalKey).(string)
	return kind != ""
}

func validAppVersion(v string) bool {
	if v == "" || len(v) > 16 {
		return false
	}
	for _, r := range v {
		if (r < '0' || r > '9') && r != '.' {
			return false
		}
	}
	return true
}

// Identify records who is asking for the rest of the request: their client id and what kind of client they are.
func Identify(c fiber.Ctx, salt string) (hash, kind string) {
	hash = ComputeClientHash(c, salt)
	kind = ClientKind(c.Get(fiber.HeaderUserAgent), c.Get("Sec-Fetch-Site"))
	SetClient(c, hash, kind)
	return hash, kind
}

func SetClient(c fiber.Ctx, hash, kind string) {
	c.Locals(clientHashLocalKey, hash)
	c.Locals(clientKindLocalKey, kind)
}
