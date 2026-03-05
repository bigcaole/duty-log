package handlers

import (
	"duty-log-system/internal/middleware"
	"duty-log-system/pkg/db"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.Engine, app *AppContext) {
	registerAuthRoutes(router, app)

	router.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/dashboard")
	})

	router.GET("/livez", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	readinessHandler := func(c *gin.Context) {
		if err := db.HealthCheck(app.DB); err != nil {
			c.JSON(503, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"ok": true})
	}

	router.GET("/health", readinessHandler)
	router.GET("/healthz", readinessHandler)
	router.GET("/readyz", readinessHandler)

	protected := router.Group("/")
	protected.Use(middleware.LoadCurrentUser(app.DB), middleware.Require2FA(app.DB), middleware.AuditLogger(app.DB))
	{
		registerMainRoutes(protected, app)
		registerIDCDutyRoutes(protected, app)
		registerDutyLogRoutes(protected, app)
		registerTicketRoutes(protected, app)
		registerWorkTicketRoutes(protected, app)
		registerFaultRecordRoutes(protected, app)
		registerInstructionRoutes(protected, app)
		registerReportRoutes(protected, app)
	}

	admin := protected.Group("/admin")
	admin.Use(middleware.AdminRequired(app.DB))
	{
		registerAdminRoutes(admin, app)
		registerCategoryRoutes(admin, app)
		registerSystemConfigRoutes(admin, app)
	}
}
