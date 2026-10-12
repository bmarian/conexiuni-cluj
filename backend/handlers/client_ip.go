package handlers

import (
	"net"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// ClientIP is the address nginx saw. nginx runs on the same machine and appends that
// address to X-Forwarded-For, so only the last entry is trusted, and only on requests
// that came through it: anything before it, or on a direct request, the client wrote.
func ClientIP(c fiber.Ctx) string {
	return clientIPFrom(c.RequestCtx().RemoteIP(), c.Get(fiber.HeaderXForwardedFor))
}

// One IPv6 subscriber holds a whole /64, so that is what counts as one client.
func clientKey(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.To4() != nil {
		return ip
	}
	return parsed.Mask(net.CIDRMask(64, 128)).String()
}

func clientIPFrom(peer net.IP, forwardedFor string) string {
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
