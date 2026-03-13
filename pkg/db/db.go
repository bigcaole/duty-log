package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"duty-log-system/internal/config"
	"duty-log-system/internal/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
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
	if err := ensureIDCOpsTicketDefaults(db); err != nil {
		return err
	}
	if err := db.AutoMigrate(&models.IPAMSection{}); err != nil {
		return err
	}
	if err := ensureIPAMSubnetDefaults(db); err != nil {
		return err
	}
	if err := db.AutoMigrate(models.AllModels()...); err != nil {
		return err
	}
	if err := ensureIPAMConstraints(db); err != nil {
		return err
	}
	return ensureIDCDutyUserDateUniqueIndex(db)
}

func ensureIDCOpsTicketDefaults(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if !db.Migrator().HasTable(&models.IDCOpsTicket{}) {
		return nil
	}
	table, err := resolveTableName(db, &models.IDCOpsTicket{})
	if err != nil {
		return err
	}

	steps := []string{
		fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS duty_person varchar(100)`, table),
		fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS processing_status varchar(20)`, table),
		fmt.Sprintf(`UPDATE %s SET duty_person = '' WHERE duty_person IS NULL`, table),
		fmt.Sprintf(`UPDATE %s SET processing_status = 'pending' WHERE processing_status IS NULL OR processing_status = ''`, table),
		fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN duty_person SET DEFAULT ''`, table),
		fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN duty_person SET NOT NULL`, table),
		fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN processing_status SET DEFAULT 'pending'`, table),
		fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN processing_status SET NOT NULL`, table),
	}
	for _, sql := range steps {
		if err := db.Exec(sql).Error; err != nil {
			return fmt.Errorf("ensure idc ops ticket defaults failed: %w", err)
		}
	}
	return nil
}

func resolveTableName(db *gorm.DB, model any) (string, error) {
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(model); err != nil {
		return "", err
	}
	if stmt.Schema == nil {
		return "", schema.ErrUnsupportedDataType
	}
	return stmt.Schema.Table, nil
}

func ensureIPAMConstraints(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if !db.Migrator().HasTable(&models.IPAMSubnet{}) {
		return nil
	}
	if err := db.Exec(`CREATE EXTENSION IF NOT EXISTS btree_gist`).Error; err != nil {
		return fmt.Errorf("enable btree_gist failed: %w", err)
	}
	table, err := resolveTableName(db, &models.IPAMSubnet{})
	if err != nil {
		return err
	}
	if err := ensureIPAMNetworkType(db, table); err != nil {
		return err
	}
	var exists bool
	if err := db.Raw(`SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = ?)`, "ipam_subnets_no_overlap").Scan(&exists).Error; err != nil {
		return fmt.Errorf("check ipam constraint failed: %w", err)
	}
	if !exists {
		sql := fmt.Sprintf(`ALTER TABLE %s ADD CONSTRAINT ipam_subnets_no_overlap EXCLUDE USING gist (network inet_ops WITH &&)`, table)
		if err := db.Exec(sql).Error; err != nil {
			return fmt.Errorf("create ipam overlap constraint failed: %w", err)
		}
	}
	return nil
}

func ensureIPAMSubnetDefaults(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if !db.Migrator().HasTable(&models.IPAMSubnet{}) {
		return nil
	}

	defaultSectionID, err := ensureDefaultIPAMSection(db)
	if err != nil {
		return err
	}

	table, err := resolveTableName(db, &models.IPAMSubnet{})
	if err != nil {
		return err
	}

	steps := []string{
		fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS section_id bigint`, table),
		fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS parent_id bigint`, table),
		fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS vrf varchar(120)`, table),
		fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS description text`, table),
		fmt.Sprintf(`UPDATE %s SET section_id = %d WHERE section_id IS NULL OR section_id = 0`, table, defaultSectionID),
		fmt.Sprintf(`UPDATE %s SET vrf = 'default' WHERE vrf IS NULL OR vrf = ''`, table),
		fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN section_id SET DEFAULT %d`, table, defaultSectionID),
		fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN section_id SET NOT NULL`, table),
		fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN vrf SET DEFAULT 'default'`, table),
		fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN vrf SET NOT NULL`, table),
	}
	for _, sql := range steps {
		if err := db.Exec(sql).Error; err != nil {
			return fmt.Errorf("ensure ipam subnet defaults failed: %w", err)
		}
	}
	return nil
}

func ensureDefaultIPAMSection(db *gorm.DB) (uint, error) {
	var count int64
	if err := db.Model(&models.IPAMSection{}).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("check ipam section failed: %w", err)
	}
	if count > 0 {
		var section models.IPAMSection
		if err := db.Order("id").First(&section).Error; err != nil {
			return 0, fmt.Errorf("read ipam section failed: %w", err)
		}
		return section.ID, nil
	}
	defaultSection := models.IPAMSection{Name: "默认大区", RootCIDR: "", Description: "系统自动创建"}
	if err := db.Create(&defaultSection).Error; err != nil {
		return 0, fmt.Errorf("create default ipam section failed: %w", err)
	}
	return defaultSection.ID, nil
}

func ensureIPAMNetworkType(db *gorm.DB, table string) error {
	if db == nil || table == "" {
		return nil
	}
	schemaName, tableName := splitSchemaTable(table)
	var dataType string
	var udtName string
	row := db.Raw(
		`SELECT data_type, udt_name FROM information_schema.columns WHERE table_schema = ? AND table_name = ? AND column_name = 'network'`,
		schemaName,
		tableName,
	).Row()
	if err := row.Scan(&dataType, &udtName); err != nil {
		return fmt.Errorf("check ipam network type failed: %w", err)
	}
	if dataType == "" && udtName == "" {
		return fmt.Errorf("ipam network column not found")
	}
	if !(strings.EqualFold(dataType, "cidr") || strings.EqualFold(udtName, "cidr") || strings.EqualFold(dataType, "inet") || strings.EqualFold(udtName, "inet")) {
		return fmt.Errorf("unsupported ipam network column type: %s/%s", dataType, udtName)
	}
	sql := fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN network TYPE cidr USING network::cidr`, table)
	if err := db.Exec(sql).Error; err != nil {
		return fmt.Errorf("convert ipam network type failed: %w", err)
	}
	return nil
}

func splitSchemaTable(full string) (string, string) {
	parts := strings.SplitN(full, ".", 2)
	if len(parts) == 2 {
		return strings.Trim(parts[0], `"`), strings.Trim(parts[1], `"`)
	}
	return "public", strings.Trim(full, `"`)
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
