package db

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type OpenOptions struct {
	LogLevel            logger.LogLevel
	MaxOpenConns        int
	MaxIdleConns        int
	ConnMaxLifetimemins int
}

var DefaultOpenOptions = OpenOptions{
	LogLevel:            logger.Silent,
	MaxOpenConns:        25,
	MaxIdleConns:        5,
	ConnMaxLifetimemins: 30,
}

func NormalizeDriver(value string) (Driver, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(DriverSQLite), "sqlite", "":
		return DriverSQLite, nil
	case string(DriverMariaDB), "mysql":
		return DriverMariaDB, nil
	case string(DriverPostgreSQL), "postgres":
		return DriverPostgreSQL, nil
	default:
		return "", fmt.Errorf("unsupported db driver: %s", value)
	}
}

func Open(driver string, dsn string) (*gorm.DB, error) {
	return OpenWithOptions(driver, dsn, DefaultOpenOptions)
}

func OpenWithOptions(driver string, dsn string, opts OpenOptions) (*gorm.DB, error) {
	drv, err := NormalizeDriver(driver)
	if err != nil {
		return nil, err
	}

	var dialector gorm.Dialector
	switch drv {
	case DriverMariaDB:
		dialector = mysql.Open(dsn)
	case DriverPostgreSQL:
		dialector = postgres.Open(dsn)
	default:
		dialector = sqlite.Open(dsn)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		TranslateError: true,
		Logger:         logger.Default.LogMode(opts.LogLevel),
	})
	if err != nil {
		return nil, err
	}

	if drv == DriverSQLite {
		if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
			return nil, err
		}
		if err := db.Exec("PRAGMA journal_mode = WAL").Error; err != nil {
			return nil, err
		}
	}

	if err := configurePool(db, drv, opts); err != nil {
		return nil, err
	}

	return db, nil
}

func configurePool(db *gorm.DB, drv Driver, opts OpenOptions) error {
	if drv == DriverSQLite {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	if opts.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(opts.MaxOpenConns)
	}
	if opts.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(opts.MaxIdleConns)
	}
	if opts.ConnMaxLifetimemins > 0 {
		sqlDB.SetConnMaxLifetime(time.Duration(opts.ConnMaxLifetimemins) * time.Minute)
	}
	return nil
}

func Ping(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

func Close(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func SqlDB(db *gorm.DB) (*sql.DB, error) {
	return db.DB()
}

var validDBName = regexp.MustCompile(`^[a-zA-Z0-9_\-\.]+$`)

func HardReset(driver string, dsn string) error {
	drv, err := NormalizeDriver(driver)
	if err != nil {
		return err
	}

	switch drv {
	case DriverMariaDB:
		cfg, err := mysqlDriver.ParseDSN(dsn)
		if err != nil {
			return err
		}
		dbName := cfg.DBName
		if dbName == "" {
			return errors.New("no database selected")
		}
		if !validDBName.MatchString(dbName) {
			return fmt.Errorf("invalid database name: %q", dbName)
		}
		cfg.DBName = ""
		root, err := Open(string(DriverMariaDB), cfg.FormatDSN())
		if err != nil {
			return err
		}
		defer Close(root)
		if err := root.Exec("DROP DATABASE IF EXISTS `" + dbName + "`").Error; err != nil {
			return err
		}
		if err := root.Exec("CREATE DATABASE `" + dbName + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci").Error; err != nil {
			return err
		}
		return nil
	case DriverPostgreSQL:
		cfg, err := pgx.ParseConfig(dsn)
		if err != nil {
			return err
		}
		dbName := cfg.Database
		if dbName == "" {
			return errors.New("no database selected")
		}
		if !validDBName.MatchString(dbName) {
			return fmt.Errorf("invalid database name: %q", dbName)
		}
		cfg.Database = "postgres"
		root, err := Open(string(DriverPostgreSQL), cfg.ConnString())
		if err != nil {
			return err
		}
		defer Close(root)
		if err := root.Exec("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = ? AND pid <> pg_backend_pid()", dbName).Error; err != nil {
			return err
		}
		if err := root.Exec(`DROP DATABASE IF EXISTS "` + dbName + `"`).Error; err != nil {
			return err
		}
		if err := root.Exec(`CREATE DATABASE "` + dbName + `"`).Error; err != nil {
			return err
		}
		return nil
	default:
		path := sqlitePath(dsn)
		if path == "" || path == ":memory:" {
			return nil
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
}

func sqlitePath(dsn string) string {
	s := strings.TrimSpace(dsn)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "file:") {
		u, err := url.Parse(s)
		if err != nil {
			return s
		}
		if u.Opaque != "" {
			return u.Opaque
		}
		return strings.TrimPrefix(u.Path, "//")
	}
	base := strings.SplitN(s, "?", 2)[0]
	if base == "" {
		return ""
	}
	if filepath.IsAbs(base) || base == ":memory:" {
		return base
	}
	abs, err := filepath.Abs(base)
	if err != nil {
		return base
	}
	return abs
}
