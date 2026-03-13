package db

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Migration struct {
	Version int
	Name    string
	Up      func(context.Context, *gorm.DB) error
	Down    func(context.Context, *gorm.DB) error
}

type Status struct {
	Version int
	Dirty   bool
}

var migrations []Migration

func init() {
	registerMigration(Migration{
		Version: 1,
		Name:    "baseline",
		Up:      migrateBaseline,
		Down:    rollbackBaseline,
	})
}

func registerMigration(migration Migration) {
	migrations = append(migrations, migration)
	slices.SortFunc(migrations, func(a, b Migration) int {
		switch {
		case a.Version < b.Version:
			return -1
		case a.Version > b.Version:
			return 1
		default:
			return 0
		}
	})
}

func RegisteredMigrations() []Migration {
	ordered := slices.Clone(migrations)
	slices.SortFunc(ordered, func(a, b Migration) int {
		return a.Version - b.Version
	})
	return ordered
}

func Migrate(ctx context.Context, db *gorm.DB, steps int) error {
	if err := ensureMigrationTable(ctx, db); err != nil {
		return err
	}

	status, err := MigrationStatus(ctx, db)
	if err != nil {
		return err
	}
	if status.Dirty {
		return errors.New("schema_migrations is dirty")
	}

	applied, err := appliedVersions(ctx, db)
	if err != nil {
		return err
	}

	if steps < 0 {
		return rollback(ctx, db, applied, -steps)
	}
	return migrateUp(ctx, db, applied, steps)
}

func MigrationStatus(ctx context.Context, db *gorm.DB) (Status, error) {
	if !db.Migrator().HasTable(&MigrationRecord{}) {
		return Status{}, nil
	}

	var record MigrationRecord
	tx := db.WithContext(ctx).Order("version desc").Limit(1).Find(&record)
	if tx.RowsAffected == 0 {
		return Status{}, nil
	}
	if tx.Error != nil {
		return Status{}, tx.Error
	}

	return Status{
		Version: record.Version,
		Dirty:   record.Dirty,
	}, nil
}

func ensureMigrationTable(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).AutoMigrate(&MigrationRecord{})
}

func appliedVersions(ctx context.Context, db *gorm.DB) (map[int]MigrationRecord, error) {
	var rows []MigrationRecord
	if err := db.WithContext(ctx).Order("version").Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make(map[int]MigrationRecord, len(rows))
	for _, row := range rows {
		result[row.Version] = row
	}
	return result, nil
}

func migrateUp(ctx context.Context, db *gorm.DB, applied map[int]MigrationRecord, steps int) error {
	remaining := 0
	if steps > 0 {
		remaining = steps
	}

	for _, migration := range RegisteredMigrations() {
		if _, ok := applied[migration.Version]; ok {
			continue
		}
		if remaining == 0 && steps > 0 {
			break
		}
		if err := applyMigration(ctx, db, migration); err != nil {
			return err
		}
		if steps > 0 {
			remaining--
		}
	}

	return nil
}

func rollback(ctx context.Context, db *gorm.DB, applied map[int]MigrationRecord, steps int) error {
	if steps == 0 {
		return nil
	}

	stack := make([]Migration, 0, len(migrations))
	for _, migration := range RegisteredMigrations() {
		if _, ok := applied[migration.Version]; ok {
			stack = append(stack, migration)
		}
	}

	for i := len(stack) - 1; i >= 0 && steps > 0; i-- {
		if err := rollbackMigration(ctx, db, stack[i]); err != nil {
			return err
		}
		steps--
	}

	return nil
}

func applyMigration(ctx context.Context, db *gorm.DB, migration Migration) error {
	record := MigrationRecord{
		Version:   migration.Version,
		Name:      migration.Name,
		Dirty:     true,
		AppliedAt: time.Now().UTC(),
	}
	if err := db.WithContext(ctx).Create(&record).Error; err != nil {
		return err
	}

	if err := migration.Up(ctx, db); err != nil {
		return err
	}

	return db.WithContext(ctx).
		Model(&MigrationRecord{}).
		Where("version = ?", migration.Version).
		Updates(map[string]any{"dirty": false, "name": migration.Name, "applied_at": time.Now().UTC()}).
		Error
}

func rollbackMigration(ctx context.Context, db *gorm.DB, migration Migration) error {
	if migration.Down == nil {
		return fmt.Errorf("migration %d has no down function", migration.Version)
	}

	if err := db.WithContext(ctx).
		Model(&MigrationRecord{}).
		Where("version = ?", migration.Version).
		Update("dirty", true).
		Error; err != nil {
		return err
	}

	if err := migration.Down(ctx, db); err != nil {
		return err
	}

	return db.WithContext(ctx).Where("version = ?", migration.Version).Delete(&MigrationRecord{}).Error
}

func migrateBaseline(ctx context.Context, db *gorm.DB) error {
	if err := db.WithContext(ctx).AutoMigrate(schemaModels()...); err != nil {
		return err
	}
	if err := ensurePostWindowIndexes(ctx, db); err != nil {
		return err
	}
	if err := ensureUserIndex(ctx, db); err != nil {
		return err
	}
	if err := ensureListSequence(ctx, db); err != nil {
		return err
	}
	return seedBaseline(ctx, db)
}

