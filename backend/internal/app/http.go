package app

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/patdyyew-emran/phishing-detector/internal/auth"
	services_scan "github.com/patdyyew-emran/phishing-detector/internal/services"
	"gorm.io/gorm"
)

type Dependencies struct {
	DB          *gorm.DB
	ScanService *services_scan.Scan
	JWTSecret   string
}

func NewRouter(deps Dependencies) *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: services_scan.ErrorHandler,

		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Authorization, Content-Type",
		AllowMethods: "GET,POST,DELETE,OPTIONS",
	}))

	registerHealthRoutes(app)

	api := app.Group("/api")

	registerAuthRoutes(api, deps)

	protected := api.Group("", auth.AuthMiddleware(deps.JWTSecret))

	registerScanRoutes(protected, deps.ScanService)
	registerStatsRoutes(protected, deps.ScanService)

	return app
}

func registerHealthRoutes(app *fiber.App) {
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
		})
	})
}

func registerAuthRoutes(api fiber.Router, deps Dependencies) {
	authRoutes := api.Group("/auth")

	authRoutes.Post(
		"/login",
		auth.LoginHandler(deps.DB, deps.JWTSecret),
	)
}

func registerScanRoutes(
	protected fiber.Router,
	scanService *services_scan.Scan,
) {
	scans := protected.Group("/scans")

	scans.Post("/", scanService.CreateScan)
	scans.Get("/", scanService.ListScans)
	scans.Get("/:id", scanService.GetScan)
}

func registerStatsRoutes(
	protected fiber.Router,
	scanService *services_scan.Scan,
) {

	stats := protected.Group(
		"/stats",
		auth.RequireRole(auth.RoleAdmin),
	)

	stats.Get("/", scanService.Stats)
}
