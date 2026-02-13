package handlers

import (
	"conexiuni-cluj/services/tranzy"

	"github.com/gofiber/fiber/v3"
)

var tranzyClient *tranzy.Client

func InitTranzyClient(baseUrl string, apiKey string, agencyId string) {
	tranzyClient = tranzy.NewClient(baseUrl, apiKey, agencyId)
}

func GetRoutes(c fiber.Ctx) error {
	routes, err := tranzyClient.GetRoutes()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(routes)
}
