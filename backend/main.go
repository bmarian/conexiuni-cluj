package main

import (
	"conexiuni-cluj/admin"
	"conexiuni-cluj/cities"
	"conexiuni-cluj/config"
	"conexiuni-cluj/devicesync"
	"conexiuni-cluj/pages"
	"conexiuni-cluj/web"
	"flag"
	"io"
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
	if cached := web.ClientHash(c); cached != "" {
		return cached
	}
	hash := web.ComputeClientHash(c, salt)
	if hash == "" {
		return "-"
	}
	return hash
}

func sendIndex(c fiber.Ctx) error {
	return c.SendFile("./dist/index.html")
}

func runReplay(cfg *config.Config, slug, span string) {
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
	if err := city.Live.LoadCalendar(); err != nil {
		log.Fatalf("Failed to load the calendar: %v", err)
	}
	city.Live.LoadWindowTuning()
	if err := city.Live.RunReplay(from, to, os.Stdout); err != nil {
		log.Fatalf("replay: %v", err)
	}
}

func runShift(cfg *config.Config, slug, date string) {
	city, err := offlineCity(cfg, slug)
	if err != nil {
		log.Fatalf("Failed to open %s: %v", slug, err)
	}
	defer city.Close()
	day, err := time.ParseInLocation("2006-01-02", date, city.Location)
	if err != nil {
		log.Fatalf("shift: want a date like 2026-09-28, got %q", date)
	}
	if err := city.Live.LoadCalendar(); err != nil {
		log.Fatalf("Failed to load the calendar: %v", err)
	}
	if err := city.Live.RunShift(day, os.Stdout); err != nil {
		log.Fatalf("shift: %v", err)
	}
}

func accessLogger(out io.Writer, logHashSalt string) fiber.Handler {
	return logger.New(logger.Config{
		Stream: out,
		Format: "${time} | status=${status} | latency=${latency} | user=${clientId} | method=${method} | url=${loggedUrl} | ua=${ua} | error=${error}\n",
		CustomTags: map[string]logger.LogFunc{
			"clientId": func(output logger.Buffer, c fiber.Ctx, _ *logger.Data, _ string) (int, error) {
				return output.WriteString(clientIDForLog(c, logHashSalt))
			},
			"loggedUrl": func(output logger.Buffer, c fiber.Ctx, _ *logger.Data, _ string) (int, error) {
				return output.WriteString(web.LogURL(devicesync.LogPath(c.OriginalURL())))
			},
		},
		TimeFormat: StandardLogTimestampLayout,
		TimeZone:   "Europe/Bucharest",
	})
}

func newApp(cfg *config.Config, built []*cities.City, registry *cities.Registry, devices *devicesync.Service, accessLog io.Writer, logHashSalt string) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName: "Ce autobuz iau?",
	})
	bySlug := make(map[string]*cities.City, len(built))
	for _, city := range built {
		bySlug[city.Slug] = city
	}
	app.Use(cities.StatsMiddleware(bySlug, logHashSalt))
	app.Use(accessLogger(accessLog, logHashSalt))
	app.Use(cors.New(cors.Config{
		AllowHeaders: []string{"Origin", "Content-Type", "Accept"},
	}))
	app.Use(web.SecurityHeaders)
	app.Use(compress.New(compress.Config{
		Next: func(c fiber.Ctx) bool {
			return web.EncodesItself(c)
		},
	}))

	app.Get("/.well-known/assetlinks.json", web.AssetLinksHandler(cfg.AndroidPackage, cfg.AndroidCertSHA256))

	api := app.Group("/api")
	api.Use(etag.New(etag.Config{
		Weak: true,
		Next: func(c fiber.Ctx) bool {
			return web.EncodesItself(c)
		},
	}))
	api.Get("/cities", cities.Handler(registry))
	devices.Register(api)
	admin.Register(api, registry, devices.Stats, cfg.AdminToken, cfg.Environment == "production")
	notFound := func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusNotFound) }
	for _, city := range built {
		city.Register(api.Group("/"+city.Slug, city.OnlyEnabled(notFound)))
	}

	if _, err := os.Stat("./dist"); err == nil {
		pages.LoadIndexHTML()

		app.Get("/robots.txt", func(c fiber.Ctx) error {
			c.Set("Cache-Control", "public, max-age=86400")
			return c.SendFile("./dist/robots.txt")
		})
		app.Get("/sitemap.xml", pages.Sitemap(registry))
		for _, name := range []string{"sw.js", "registerSW.js"} {
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

		for _, city := range built {
			pages.Register(app.Group("/"+city.Slug, city.OnlyEnabled(sendIndex)), city)
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
			return sendIndex(c)
		})

		log.Println("Serving frontend from ./dist")
	} else {
		log.Println("Frontend dist folder not found. Run 'npm run build' in frontend directory.")
	}
	return app
}

