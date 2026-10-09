package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v3"
)

type assetLink struct {
	Relation []string        `json:"relation"`
	Target   assetLinkTarget `json:"target"`
}

type assetLinkTarget struct {
	Namespace              string   `json:"namespace"`
	PackageName            string   `json:"package_name"`
	SHA256CertFingerprints []string `json:"sha256_cert_fingerprints"`
}

func AssetLinksHandler(packageName string, fingerprints []string) fiber.Handler {
	links := []assetLink{}
	if packageName != "" && len(fingerprints) > 0 {
		upper := make([]string, len(fingerprints))
		for i, f := range fingerprints {
			upper[i] = strings.ToUpper(f)
		}
		links = append(links, assetLink{
			Relation: []string{"delegate_permission/common.handle_all_urls"},
			Target:   assetLinkTarget{Namespace: "android_app", PackageName: packageName, SHA256CertFingerprints: upper},
		})
	}
	return func(c fiber.Ctx) error {
		c.Set("Cache-Control", "public, max-age=3600")
		return c.JSON(links)
	}
}

func EncodesItself(path string) bool {
	switch path {
	case "/api/vehicles/stream", "/api/network":
		return true
	}
	return false
}
