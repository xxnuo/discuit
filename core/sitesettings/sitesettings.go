package sitesettings

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	idb "github.com/discuitnet/discuit/internal/db"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var cache = &ssCache{}

// SiteSettings are distinct from config.Config. SiteSettings are those settings
// that can be changed on the fly (while everything is running) from the admin
// dashboard.
type SiteSettings struct {
	SignupsDisabled bool `json:"signupsDisabled"`
	TorBlocked      bool `json:"torBlocked"`

	// note: ssCache.store() and ssCache.get() uses shallow-copy on this struct.
	// So those lines of code need updating if pointer fields are added to this
	// struct.
}

// Save persists s to the database.
func (s *SiteSettings) Save(ctx context.Context, db *gorm.DB) error {
	defer cache.bust()

	bytes, err := json.Marshal(s)
	if err != nil {
		return err
	}
	value := string(bytes)

	return db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.Assignments(map[string]any{"value": string(bytes)}),
		}).
		Create(&idb.ApplicationData{
			Key:   "site_settings",
			Value: &value,
		}).
		Error
}

type ssCache struct {
	mu       sync.RWMutex
	settings *SiteSettings
}

func (c *ssCache) get() *SiteSettings {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.settings != nil {
		cp := &SiteSettings{}
		*cp = *c.settings // shallow copy
		return cp
	}
	return nil
}

func (c *ssCache) store(s *SiteSettings) {
	c.mu.Lock()
	cp := &SiteSettings{}
	*cp = *s // shallow copy
	c.settings = cp
	c.mu.Unlock()
}

func (c *ssCache) bust() {
	c.mu.Lock()
	c.settings = nil
	c.mu.Unlock()
}

// GetSiteSettings retrieves the site settings (of the admin dashboard) from the
// database. If the data is not found in the database (as the case may be if
// they were never altered), then the default settings object is returned.
func GetSiteSettings(ctx context.Context, db *gorm.DB) (*SiteSettings, error) {
	if settings := cache.get(); settings != nil {
		return settings, nil
	}

	var record idb.ApplicationData
	err := db.WithContext(ctx).
		Select("value").
		Where("key = ?", "site_settings").
		Take(&record).
		Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("error reading site_settings from db: %w", err)
	}

	settings := &SiteSettings{}
	jsonText := ""
	if record.Value != nil {
		jsonText = *record.Value
	}
	if jsonText == "" {
		// Do nothhing. No row was found in the application_data table. Return
		// the deafult SiteSettings object.
	} else {
		if err = json.Unmarshal([]byte(jsonText), settings); err != nil {
			return nil, err
		}
	}

	cache.store(settings)
	return settings, nil
}
