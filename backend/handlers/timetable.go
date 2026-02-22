package handlers

import (
	ctpcj "conexiuni-cluj/services/ctp-cj"
	"time"

	"github.com/gofiber/fiber/v3"
)

func GetTimetable(c fiber.Ctx, ctpCjClient *ctpcj.Client, cacheShelfLife time.Duration, routeShortName string) error {
	return nil
}
