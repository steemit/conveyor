package models

import (
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/steemit/conveyor/internal/config"
)

func freshDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := NewTestDB()
	if err != nil {
		t.Fatalf("NewTestDB: %v", err)
	}
	return db
}

func TestUser_CRUD(t *testing.T) {
	db := freshDB(t)
	email := "alice@example.com"
	phone := "+1234567890"
	u := User{Account: "alice", Email: &email, Phone: &phone}
	if err := db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	var got User
	if err := db.First(&got, "account = ?", "alice").Error; err != nil {
		t.Fatal(err)
	}
	if *got.Email != email || *got.Phone != phone {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestUser_UniqueEmail(t *testing.T) {
	db := freshDB(t)
	email := "shared@example.com"
	db.Create(&User{Account: "a", Email: &email})
	// Second user with the same email should fail on the unique index.
	err := db.Create(&User{Account: "b", Email: &email}).Error
	if err == nil {
		t.Fatal("expected unique constraint violation for duplicate email")
	}
}

func TestUser_NullableEmail(t *testing.T) {
	db := freshDB(t)
	// Users with NULL email should both succeed (SQL allows multiple NULLs
	// in a unique index).
	if err := db.Create(&User{Account: "x"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&User{Account: "y"}).Error; err != nil {
		t.Fatal(err)
	}
}

func TestTag_CRUD(t *testing.T) {
	db := freshDB(t)
	if err := db.Create(&Tag{Name: "abuse", Description: "bad actor"}).Error; err != nil {
		t.Fatal(err)
	}
	var got Tag
	if err := db.First(&got, "name = ?", "abuse").Error; err != nil {
		t.Fatal(err)
	}
	if got.Description != "bad actor" {
		t.Fatalf("unexpected description: %s", got.Description)
	}
}

func TestUserTag_SoftDelete_Manual(t *testing.T) {
	db := freshDB(t)
	db.Create(&Tag{Name: "verified", Description: "verified user"})

	// Create an active usertag.
	ut := UserTag{UID: "alice", Tag: "verified", Memo: "test"}
	db.Create(&ut)

	// Verify it shows as active (DeletedAt IS NULL).
	var active []UserTag
	db.Where("uid = ? AND deleted_at IS NULL", "alice").Find(&active)
	if len(active) != 1 {
		t.Fatalf("expected 1 active tag, got %d", len(active))
	}

	// Soft-delete it (manual DeletedAt, not gorm.DeletedAt).
	now := time.Now()
	db.Model(&UserTag{}).Where("uid = ? AND tag = ?", "alice", "verified").
		Update("deleted_at", now)

	// Active query should now return 0.
	db.Where("uid = ? AND deleted_at IS NULL", "alice").Find(&active)
	if len(active) != 0 {
		t.Fatalf("expected 0 active after soft-delete, got %d", len(active))
	}

	// Audit query (no DeletedAt filter) should return 1.
	var all []UserTag
	db.Where("uid = ?", "alice").Find(&all)
	if len(all) != 1 {
		t.Fatalf("audit query should include soft-deleted, got %d", len(all))
	}
	if all[0].DeletedAt == nil {
		t.Fatal("DeletedAt should be set")
	}
}

func TestUserTag_AuditOrderByCreatedAt(t *testing.T) {
	db := freshDB(t)
	db.Create(&Tag{Name: "t1", Description: "d"})
	db.Create(&Tag{Name: "t2", Description: "d"})

	db.Create(&UserTag{UID: "bob", Tag: "t1", Memo: "first"})
	time.Sleep(10 * time.Millisecond)
	db.Create(&UserTag{UID: "bob", Tag: "t2", Memo: "second"})

	var all []UserTag
	db.Where("uid = ?", "bob").Order("created_at ASC").Find(&all)
	if len(all) != 2 {
		t.Fatalf("expected 2, got %d", len(all))
	}
	if all[0].Memo != "first" || all[1].Memo != "second" {
		t.Fatalf("order mismatch: %+v", all)
	}
}

func TestTableName(t *testing.T) {
	if (User{}).TableName() != "user" {
		t.Error("User table should be 'user'")
	}
	if (Tag{}).TableName() != "tag" {
		t.Error("Tag table should be 'tag'")
	}
	if (UserTag{}).TableName() != "usertag" {
		t.Error("UserTag table should be 'usertag'")
	}
}

// TestPostgresDSN verifies the libpq DSN assembly (audit 2026-08-18 T-008):
// sslmode is deployment-controlled — defaults to "prefer" (friction-free for
// self-hosted postgres without TLS), and verify-full deployments attach a CA
// bundle via sslrootcert.
func TestPostgresDSN(t *testing.T) {
	base := config.DatabaseConfig{
		Dialect: "postgres", Host: "db.internal", Port: "5432",
		Username: "conveyor", Password: "secret", Name: "conveyor",
	}

	// Empty ssl_mode falls back to prefer (code-level belt in addition to the
	// viper default).
	got := postgresDSN(base)
	want := "host=db.internal port=5432 user=conveyor password=secret dbname=conveyor sslmode=prefer"
	if got != want {
		t.Fatalf("default DSN mismatch:\n got: %s\nwant: %s", got, want)
	}

	// verify-full + CA bundle path (quoted to survive unusual paths).
	full := base
	full.SSLMode = "verify-full"
	full.SSLRootCert = "certs/rds-us-east-1-bundle.pem"
	got = postgresDSN(full)
	want = "host=db.internal port=5432 user=conveyor password=secret dbname=conveyor sslmode=verify-full sslrootcert='certs/rds-us-east-1-bundle.pem'"
	if got != want {
		t.Fatalf("verify-full DSN mismatch:\n got: %s\nwant: %s", got, want)
	}
}
