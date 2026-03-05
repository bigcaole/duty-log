package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"duty-log-system/internal/config"
	"duty-log-system/internal/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect(cfg config.AppConfig) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.PostgresDSN()), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, err
	}
	return db, nil
}

func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(models.AllModels()...); err != nil {
		return err
	}
	return ensureIDCDutyUserDateUniqueIndex(db)
}

func ensureIDCDutyUserDateUniqueIndex(db *gorm.DB) error {
	for _, sql := range idcDutyLegacyUniqueDateConstraintSQL() {
		if err := db.Exec(sql).Error; err != nil {
			return fmt.Errorf("drop legacy idc duty date constraint/index failed: %w", err)
		}
	}
	createSQL := fmt.Sprintf(
		`CREATE UNIQUE INDEX IF NOT EXISTS %s ON idc_duty_records (user_id, date)`,
		idcDutyUserDateUniqueIndexName(),
	)
	if err := db.Exec(createSQL).Error; err != nil {
		return fmt.Errorf("create idc duty user-date unique index failed: %w", err)
	}
	return nil
}

func idcDutyUserDateUniqueIndexName() string {
	return "idx_idc_duty_user_date"
}

func idcDutyLegacyUniqueDateConstraintSQL() []string {
	return []string{
		`ALTER TABLE idc_duty_records DROP CONSTRAINT IF EXISTS idc_duty_records_date_key`,
		`DROP INDEX IF EXISTS idx_idc_duty_records_date`,
		`DROP INDEX IF EXISTS idc_duty_records_date_idx`,
	}
}

func SeedDefaultAdmin(db *gorm.DB) error {
	const (
		defaultAdminUsername = "admin"
		defaultAdminPassword = "admin123"
		defaultAdminEmail    = "admin@example.com"
	)

	var existing models.User
	err := db.Where("username = ?", defaultAdminUsername).First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(defaultAdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	admin := models.User{
		Username:     defaultAdminUsername,
		PasswordHash: string(hash),
		Email:        defaultAdminEmail,
		IsActive:     true,
		IsAdmin:      true,
	}
	return db.Create(&admin).Error
}

func HealthCheck(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return sqlDB.PingContext(ctx)
}
