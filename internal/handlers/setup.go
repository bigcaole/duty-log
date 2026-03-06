package handlers

import (
	"net/http"
	"net/url"
	"os"
	"strings"

	"duty-log-system/internal/middleware"
	"duty-log-system/internal/models"
	"duty-log-system/pkg/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type setupConfigItem struct {
	Key         string
	Description string
	Value       string
}

func registerSetupRoutes(group *gin.RouterGroup, app *AppContext) {
	group.GET("/setup/config", app.setupConfigPage)
	group.POST("/setup/config", app.setupConfigSave)
}

func (a *AppContext) RequireInitialSetup() gin.HandlerFunc {
	return func(c *gin.Context) {
		if isInitialSetupExemptPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		currentUser, err := middleware.CurrentUser(c, a.DB)
		if err != nil {
			c.Next()
			return
		}
		if !currentUser.IsAdmin {
			c.Next()
			return
		}

		complete, checkErr := a.IsInitialSetupComplete()
		if checkErr != nil {
			c.Next()
			return
		}
		if complete {
			c.Next()
			return
		}

		c.Redirect(http.StatusFound, "/admin/setup/config")
		c.Abort()
	}
}

func isInitialSetupExemptPath(path string) bool {
	p := strings.TrimSpace(path)
	return strings.HasPrefix(p, "/admin/setup/config")
}

func (a *AppContext) IsInitialSetupComplete() (bool, error) {
	requiredDefs := requiredSetupConfigDefinitions()
	if len(requiredDefs) == 0 {
		return true, nil
	}

	keys := make([]string, 0, len(requiredDefs))
	for _, def := range requiredDefs {
		keys = append(keys, def.Key)
	}

	var rows []models.SystemConfig
	if err := a.DB.Where("key IN ?", keys).Find(&rows).Error; err != nil {
		return false, err
	}
	dbValues := make(map[string]string, len(rows))
	for _, row := range rows {
		dbValues[strings.TrimSpace(row.Key)] = strings.TrimSpace(row.Value)
	}

	for _, def := range requiredDefs {
		key := strings.TrimSpace(def.Key)
		if value, ok := dbValues[key]; ok && value != "" {
			continue
		}
		if envProvided(key) {
			continue
		}
		return false, nil
	}
	return true, nil
}

func envProvided(key string) bool {
	value, ok := os.LookupEnv(strings.TrimSpace(key))
	if !ok {
		return false
	}
	return strings.TrimSpace(value) != ""
}

func (a *AppContext) setupConfigPage(c *gin.Context) {
	items := make([]setupConfigItem, 0, len(requiredSetupConfigDefinitions()))
	for _, def := range requiredSetupConfigDefinitions() {
		current := strings.TrimSpace(a.ConfigCenter.Get(def.Key, defaultValueForSystemConfigKey(def.Key)))
		if current == "" {
			current = strings.TrimSpace(def.DefaultValue)
		}
		items = append(items, setupConfigItem{
			Key:         def.Key,
			Description: def.Description,
			Value:       current,
		})
	}

	c.HTML(http.StatusOK, "admin/setup_config.html", gin.H{
		"Title":         "首次初始化配置",
		"Items":         items,
		"OptionalCount": len(optionalSetupConfigDefinitions()),
		"Msg":           strings.TrimSpace(c.Query("msg")),
		"Error":         strings.TrimSpace(c.Query("error")),
	})
}

func (a *AppContext) setupConfigSave(c *gin.Context) {
	requiredDefs := requiredSetupConfigDefinitions()
	updates := make([]systemConfigUpdate, 0, len(requiredDefs))

	for _, def := range requiredDefs {
		raw := strings.TrimSpace(c.PostForm(def.Key))
		if raw == "" {
			raw = strings.TrimSpace(def.DefaultValue)
		}
		if strings.TrimSpace(raw) == "" {
			c.Redirect(http.StatusFound, "/admin/setup/config?error="+url.QueryEscape(def.Key+" 不能为空"))
			return
		}
		normalized, err := normalizeSystemConfigValue(def.Key, raw)
		if err != nil {
			c.Redirect(http.StatusFound, "/admin/setup/config?error="+url.QueryEscape(err.Error()))
			return
		}
		updates = append(updates, systemConfigUpdate{
			Key:   def.Key,
			Value: normalized,
		})
	}

	if err := a.DB.Transaction(func(tx *gorm.DB) error {
		txConfigCenter := utils.NewConfigCenter(tx, a.Config.SecretKey)
		for _, update := range updates {
			if upsertErr := txConfigCenter.Upsert(update.Key, update.Value, "initial setup wizard"); upsertErr != nil {
				return upsertErr
			}
		}
		return nil
	}); err != nil {
		c.Redirect(http.StatusFound, "/admin/setup/config?error="+url.QueryEscape(err.Error()))
		return
	}

	applyLoginRateLimiterConfig(a.ConfigCenter)
	if err := a.ReloadBackupScheduler(); err != nil {
		c.Redirect(http.StatusFound, "/admin/setup/config?error="+url.QueryEscape("配置已保存，但备份调度器重载失败："+err.Error()))
		return
	}

	c.Redirect(http.StatusFound, "/dashboard?msg="+url.QueryEscape("初始配置已完成，可在后台继续调整可选项"))
}
