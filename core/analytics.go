package core

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"strconv"
	"time"

	idb "github.com/discuitnet/discuit/internal/db"
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

	record := idb.Analytics{
		EventName: name,
		Payload:   &payload,
	}
	if len(uniqueKeyHash) > 0 {
		record.UniqueKey = idb.Bytes16(uniqueKeyHash)
	}

	err := db.WithContext(ctx).Create(&record).Error
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

	var count int64
	if err := db.WithContext(ctx).Model(&idb.User{}).Where("last_seen > ?", lastDay).Count(&count).Error; err != nil {
		return err
	}
	stats.UsersLastDay = int(count)
	if err := db.WithContext(ctx).Model(&idb.User{}).Where("last_seen > ?", lastWeek).Count(&count).Error; err != nil {
		return err
	}
	stats.UsersLastWeek = int(count)
	if err := db.WithContext(ctx).Model(&idb.User{}).Where("last_seen > ?", lastMonth).Count(&count).Error; err != nil {
		return err
	}
	stats.UsersLastMonth = int(count)
	if err := db.WithContext(ctx).Model(&idb.User{}).Where("last_seen > ? AND created_at <= ?", lastDay, lastDay).Count(&count).Error; err != nil {
		return err
	}
	stats.ReturnUsersLastDay = int(count)
	if err := db.WithContext(ctx).Model(&idb.User{}).Where("last_seen > ? AND created_at <= ?", lastWeek, lastWeek).Count(&count).Error; err != nil {
		return err
	}
	stats.ReturnUsersLastWeek = int(count)
	if err := db.WithContext(ctx).Model(&idb.User{}).Where("last_seen > ? AND created_at <= ?", lastMonth, lastMonth).Count(&count).Error; err != nil {
		return err
	}
	stats.ReturnUsersLastMonth = int(count)
	if err := db.WithContext(ctx).Model(&idb.User{}).Count(&count).Error; err != nil {
		return err
	}
	stats.TotalSignups = int(count)
	if err := db.WithContext(ctx).Model(&idb.Analytics{}).Count(&count).Error; err != nil {
		return err
	}
	stats.PWAInstalls = int(count)
	if err := db.WithContext(ctx).Model(&idb.WebPushSubscription{}).Count(&count).Error; err != nil {
		return err
	}
	stats.PushNotifications = int(count)
	if err := db.WithContext(ctx).Model(&idb.Post{}).Where("created_at > ?", lastDay).Count(&count).Error; err != nil {
		return err
	}
	stats.PostsLastDay = int(count)
	if err := db.WithContext(ctx).Model(&idb.Post{}).Where("created_at > ?", lastWeek).Count(&count).Error; err != nil {
		return err
	}
	stats.PostsLastWeek = int(count)
	if err := db.WithContext(ctx).Model(&idb.Comment{}).Where("created_at > ?", lastDay).Count(&count).Error; err != nil {
		return err
	}
	stats.CommentsLastDay = int(count)
	if err := db.WithContext(ctx).Model(&idb.Comment{}).Where("created_at > ?", lastWeek).Count(&count).Error; err != nil {
		return err
	}
	stats.CommentsLastWeek = int(count)

	b, _ := json.Marshal(stats)
	return CreateAnalyticsEvent(ctx, db, BasicSiteStatsEventName, "", string(b))
}

func GetBasicSiteStats(ctx context.Context, db *gorm.DB, limit int, next string) ([]*AnalyticsEvent, string, error) {
	query := idb.Select(ctx, db, "analytics", []string{"id", "payload", "created_at"}).
		Where("event_name = ?", BasicSiteStatsEventName)
	if next != "" {
		nextInt, err := strconv.ParseInt(next, 10, 64)
		if err != nil {
			return nil, "", httperr.NewBadRequest("invalid-next-value", "Invalid next parameter.")
		}
		nextTime := time.Unix(nextInt, 0)
		query = query.Where("created_at <= ?", nextTime)
	}
	query = query.Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit + 1)
	}

	rows, err := query.Rows()
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