func ensurePostWindowIndexes(ctx context.Context, db *gorm.DB) error {
	type indexDef struct {
		sql []string
	}

	defs := []indexDef{
		{
			sql: []string{
				"CREATE UNIQUE INDEX IF NOT EXISTS posts_today_post_id ON posts_today(post_id)",
				"CREATE INDEX IF NOT EXISTS posts_today_created_at ON posts_today(created_at)",
				"CREATE INDEX IF NOT EXISTS posts_today_community_created_at ON posts_today(community_id, created_at)",
				"CREATE INDEX IF NOT EXISTS posts_today_points_post_id ON posts_today(points, post_id)",
				"CREATE INDEX IF NOT EXISTS posts_today_community_points ON posts_today(community_id, points)",
			},
		},
		{
			sql: []string{
				"CREATE UNIQUE INDEX IF NOT EXISTS posts_week_post_id ON posts_week(post_id)",
				"CREATE INDEX IF NOT EXISTS posts_week_created_at ON posts_week(created_at)",
				"CREATE INDEX IF NOT EXISTS posts_week_community_created_at ON posts_week(community_id, created_at)",
				"CREATE INDEX IF NOT EXISTS posts_week_points_post_id ON posts_week(points, post_id)",
				"CREATE INDEX IF NOT EXISTS posts_week_community_points ON posts_week(community_id, points)",
			},
		},
		{
			sql: []string{
				"CREATE UNIQUE INDEX IF NOT EXISTS posts_month_post_id ON posts_month(post_id)",
				"CREATE INDEX IF NOT EXISTS posts_month_created_at ON posts_month(created_at)",
				"CREATE INDEX IF NOT EXISTS posts_month_community_created_at ON posts_month(community_id, created_at)",
				"CREATE INDEX IF NOT EXISTS posts_month_points_post_id ON posts_month(points, post_id)",
				"CREATE INDEX IF NOT EXISTS posts_month_community_points ON posts_month(community_id, points)",
			},
		},
		{
			sql: []string{
				"CREATE UNIQUE INDEX IF NOT EXISTS posts_year_post_id ON posts_year(post_id)",
				"CREATE INDEX IF NOT EXISTS posts_year_created_at ON posts_year(created_at)",
				"CREATE INDEX IF NOT EXISTS posts_year_community_created_at ON posts_year(community_id, created_at)",
				"CREATE INDEX IF NOT EXISTS posts_year_points_post_id ON posts_year(points, post_id)",
				"CREATE INDEX IF NOT EXISTS posts_year_community_points ON posts_year(community_id, points)",
			},
		},
	}

	for _, def := range defs {
		for _, statement := range def.sql {
			if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
				return err
			}
		}
	}

	return nil
}

func rollbackBaseline(ctx context.Context, db *gorm.DB) error {
	models := schemaModels()
	for i := len(models) - 1; i >= 0; i-- {
		if err := db.WithContext(ctx).Migrator().DropTable(models[i]); err != nil {
			return err
		}
	}
	return nil
}

func schemaModels() []any {
	models := AllModels()
	return models[1:]
}

func ensureUserIndex(ctx context.Context, db *gorm.DB) error {
	switch db.Dialector.Name() {
	case "mysql":
		return db.WithContext(ctx).Exec("ALTER TABLE users MODIFY COLUMN user_index BIGINT NOT NULL AUTO_INCREMENT").Error
	case "postgres":
		statements := []string{
			"CREATE SEQUENCE IF NOT EXISTS users_user_index_seq",
			"ALTER TABLE users ALTER COLUMN user_index TYPE BIGINT",
			"ALTER TABLE users ALTER COLUMN user_index SET DEFAULT nextval('users_user_index_seq')",
			"ALTER SEQUENCE users_user_index_seq OWNED BY users.user_index",
		}
		for _, statement := range statements {
			if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
				return err
			}
		}
		return nil
	case "sqlite":
		statements := []string{
			"CREATE TRIGGER IF NOT EXISTS users_user_index_insert AFTER INSERT ON users FOR EACH ROW WHEN NEW.user_index IS NULL BEGIN UPDATE users SET user_index = (SELECT COALESCE(MAX(user_index), 0) + 1 FROM users WHERE id <> NEW.id) WHERE rowid = NEW.rowid; END",
		}
		for _, statement := range statements {
			if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported dialector %q", db.Dialector.Name())
	}
}

func ensureListSequence(ctx context.Context, db *gorm.DB) error {
	switch db.Dialector.Name() {
	case "mysql":
		return db.WithContext(ctx).Exec("ALTER TABLE lists AUTO_INCREMENT = 100000").Error
	case "postgres":
		return db.WithContext(ctx).Exec("ALTER SEQUENCE IF EXISTS lists_id_seq RESTART WITH 100000").Error
	case "sqlite":
		if err := db.WithContext(ctx).Exec("UPDATE sqlite_sequence SET seq = 99999 WHERE name = 'lists'").Error; err != nil && !strings.Contains(err.Error(), "no such table: sqlite_sequence") {
			return err
		}
		err := db.WithContext(ctx).Exec("INSERT OR IGNORE INTO sqlite_sequence(name, seq) VALUES ('lists', 99999)").Error
		if err != nil && !strings.Contains(err.Error(), "no such table: sqlite_sequence") {
			return err
		}
		return nil
	default:
		return fmt.Errorf("unsupported dialector %q", db.Dialector.Name())
	}
}

func seedBaseline(ctx context.Context, db *gorm.DB) error {
	reasons := []ReportReason{
		{ID: 1, Title: "Breaks community rules"},
		{ID: 2, Title: "Copyright violation"},
		{ID: 3, Title: "Spam"},
		{ID: 4, Title: "Pornography"},
	}

	return db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&reasons).
		Error
}
