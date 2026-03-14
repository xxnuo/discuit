package core

import (
	"context"
	"testing"
	"time"

	idb "github.com/discuitnet/discuit/internal/db"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGetCommunityRequestsSQLite(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	if err := idb.Migrate(ctx, db, 0); err != nil {
		t.Fatal(err)
	}

	if err := CreateCommunityRequest(ctx, db, "alice", "FreshCommunity", "note"); err != nil {
		t.Fatal(err)
	}

	oldTimestamp := time.Now().AddDate(0, 0, -91)
	if err := db.Exec(
		"INSERT INTO community_requests (by_user, community_name, community_name_lc, note, created_at) VALUES (?, ?, ?, ?, ?)",
		"bob",
		"OldCommunity",
		"oldcommunity",
		"old note",
		oldTimestamp,
	).Error; err != nil {
		t.Fatal(err)
	}

	requests, err := GetCommunityRequests(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	if len(requests) != 1 {
		t.Fatalf("unexpected request count %d", len(requests))
	}

	if requests[0].ByUser != "alice" {
		t.Fatalf("unexpected request user %q", requests[0].ByUser)
	}
}
