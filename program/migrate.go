package program

import (
	"errors"
	"fmt"
	"log"

	dbx "github.com/discuitnet/discuit/internal/db"
	"gorm.io/gorm"
)

var ErrMigrationsTableNotFound = errors.New("migrations table not found")

type migrationsLogger struct {
	verbose bool
}

func (ml *migrationsLogger) Printf(format string, v ...any) {
	log.Printf(format, v...)
}

func (ml *migrationsLogger) Verbose() bool {
	return ml.verbose
}

func (pg *Program) Migrate(logMigrations bool, steps int) error {
	fmt.Println("Running migrations")
	if pg.db == nil {
		if _, err := pg.OpenDatabase(); err != nil {
			return err
		}
	}
	return dbx.Migrate(pg.ctx, pg.db, steps)
}

type MigrationsStatus struct {
	Version int `json:"version"`
	Dirty   int `json:"dirty"`
}

func (pg *Program) MigrationsStatus() (MigrationsStatus, error) {
	if pg.db == nil {
		if _, err := pg.OpenDatabase(); err != nil {
			return MigrationsStatus{}, err
		}
	}
	if !pg.db.Migrator().HasTable(&dbx.MigrationRecord{}) {
		return MigrationsStatus{}, ErrMigrationsTableNotFound
	}

	status, err := dbx.MigrationStatus(pg.ctx, pg.db)
	if err != nil {
		return MigrationsStatus{}, err
	}
	if status.Version == 0 && !status.Dirty {
		return MigrationsStatus{}, gorm.ErrRecordNotFound
	}

	dirty := 0
	if status.Dirty {
		dirty = 1
	}

	return MigrationsStatus{
		Version: status.Version,
		Dirty:   dirty,
	}, nil
}
