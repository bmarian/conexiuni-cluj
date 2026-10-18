package main

import (
	"conexiuni-cluj/handlers"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/etag"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/static"
)

func clientIDForLog(c fiber.Ctx, salt string) string {
	if cached := handlers.ClientHashFromLocals(c); cached != "" {
		return cached
	}
	hash := handlers.ComputeClientHash(c, salt)
	if hash == "" {
		return "-"
	}
	return hash
}

func runReplay(cfg *Config, slug, span string) {
	fromText, toText, ok := strings.Cut(span, ":")
	if !ok {
		log.Fatalf("replay: want FROM:TO, like 2026-09-01:2026-10-15")
	}
	city, err := offlineCity(cfg, slug)
	if err != nil {
		log.Fatalf("Failed to open %s: %v", slug, err)
	}
	defer city.Close()
	from, errFrom := time.ParseInLocation("2006-01-02", fromText, city.Location)
	to, errTo := time.ParseInLocation("2006-01-02", toText, city.Location)
	if errFrom != nil || errTo != nil || to.Before(from) {
		log.Fatalf("replay: %q is not a FROM:TO date range", span)
	}
	if err := city.LoadCalendar(); err != nil {
		log.Fatalf("Failed to load the calendar: %v", err)
	}
	city.LoadWindowTuning()
	if err := city.RunReplay(from, to, os.Stdout); err != nil {
		log.Fatalf("replay: %v", err)
	}
}

func runShift(cfg *Config, slug, date string) {
	city, err := offlineCity(cfg, slug)
	if err != nil {
		log.Fatalf("Failed to open %s: %v", slug, err)
	}
	defer city.Close()
	day, err := time.ParseInLocation("2006-01-02", date, city.Location)
	if err != nil {
		log.Fatalf("shift: want a date like 2026-09-28, got %q", date)
	}
	if err := city.LoadCalendar(); err != nil {
		log.Fatalf("Failed to load the calendar: %v", err)
	}
	if err := city.RunShift(day, os.Stdout); err != nil {
		log.Fatalf("shift: %v", err)
	}
}

