package infra

import (
	"fmt"
	"strings"

	"github.com/patdyyew-emran/phishing-detector/config"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func NewDB(cfg *config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DBUrl), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL: %w", err)
	}

	fail := func(cause error) (*gorm.DB, error) {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}

		return nil, cause
	}

	if err := db.AutoMigrate(&UserModel{}); err != nil {
		return fail(fmt.Errorf("migrate users: %w", err))
	}

	if err := seed(db, cfg); err != nil {
		return fail(fmt.Errorf("seed users: %w", err))
	}

	if err := migrateScanOwnership(db, cfg.AdminEmail); err != nil {
		return fail(fmt.Errorf("migrate scan ownership: %w", err))
	}

	if err := db.AutoMigrate(
		&ScanModel{},
		&IndicatorModel{},
		&AuditLogModel{},
	); err != nil {
		return fail(fmt.Errorf("migrate application tables: %w", err))
	}

	if err := migrateUpdateScanUserForeignKey(db); err != nil {
		return fail(fmt.Errorf("migrate scan foreign key: %w", err))
	}

	return db, nil
}

func seed(db *gorm.DB, cfg *config.Config) error {
	adminEmail := strings.ToLower(strings.TrimSpace(cfg.AdminEmail))
	testEmail := strings.ToLower(strings.TrimSpace(cfg.TestEmail))

	if adminEmail == "" || cfg.AdminPassword == "" {
		return fmt.Errorf("admin email and password are required")
	}

	if testEmail == "" || cfg.TestPassword == "" {
		return fmt.Errorf("test email and password are required")
	}

	var adminCount int64

	if err := db.Model(&UserModel{}).
		Where("email = ?", adminEmail).
		Count(&adminCount).Error; err != nil {
		return err
	}

	if adminCount == 0 {
		hash, err := bcrypt.GenerateFromPassword(
			[]byte(cfg.AdminPassword),
			bcrypt.DefaultCost,
		)
		if err != nil {
			return fmt.Errorf("hash admin password: %w", err)
		}

		admin := UserModel{
			Email:        adminEmail,
			PasswordHash: string(hash),
			Role:         "admin",
		}

		if err := db.Create(&admin).Error; err != nil {
			return fmt.Errorf("create admin user: %w", err)
		}
	}

	var testCount int64

	if err := db.Model(&UserModel{}).
		Where("email = ?", testEmail).
		Count(&testCount).Error; err != nil {
		return err
	}

	if testCount == 0 {
		hash, err := bcrypt.GenerateFromPassword(
			[]byte(cfg.TestPassword),
			bcrypt.DefaultCost,
		)
		if err != nil {
			return fmt.Errorf("hash test password: %w", err)
		}

		user := UserModel{
			Email:        testEmail,
			PasswordHash: string(hash),
			Role:         "user",
		}

		if err := db.Create(&user).Error; err != nil {
			return fmt.Errorf("create test user: %w", err)
		}
	}

	return nil
}

func migrateScanOwnership(db *gorm.DB, adminEmail string) error {
	if !db.Migrator().HasTable(&ScanModel{}) {
		return nil
	}

	adminEmail = strings.ToLower(strings.TrimSpace(adminEmail))

	var admin UserModel

	if err := db.Where("email = ?", adminEmail).First(&admin).Error; err != nil {
		return fmt.Errorf("find admin for legacy scans: %w", err)
	}

	if err := db.Exec(`
		ALTER TABLE scan_models
		ADD COLUMN IF NOT EXISTS user_id bigint
	`).Error; err != nil {
		return fmt.Errorf("add scan_models.user_id: %w", err)
	}

	if err := db.Exec(
		`UPDATE scan_models
		 SET user_id = ?
		 WHERE user_id IS NULL OR user_id = 0`,
		admin.ID,
	).Error; err != nil {
		return fmt.Errorf("assign legacy scans to admin: %w", err)
	}

	if err := db.Exec(`
		ALTER TABLE scan_models
		ALTER COLUMN user_id SET NOT NULL
	`).Error; err != nil {
		return fmt.Errorf("make scan_models.user_id required: %w", err)
	}

	return nil
}

func migrateUpdateScanUserForeignKey(db *gorm.DB) error {
	if !db.Migrator().HasTable(&ScanModel{}) {
		return nil
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`
			ALTER TABLE scan_models
			DROP CONSTRAINT IF EXISTS fk_user_models_scans
		`).Error; err != nil {
			return err
		}

		return tx.Exec(`
			ALTER TABLE scan_models
			ADD CONSTRAINT fk_user_models_scans
			FOREIGN KEY (user_id)
			REFERENCES user_models(id)
			ON UPDATE CASCADE
			ON DELETE CASCADE
		`).Error
	})
}
