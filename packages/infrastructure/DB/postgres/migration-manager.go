package postgres

import (
	"fmt"
	"sentinel/packages/common/logger"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/stdlib"
)

var migrationLogger = logger.NewSource("MIGRATION", logger.Default)

type Migrate struct {
	//
}

func (_ Migrate) init() (*migrate.Migrate, error) {
	migrationLogger.Trace("Initializing DB driver for migrations...", nil)

	db := stdlib.OpenDB(*driver.PrimaryConfig.ConnConfig)

	dbDriver, e := postgres.WithInstance(db, &postgres.Config{})
	if e != nil {
		return nil, e
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://../../migrations",
		"postgres",
		dbDriver,
	)
	if err !=  nil {
		return nil, err
	}

	migrationLogger.Trace("Initializing DB driver for migrations: OK", nil)

	return m, nil
}

func (m Migrate) step(n int) error {
	version := strconv.FormatInt(int64(n), 10)

	migrator, err := m.init()
	if err != nil {
		migrationLogger.Fatal(
			"Failed to apply migrations",
			err.Error(),
			nil,
		)
		return err
	}

	ver, dirty, err := migrator.Version()
	if err != nil {
		return err
	}

	if dirty {
		migrationLogger.Info(
			fmt.Sprintf("Detected dirty database at version %d. Forcing clean state...", ver),
			nil,
		)
		if err := migrator.Force(int(ver)); err != nil {
			migrationLogger.Panic(
				fmt.Sprintf("Failed to force clean state at version %d", ver),
				err.Error(),
				nil,
			)
			return err
		}
		migrationLogger.Info(
			fmt.Sprintf("Forced clean state at version %d. ", ver),
			nil,
		)
	}

	migrationLogger.Info("Applying migrations... (version change: "+version+")", nil)

	err = migrator.Steps(n)
	if err != nil && err != migrate.ErrNoChange {
		migrationLogger.Error("Failed to apply migrations", err.Error(), nil)
		return err
	}

	migrationLogger.Info("Migrations applied (version change: "+version+")", nil)

	return nil
}

func (m Migrate) Up() error {
	return m.step(1)
}

func (m Migrate) Down() error {
	return m.step(-1)
}

func (m Migrate) Steps(n int) error {
	return m.step(n)
}

