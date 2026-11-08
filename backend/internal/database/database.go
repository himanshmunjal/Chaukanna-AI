// Package database opens the GORM connection and creates the tables.
//
// SQLite is used by default (no setup needed). Set DATABASE_URL to a Postgres
// DSN to use Postgres instead.
package database

import (
	"fmt"
	"log"

	"github.com/glebarez/sqlite" // pure-Go SQLite driver (no CGO needed)
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"chaukanna/internal/config"
	"chaukanna/internal/models"
)

// Connect opens the database and runs auto-migrations.
func Connect(cfg config.Config) (*gorm.DB, error) {
	var dialector gorm.Dialector
	if cfg.DatabaseURL != "" {
		dialector = postgres.Open(cfg.DatabaseURL)
		log.Println("database: using Postgres")
	} else {
		// WAL + busy timeout let several goroutines write at the same time safely.
		dsn := cfg.SQLitePath + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
		dialector = sqlite.Open(dsn)
		log.Printf("database: using SQLite file %s", cfg.SQLitePath)
	}

	if err := models.SetEncryptionKey(cfg.EncryptionKey); err != nil {
		return nil, fmt.Errorf("encryption key: %w", err)
	}

	db, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.AutoMigrate(
		&models.ProtectedUser{},
		&models.Call{},
		&models.TranscriptLine{},
		&models.RedFlag{},
		&models.Alert{},
		&models.ScamNumber{},
	); err != nil {
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	return db, nil
}

// SeedDefaultUser creates (or updates) the protected user described in .env,
// so a fresh install can take calls immediately.
func SeedDefaultUser(db *gorm.DB, cfg config.Config) {
	phone := models.NormalizePhone(cfg.DefaultUserPhone)
	if phone == "" {
		log.Println("database: DEFAULT_USER_PHONE is empty, skipping default user")
		return
	}

	user := models.ProtectedUser{Phone: phone}
	db.Where(models.ProtectedUser{Phone: phone}).
		Assign(models.ProtectedUser{
			Name:        cfg.DefaultUserName,
			Language:    cfg.DefaultUserLanguage,
			FamilyName:  cfg.DefaultFamilyName,
			FamilyPhone: models.NormalizePhone(cfg.DefaultFamilyPhone),
		}).
		FirstOrCreate(&user)
	log.Printf("database: default protected user %q (%s) ready", user.Name, user.Phone)
}
