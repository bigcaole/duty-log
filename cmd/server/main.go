package main

import (
	"context"
	"errors"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"time"

	"duty-log-system/internal/config"
	"duty-log-system/internal/handlers"
	"duty-log-system/internal/middleware"
	"duty-log-system/pkg/db"
	"duty-log-system/pkg/utils"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	gin.SetMode(cfg.GinMode)

	database, err := db.Connect(cfg)
	if err != nil {
		log.Fatalf("connect database failed: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		log.Fatalf("get sql db failed: %v", err)
	}
	defer func() {
		if closeErr := sqlDB.Close(); closeErr != nil {
			log.Printf("close database failed: %v", closeErr)
		}
	}()

	if err := db.AutoMigrate(database); err != nil {
		log.Fatalf("auto migrate failed: %v", err)
	}

	if err := db.SeedDefaultAdmin(database); err != nil {
		log.Fatalf("seed default admin failed: %v", err)
	}

	configCenter := utils.NewConfigCenter(database, cfg.SecretKey)
	middleware.ConfigureLoginRateLimiter(
		middleware.ParsePositiveInt(configCenter.Get("LOGIN_MAX_ATTEMPTS", "5"), 5),
		time.Duration(middleware.ParsePositiveInt(configCenter.Get("LOGIN_WINDOW_SECONDS", "600"), 600))*time.Second,
		time.Duration(middleware.ParsePositiveInt(configCenter.Get("LOGIN_BLOCK_SECONDS", "900"), 900))*time.Second,
	)

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), middleware.SecurityHeaders())
	router.MaxMultipartMemory = 10 << 30
	if err := router.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		log.Printf("set trusted proxies failed: %v", err)
	}

	setupSession(router, cfg)
	loadTemplates(router, "templates")
	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		log.Printf("create upload dir failed: %v", err)
	}
	uploadBase := strings.TrimRight(cfg.UploadURLBase, "/")
	if uploadBase == "" {
		uploadBase = "/static/uploads"
	}
	router.GET("/static/*filepath", handlers.StaticFileHandler(cfg.UploadDir, "./static"))
	if !strings.HasPrefix(uploadBase, "/static") {
		router.GET(uploadBase+"/*filepath", handlers.UploadFileHandler(cfg.UploadDir))
	}

	app := handlers.NewAppContext(database, configCenter, cfg)
	handlers.RegisterRoutes(router, app)

	if err := app.ReloadBackupScheduler(); err != nil {
		log.Printf("backup scheduler startup failed: %v", err)
	}
	if err := app.ReloadReportScheduler(); err != nil {
		log.Printf("report scheduler startup failed: %v", err)
	}
	if err := app.ReloadReminderScheduler(); err != nil {
		log.Printf("reminder scheduler startup failed: %v", err)
	}

	serverAddr := ":" + cfg.Port
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.HTTPReadTimeoutSec) * time.Second,
		WriteTimeout: time.Duration(cfg.HTTPWriteTimeoutSec) * time.Second,
		IdleTimeout:  time.Duration(cfg.HTTPIdleTimeoutSec) * time.Second,
	}

	errChan := make(chan error, 1)
	go func() {
		log.Printf("Duty-Log server listening on %s (gin_mode=%s)", serverAddr, cfg.GinMode)
		errChan <- srv.ListenAndServe()
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		log.Printf("received signal: %s", sig.String())
	case runErr := <-errChan:
		if runErr != nil && !errors.Is(runErr, http.ErrServerClosed) {
			log.Fatalf("server run failed: %v", runErr)
		}
		return
	}

	app.StopBackupScheduler()
	app.StopReportScheduler()
	app.StopReminderScheduler()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.HTTPShutdownTimeoutSec)*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
		if closeErr := srv.Close(); closeErr != nil {
			log.Printf("force close server failed: %v", closeErr)
		}
	}

	log.Printf("server stopped")
}

func setupSession(router *gin.Engine, cfg config.AppConfig) {
	store := cookie.NewStore([]byte(cfg.SecretKey))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   cfg.SessionMaxAge,
		HttpOnly: true,
		Secure:   cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	router.Use(sessions.Sessions("duty-log-session", store))
}

func loadTemplates(router *gin.Engine, root string) {
	files := make([]string, 0, 64)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(path), ".html") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		log.Fatalf("walk template dir failed: %v", err)
	}
	if len(files) == 0 {
		log.Fatalf("no templates found under %s", root)
	}

	tpl := template.New("").Funcs(template.FuncMap{
		"isAdmin": isAdminValue,
	})
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			log.Fatalf("read template file failed (%s): %v", file, err)
		}
		relative, err := filepath.Rel(root, file)
		if err != nil {
			log.Fatalf("build relative template path failed (%s): %v", file, err)
		}
		name := filepath.ToSlash(relative)
		if _, err := tpl.New(name).Parse(string(content)); err != nil {
			log.Fatalf("parse template failed (%s): %v", file, err)
		}
	}
	if len(tpl.Templates()) == 0 {
		log.Fatalf("parsed template set is empty under %s", root)
	}
	if tpl.Lookup("dashboard.html") == nil {
		log.Fatalf("required template missing: dashboard.html")
	}
	if tpl.Lookup("coming_soon.html") == nil {
		log.Fatalf("required template missing: coming_soon.html")
	}
	if tpl.Lookup("auth/login.html") == nil {
		log.Fatalf("required template missing: auth/login.html")
	}
	log.Printf("loaded %d templates", len(tpl.Templates()))
	router.SetHTMLTemplate(tpl)
}

func isAdminValue(data any) bool {
	if data == nil {
		return false
	}
	v := reflect.ValueOf(data)
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return false
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Map:
		key := reflect.ValueOf("IsAdmin")
		item := v.MapIndex(key)
		if !item.IsValid() {
			return false
		}
		return toBool(item.Interface())
	case reflect.Struct:
		field := v.FieldByName("IsAdmin")
		if !field.IsValid() {
			return false
		}
		return toBool(field.Interface())
	default:
		return false
	}
}

func toBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case *bool:
		if v == nil {
			return false
		}
		return *v
	default:
		return false
	}
}

func init() {
	mustMkdir("static/uploads")
	mustMkdir("fonts")
}

func mustMkdir(path string) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		log.Fatalf("create dir failed (%s): %v", path, err)
	}
}
