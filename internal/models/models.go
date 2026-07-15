// Package models defines the GORM models for conveyor's relational database
// (users, tags, user-tag assignments), mirroring the Sequelize models in the
// original TS src/database.ts. Table names are explicitly set to match the
// TS schema (user, tag, usertag) for database-level compatibility.
package models

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/steemit/conveyor/internal/config"
)

// User stores sensitive user data (email, phone). Conveyor is the central
// store for this data — no other service should persist it.
// Matches TS model "user" (database.ts:30).
type User struct {
	Account string  `gorm:"primaryKey" json:"account"`
	Email   *string `gorm:"uniqueIndex" json:"email,omitempty"`
	Phone   *string `gorm:"uniqueIndex" json:"phone,omitempty"`
}

// TableName forces the singular "user" to match the TS/Sequelize table name
// (GORM would otherwise pluralise to "users").
func (User) TableName() string { return "user" }

// Tag is a user-defined classification label.
// Matches TS model "tag" (database.ts:65).
type Tag struct {
	Name        string `gorm:"primaryKey" json:"name"`
	Description string `gorm:"not null"   json:"description"`
}

func (Tag) TableName() string { return "tag" }

// UserTag associates a uid (account or other identifier) with a tag.
// Matches TS model "usertag" (database.ts:95).
//
// DESIGN DECISION — no DB-level foreign key constraint on Tag:
//
// The existing production database has a FK usertag.tag → tag.name, created
// by Sequelize. The Go version does NOT create this FK via GORM (GORM's
// philosophy is that relations are managed at the application layer). This
// means:
//   - The existing production FK remains in place (AutoMigrate never drops
//     constraints), so inserting a usertag with a non-existent tag still
//     triggers a DB FK error on the current database.
//   - If the database is ever recreated, the FK will NOT exist, and the
//     "tag not found" check MUST be performed at the application layer
//     (assign_tag handler queries Tag first before inserting UserTag).
//
// DESIGN DECISION — DeletedAt is *time.Time, NOT gorm.DeletedAt:
//
// The TS code uses deletedAt as a plain nullable column (not Sequelize's
// built-in paranoid mode). getTagsForUser(audit=true) queries ALL records
// including soft-deleted ones. GORM's gorm.DeletedAt auto-filters soft-deleted
// rows from every query, which would break the audit query. Using *time.Time
// gives us full manual control over the soft-delete behaviour.
type UserTag struct {
	ID        uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	UID       string     `gorm:"not null;index"          json:"uid"`
	Tag       string     `gorm:"not null"                json:"tag"` // no FK — see above
	Memo      string     `gorm:"not null"                json:"memo"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`              // manual soft-delete
	CreatedAt time.Time  `json:"createdAt"`                        // GORM auto; audit sort uses this
}

func (UserTag) TableName() string { return "usertag" }

// NewDB opens a GORM connection and auto-migrates all models, based on the
// configured dialect (sqlite for dev, postgres for production).
func NewDB(cfg config.DatabaseConfig) (*gorm.DB, error) {
	var db *gorm.DB
	var err error

	switch cfg.Dialect {
	case "sqlite":
		dsn := "file:conveyor.db?cache=shared"
		db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Warn),
		})
	case "postgres":
		dsn := fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=require",
			cfg.Host, cfg.Port, cfg.Username, cfg.Password, cfg.Name,
		)
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Warn),
		})
	default:
		return nil, fmt.Errorf("unsupported database dialect: %s", cfg.Dialect)
	}
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.AutoMigrate(&User{}, &Tag{}, &UserTag{}); err != nil {
		return nil, fmt.Errorf("auto-migrate: %w", err)
	}

	return db, nil
}

// NewTestDB creates an in-memory SQLite database (for unit tests). Each call
// returns an independent database — tests never share state.
func NewTestDB() (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&User{}, &Tag{}, &UserTag{}); err != nil {
		return nil, err
	}
	return db, nil
}
