package database

import (
	"database/sql"
	"fmt"
)

type Config struct {
	Driver string // "postgres", "mysql", "mssql"
	DSN    string
}

func New(cfg Config) (*sql.DB, error) {
	switch cfg.Driver {
	case "postgres", "mysql", "mssql":
		db, err := sql.Open(driverName(cfg.Driver), cfg.DSN)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", cfg.Driver, err)
		}
		if err := db.Ping(); err != nil {
			return nil, fmt.Errorf("ping %s: %w", cfg.Driver, err)
		}
		return db, nil
	default:
		return nil, fmt.Errorf("unsupported driver: %s", cfg.Driver)
	}
}

func driverName(driver string) string {
	switch driver {
	case "postgres":
		return "pgx"
	case "mysql":
		return "mysql"
	case "mssql":
		return "sqlserver"
	default:
		return driver
	}
}