func main() {
	_ = os.Setenv("TZ", "Europe/Bucharest")

	replay := flag.String("replay", "", "compare the calendar learning windows with the old 14-day window on archived days FROM:TO (YYYY-MM-DD:YYYY-MM-DD), print the results and exit")
	shift := flag.String("shift", "", "print whether archived travel times stepped up on YYYY-MM-DD against the 10 days of its kind before it, then exit")
	citySlug := flag.String("city", "cluj", "the city a maintenance command works on")
	refresh := flag.Bool("refresh", false, "fetch every source again at start, whatever the database holds")
	flag.Parse()

	cfg := config.Load()

	if *replay != "" {
		runReplay(cfg, *citySlug, *replay)
		return
	}
	if *shift != "" {
		runShift(cfg, *citySlug, *shift)
		return
	}

	logHashSalt := cfg.LogIPHashSalt
	if logHashSalt == "" {
		if cfg.Environment == "production" {
			log.Fatal("LOG_IP_HASH_SALT must be set in production")
		}
		logHashSalt = "conexiuni-cluj-default-salt"
		log.Printf("Warning: LOG_IP_HASH_SALT not set, using default salt; set LOG_IP_HASH_SALT for stronger pseudonymization")
	}

	logs, err := setupLogging(cfg.LogDir, cfg.LogRetentionDays)
	if err != nil {
		log.Fatalf("Failed to initialize logging: %v", err)
	}
	defer func() {
		if err := logs.close(); err != nil {
			log.Printf("Warning: failed to close log file: %v", err)
		}
	}()

	log.Printf("Starting Ce autobuz iau? server in %s mode on port %s", cfg.Environment, cfg.Port)

	if cfg.AdminToken == "" {
		if cfg.Environment == "production" {
			log.Fatal("ADMIN_TOKEN must be set in production")
		}
		log.Printf("Warning: ADMIN_TOKEN not set; admin routes are disabled")
	}

	built, err := buildCities(cfg)
	if err != nil {
		log.Fatalf("Failed to set up the cities: %v", err)
	}
	registry, err := cities.NewRegistry(built, filepath.Join(cfg.DBDir, "disabled-cities.json"))
	if err != nil {
		log.Fatalf("Failed to read the disabled cities: %v", err)
	}
	for _, city := range built {
		if *refresh {
			if err := city.ForgetFetches(); err != nil {
				log.Fatalf("%s: %v", city.Slug, err)
			}
		}
		defer func() {
			if err := city.FlushStats(); err != nil {
				log.Printf("%s: final stats flush failed: %v", city.Slug, err)
			}
			city.Close()
		}()
	}

	devices, err := devicesync.Open(filepath.Join(cfg.DBDir, "sync.db"), devicesync.Options{Cities: len(built), MaxBytes: int64(cfg.SyncDBMaxMB) << 20})
	if err != nil {
		log.Fatalf("Failed to open the sync database: %v", err)
	}
	defer func() {
		if err := devices.Close(); err != nil {
			log.Printf("Warning: failed to close the sync database: %v", err)
		}
	}()

	app := newApp(cfg, built, registry, devices, logs.accessOut, logHashSalt)

	listenAddr := ":" + cfg.Port
	if cfg.Environment == "development" && runtime.GOOS == "windows" {
		listenAddr = "0.0.0.0:" + cfg.Port
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
	// Only once the port is bound, so a failed bind leaves nothing running.
	err = app.Listen(listenAddr, fiber.ListenConfig{ListenerAddrFunc: func(addr net.Addr) {
		log.Printf("Server listening on http://%s", addr)
		registry.Start()
		devices.Start()
	}})
	registry.Shutdown()
	if err != nil {
		log.Fatalf("Server stopped: %v", err)
	}
}
