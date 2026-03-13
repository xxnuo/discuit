package core

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/discuitnet/discuit/internal/httperr"
	msql "github.com/discuitnet/discuit/internal/sql"
	"gorm.io/gorm"
)

// AnalyticsEven represents a record in the analytics table.
type AnalyticsEvent struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	UniqueKey []byte    `json:"uniqueKey"`
	Payload   string    `json:"payload"`
	CreatedAt time.Time `json:"createdAt"`
}

// CreateAnalyticsEvent adds a record to the analytics table. If uniqueKey is
// empty, it is ignored.
func CreateAnalyticsEvent(ctx context.Context, db *gorm.DB, name string, uniqueKey string, payload string) error {
	var uniqueKeyHash []byte
	if uniqueKey != "" {
		sum := md5.Sum([]byte(uniqueKey))
		uniqueKeyHash = sum[:]
	}

	query, args := msql.BuildInsertQuery("analytics", []msql.ColumnValue{
		{Name: "event_name", Value: name},
		{Name: "unique_key", Value: uniqueKeyHash},
		{Name: "payload", Value: payload},
	})

	_, err := msql.ExecContext(ctx, db, query, args...)
	if err != nil && !msql.IsErrDuplicateErr(err) {
		return err
	}
	return nil
}

type BasicSiteStats struct {
	Version              int `json:"version"`
	UsersLastDay         int `json:"users_day"` // last 24 hours
	UsersLastWeek        int `json:"users_week"`
	UsersLastMonth       int `json:"users_month"`
	ReturnUsersLastDay   int `json:"return_users_day"` // last 24 hours
	ReturnUsersLastWeek  int `json:"return_users_week"`
	ReturnUsersLastMonth int `json:"return_users_month"`
	TotalSignups         int `json:"signups"`
	PWAInstalls          int `json:"pwa_installs"`
	PushNotifications    int `json:"notifications_enabled"`
	PostsLastDay         int `json:"posts_day"` // last 24 hours
	PostsLastWeek        int `json:"posts_week"`
	CommentsLastDay      int `json:"comments_day"` // last 24 hours
	CommentsLastWeek     int `json:"comments_week"`
}

const BasicSiteStatsEventName = "bss"

func RecordBasicSiteStats(ctx context.Context, db *gorm.DB) error {
	stats := &BasicSiteStats{Version: 0}
	now := time.Now()
	lastDay := now.Add(-24 * time.Hour)
	lastWeek := now.Add(-7 * 24 * time.Hour)
	lastMonth := now.Add(-30 * 24 * time.Hour)

	if err := msql.QueryRow(db, "select count(*) from users where last_seen > ?", lastDay).Scan(&stats.UsersLastDay); err != nil {
		return err
	}
	if err := msql.QueryRow(db, "select count(*) from users where last_seen > ?", lastWeek).Scan(&stats.UsersLastWeek); err != nil {
		return err
	}
	if err := msql.QueryRow(db, "select count(*) from users where last_seen > ?", lastMonth).Scan(&stats.UsersLastMonth); err != nil {
		return err
	}
	if err := msql.QueryRow(db, "select count(*) from users where last_seen > ? and created_at <= ?", lastDay, lastDay).Scan(&stats.ReturnUsersLastDay); err != nil {
		return err
	}
	if err := msql.QueryRow(db, "select count(*) from users where last_seen > ? and created_at <= ?", lastWeek, lastWeek).Scan(&stats.ReturnUsersLastWeek); err != nil {
		return err
	}
	if err := msql.QueryRow(db, "select count(*) from users where last_seen > ? and created_at <= ?", lastMonth, lastMonth).Scan(&stats.ReturnUsersLastMonth); err != nil {
		return err
	}
	if err := msql.QueryRow(db, "select count(*) from users").Scan(&stats.TotalSignups); err != nil {
		return err
	}
	if err := msql.QueryRow(db, "select count(*) from analytics").Scan(&stats.PWAInstalls); err != nil {
		return err
	}
	if err := msql.QueryRow(db, "select count(*) from web_push_subscriptions").Scan(&stats.PushNotifications); err != nil {
		return err
	}
	if err := msql.QueryRow(db, "select count(*) from posts where created_at > ?", lastDay).Scan(&stats.PostsLastDay); err != nil {
		return err
	}
	if err := msql.QueryRow(db, "select count(*) from posts where created_at > ?", lastWeek).Scan(&stats.PostsLastWeek); err != nil {
		return err
	}
	if err := msql.QueryRow(db, "select count(*) from comments where created_at > ?", lastDay).Scan(&stats.CommentsLastDay); err != nil {
		return err
	}
	if err := msql.QueryRow(db, "select count(*) from comments where created_at > ?", lastWeek).Scan(&stats.CommentsLastWeek); err != nil {
		return err
	}

	b, _ := json.Marshal(stats)
	return CreateAnalyticsEvent(ctx, db, BasicSiteStatsEventName, "", string(b))
}

func GetBasicSiteStats(ctx context.Context, db *gorm.DB, limit int, next string) ([]*AnalyticsEvent, string, error) {
	limitQuery := ""
	if limit > 0 {
		limitQuery = "LIMIT " + strconv.Itoa(limit+1)
	}

	query := "SELECT id, payload, created_at FROM analytics WHERE event_name = ?"
	args := []any{BasicSiteStatsEventName}

	nextQuery := ""
	if next != "" {
		nextInt, err := strconv.ParseInt(next, 10, 64)
		if err != nil {
			return nil, "", httperr.NewBadRequest("invalid-next-value", "Invalid next parameter.")
		}
		nextTime := time.Unix(nextInt, 0)
		nextQuery = "AND created_at <= ?"
		args = append(args, nextTime)
	}

	query = fmt.Sprintf("%s %s ORDER BY created_at DESC %s", query, nextQuery, limitQuery)

	rows, err := msql.QueryContext(ctx, db, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var events []*AnalyticsEvent
	for rows.Next() {
		e := &AnalyticsEvent{}
		if err := rows.Scan(&e.ID, &e.Payload, &e.CreatedAt); err != nil {
			return nil, "", err
		}
		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	if len(events) > limit {
		// return events[:limit], hex.EncodeToString([]byte(events[limit].CreatedAt.Format(time.RFC3339))), nil
		return events[:limit], strconv.FormatInt(events[limit].CreatedAt.Unix(), 10), nil
	}

	return events, "", nil
}
