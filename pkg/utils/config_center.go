package utils

import (
	"encoding/base64"
	"errors"
	"os"
	"strconv"
	"strings"

	"duty-log-system/internal/crypto"
	"duty-log-system/internal/models"

	"gorm.io/gorm"
)

var sensitiveConfigKeys = map[string]struct{}{
	"AI_API_KEY":         {},
	"MAIL_PASSWORD":      {},
	"FEISHU_WEBHOOK_URL": {},
	"NEXTCLOUD_PASSWORD": {},
}

type ConfigCenter struct {
	db     *gorm.DB
	aesKey []byte
}

const (
	aesGCMNonceSize             = 12
	aesGCMTagSize               = 16
	sensitiveConfigCipherPrefix = "enc::"
)

func NewConfigCenter(db *gorm.DB, secretKey string) *ConfigCenter {
	return &ConfigCenter{
		db:     db,
		aesKey: crypto.DeriveAES256Key(secretKey),
	}
}

func (c *ConfigCenter) Get(key, defaultValue string) string {
	if c == nil || c.db == nil {
		return envOrDefault(key, defaultValue)
	}

	var cfg models.SystemConfig
	err := c.db.Where("key = ?", key).First(&cfg).Error
	if err == nil {
		value := strings.TrimSpace(cfg.Value)
		if value == "" {
			return envOrDefault(key, defaultValue)
		}
		if _, ok := sensitiveConfigKeys[key]; ok {
			plaintext, normalizedStored, resolved := resolveSensitiveConfigValue(c.aesKey, value)
			if resolved {
				if normalizedStored != "" && normalizedStored != value {
					_ = c.db.Model(&models.SystemConfig{}).
						Where("id = ?", cfg.ID).
						Update("value", normalizedStored).Error
				}
				return plaintext
			}
			return envOrDefault(key, defaultValue)
		}
		return value
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return envOrDefault(key, defaultValue)
	}
	return envOrDefault(key, defaultValue)
}

func (c *ConfigCenter) GetBool(key string, defaultValue bool) bool {
	defaultText := "false"
	if defaultValue {
		defaultText = "true"
	}
	value := strings.TrimSpace(c.Get(key, defaultText))
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func (c *ConfigCenter) GetInt(key string, defaultValue int) int {
	value := strings.TrimSpace(c.Get(key, strconv.Itoa(defaultValue)))
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func (c *ConfigCenter) Source(key string) string {
	if c == nil || c.db == nil {
		return "env"
	}
	var count int64
	if err := c.db.Model(&models.SystemConfig{}).Where("key = ?", key).Count(&count).Error; err != nil {
		return "env"
	}
	if count > 0 {
		return "db"
	}
	return "env"
}

func (c *ConfigCenter) Upsert(key, value, description string) error {
	if c == nil || c.db == nil {
		return errors.New("config center not initialized")
	}
	storeValue := strings.TrimSpace(value)
	if _, ok := sensitiveConfigKeys[key]; ok && storeValue != "" {
		encrypted, err := crypto.EncryptAES256GCM(c.aesKey, storeValue)
		if err != nil {
			return err
		}
		storeValue = wrapSensitiveCipher(encrypted)
	}

	var cfg models.SystemConfig
	err := c.db.Where("key = ?", key).First(&cfg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c.db.Create(&models.SystemConfig{
			Key:         key,
			Value:       storeValue,
			Description: description,
		}).Error
	}
	if err != nil {
		return err
	}

	cfg.Value = storeValue
	if strings.TrimSpace(description) != "" {
		cfg.Description = description
	}
	return c.db.Save(&cfg).Error
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value != "" {
		return value
	}
	return fallback
}

func resolveSensitiveConfigValue(aesKey []byte, storedValue string) (plaintext string, normalizedStored string, resolved bool) {
	raw := strings.TrimSpace(storedValue)
	if raw == "" {
		return "", "", true
	}

	cipherText, hasPrefix := unwrapSensitiveCipher(raw)
	if decoded, err := crypto.DecryptAES256GCM(aesKey, cipherText); err == nil {
		if hasPrefix {
			return decoded, raw, true
		}
		return decoded, wrapSensitiveCipher(cipherText), true
	}

	if hasPrefix {
		return "", "", false
	}

	if looksLikeEncryptedConfigValue(raw) {
		return "", "", false
	}

	encrypted, err := crypto.EncryptAES256GCM(aesKey, raw)
	if err != nil {
		return raw, "", true
	}
	return raw, wrapSensitiveCipher(encrypted), true
}

func looksLikeEncryptedConfigValue(value string) bool {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return false
	}
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return false
	}
	return len(decoded) >= aesGCMNonceSize+aesGCMTagSize
}

func wrapSensitiveCipher(cipherText string) string {
	return sensitiveConfigCipherPrefix + strings.TrimSpace(cipherText)
}

func unwrapSensitiveCipher(stored string) (cipherText string, hasPrefix bool) {
	raw := strings.TrimSpace(stored)
	if strings.HasPrefix(raw, sensitiveConfigCipherPrefix) {
		return strings.TrimSpace(strings.TrimPrefix(raw, sensitiveConfigCipherPrefix)), true
	}
	return raw, false
}
