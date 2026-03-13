package db

import (
	"context"
	"net"
	"testing"

	"github.com/discuitnet/discuit/internal/uid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUIDRoundTrip(t *testing.T) {
	id := UIDFrom(uid.New())

	value, err := id.Value()
	if err != nil {
		t.Fatal(err)
	}

	var scanned UID
	if err := scanned.Scan(value); err != nil {
		t.Fatal(err)
	}
	if scanned.String() != id.String() {
		t.Fatalf("uid mismatch: %s != %s", scanned.String(), id.String())
	}
}

func TestIPRoundTrip(t *testing.T) {
	ip := IPFrom(net.ParseIP("2001:db8::1"))

	value, err := ip.Value()
	if err != nil {
		t.Fatal(err)
	}

	var scanned IP
	if err := scanned.Scan(value); err != nil {
		t.Fatal(err)
	}
	if scanned.String() != ip.String() {
		t.Fatalf("ip mismatch: %s != %s", scanned.String(), ip.String())
	}
}

func TestSQLiteMigrateAndRollback(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	if err := Migrate(ctx, db, 0); err != nil {
		t.Fatal(err)
	}

	status, err := MigrationStatus(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if status.Version != 1 || status.Dirty {
		t.Fatalf("unexpected status: %+v", status)
	}

	for _, model := range []any{&User{}, &Post{}, &Image{}, &List{}} {
		if !db.Migrator().HasTable(model) {
			t.Fatalf("missing table for %T", model)
		}
	}

	userID := UIDFrom(uid.New())
	if err := db.Exec(
		"INSERT INTO users (id, username, username_lc, password) VALUES (?, ?, ?, ?)",
		userID,
		"alice",
		"alice",
		"hash",
	).Error; err != nil {
		t.Fatal(err)
	}

	var userIndex int
	if err := db.Raw("SELECT user_index FROM users WHERE id = ?", userID).Scan(&userIndex).Error; err != nil {
		t.Fatal(err)
	}
	if userIndex != 1 {
		t.Fatalf("unexpected user_index %d", userIndex)
	}

	var reportReasons int64
	if err := db.Model(&ReportReason{}).Count(&reportReasons).Error; err != nil {
		t.Fatal(err)
	}
	if reportReasons != 4 {
		t.Fatalf("unexpected report reasons %d", reportReasons)
	}

	if err := Migrate(ctx, db, -1); err != nil {
		t.Fatal(err)
	}

	status, err = MigrationStatus(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if status.Version != 0 || status.Dirty {
		t.Fatalf("unexpected status after rollback: %+v", status)
	}

	if db.Migrator().HasTable(&User{}) {
		t.Fatal("users table still exists after rollback")
	}
}