func main() {
	_ = os.Setenv("TZ", "Europe/Bucharest")

	replay := flag.String("replay", "", "compare the calendar learning windows with the old 14-day window on archived days FROM:TO (YYYY-MM-DD:YYYY-MM-DD), print the results and exit")
	shift := flag.String("shift", "", "print whether archived travel times stepped up on YYYY-MM-DD against the 10 days of its kind before it, then exit")
	citySlug := flag.String("city", "cluj", "the city a maintenance command works on")
	refresh := flag.Bool("refresh", false, "fetch every source again at start, whatever the database holds")
	flag.Parse()

	config := Load()

	if *replay != "" {
		runReplay(config, *citySlug, *replay)
		return
	}
	if *shift != "" {
		runShift(config, *citySlug, *shift)
		return
	}

	logHashSalt := config.LogIPHashSalt
	if logHashSalt == "" {
		if config.Environment == "production" {
			log.Fatal("LOG_IP_HASH_SALT must be set in production")
		}
		logHashSalt = "conexiuni-cluj-default-salt"
		log.Printf("Warning: LOG_IP_HASH_SALT not set, using default salt; set LOG_IP_HASH_SALT for stronger pseudonymization")
	}

	logs, err := setupLogging(config.LogDir, config.LogRetentionDays)
	if err != nil {
		log.Fatalf("Failed to initialize logging: %v", err)
	}
	defer func() {
		if err := logs.close(); err != nil {
			log.Printf("Warning: failed to close log file: %v", err)
		}
	}()

	log.Printf("Starting Ce autobuz iau? server in %s mode on port %s", config.Environment, config.Port)

	if config.AdminToken == "" {
		if config.Environment == "production" {
			log.Fatal("ADMIN_TOKEN must be set in production")
		}
		log.Printf("Warning: ADMIN_TOKEN not set; admin routes are disabled")
	}
	if err := handlers.InitPush(config.VapidPublicKey, config.VapidPrivateKey, config.VapidSubject); err != nil {
		log.Printf("Warning: push notifications disabled: %v", err)
	}

	cities, err := buildCities(config)
	if err != nil {
		log.Fatalf("Failed to set up the cities: %v", err)
	}
	bySlug := make(map[string]*handlers.City, len(cities))
	for _, city := range cities {
		if *refresh {
			if err := city.ForgetFetches(); err != nil {
				log.Fatalf("%s: %v", city.Slug, err)
			}
		}
		bySlug[city.Slug] = city
		defer func() {
			if err := city.FlushStats(); err != nil {
				log.Printf("%s: final stats flush failed: %v", city.Slug, err)
			}
			city.Close()
		}()
	}

	app := fiber.New(fiber.Config{
		AppName: "Ce autobuz iau?",
	})
	app.Use(handlers.StatsMiddleware(bySlug, logHashSalt))
	app.Use(logger.New(logger.Config{
		Stream: logs.accessOut,
		Format: "${time} | status=${status} | latency=${latency} | user=${clientId} | method=${method} | url=${url} | ua=${ua} | error=${error}\n",
		CustomTags: map[string]logger.LogFunc{"clientId": func(output logger.Buffer, c fiber.Ctx, _ *logger.Data, _ string) (int, error) {
			return output.WriteString(clientIDForLog(c, logHashSalt))
		}},
		TimeFormat: StandardLogTimestampLayout,
		TimeZone:   "Europe/Bucharest",
	}))
	app.Use(cors.New(cors.Config{
		AllowHeaders: []string{"Origin", "Content-Type", "Accept"},
	}))
	app.Use(handlers.SecurityHeaders)
	app.Use(compress.New(compress.Config{
		Next: func(c fiber.Ctx) bool {
			return handlers.EncodesItself(c.Path())
		},
	}))

	app.Get("/.well-known/assetlinks.json", handlers.AssetLinksHandler(config.AndroidPackage, config.AndroidCertSHA256))

	api := app.Group("/api")
	api.Use(etag.New(etag.Config{
		Weak: true,
		Next: func(c fiber.Ctx) bool {
			return handlers.EncodesItself(c.Path())
		},
	}))
	api.Get("/cities", handlers.CitiesHandler(cities))
	api.Post("/transfer", handlers.CreateTransferLink)
	api.Put("/transfer/:code", handlers.SendToTransferLink)
	api.Get("/transfer/:code", handlers.ReadTransferLink)
	api.Delete("/transfer/:code", handlers.DeleteTransferLink)
	handlers.RegisterAdminRoutes(api, cities, config.AdminToken, config.Environment == "production")
	for _, city := range cities {
		city.Register(api.Group("/" + city.Slug))
	}

	if _, err := os.Stat("./dist"); err == nil {
		handlers.LoadIndexHTML()

		app.Get("/robots.txt", func(c fiber.Ctx) error {
			c.Set("Cache-Control", "public, max-age=86400")
			return c.SendFile("./dist/robots.txt")
		})
		app.Get("/sitemap.xml", handlers.SitemapHandler(cities))
		for _, name := range []string{"sw.js", "push-sw.js", "registerSW.js"} {
			app.Get("/"+name, func(c fiber.Ctx) error {
				c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
				c.Set("Service-Worker-Allowed", "/")
				return c.SendFile("./dist/" + name)
			})
		}
		app.Get("/manifest.webmanifest", func(c fiber.Ctx) error {
			c.Set("Cache-Control", "no-cache")
			c.Set("Content-Type", "application/manifest+json")
			return c.SendFile("./dist/manifest.webmanifest")
		})

		app.Get("/android", handlers.AndroidOGHandler)
		for _, city := range cities {
			city.RegisterPages(app.Group("/" + city.Slug))
		}

		app.Use("/", static.New("./dist", static.Config{
			Browse:   false,
			Compress: true,
		}))

		app.Use("*", func(c fiber.Ctx) error {
			path := c.Path()
			if strings.HasPrefix(path, "/api/") {
				return c.SendStatus(fiber.StatusNotFound)
			}
			if ext := filepath.Ext(path); ext != "" && ext != ".html" {
				return c.SendStatus(fiber.StatusNotFound)
			}
			return c.SendFile("./dist/index.html")
		})

		log.Println("Serving frontend from ./dist")
	} else {
		log.Println("Frontend dist folder not found. Run 'npm run build' in frontend directory.")
	}

	listenAddr := ":" + config.Port
	if config.Environment == "development" && runtime.GOOS == "windows" {
		listenAddr = "0.0.0.0:" + config.Port
	}
	shutdownSignals := make(chan os.Signal, 1)
	signal.Notify(shutdownSignals, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-shutdownSignals
		log.Printf("Received %s, shutting down", sig)
		if err := app.Shutdown(); err != nil {
			log.Printf("Server shutdown failed: %v", err)
		}
	}()
	// The cities start once the port is ours: a bind that fails must not leave fetches and learners running.
	err = app.Listen(listenAddr, fiber.ListenConfig{ListenerAddrFunc: func(addr net.Addr) {
		log.Printf("Server listening on http://%s", addr)
		for _, city := range cities {
			city.Start()
		}
	}})
	if err != nil {
		log.Fatalf("Server stopped: %v", err)
	}
}
