package db

import (
	"database/sql"
	"time"

	pgxv5 "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/sekolahpintar/ews-worker/internal/config"
)

func New(cfg *config.Config) (*sqlx.DB, error) {
	connConfig, err := pgxv5.ParseConfig(cfg.DB.DSN())
	if err != nil {
		return nil, err
	}
	// Use simple protocol so pgbouncer transaction-pooling mode does not
	// break on cached prepared statements.
	connConfig.DefaultQueryExecMode = pgxv5.QueryExecModeSimpleProtocol
	stdDB := stdlib.OpenDB(*connConfig)
	database := sqlx.NewDb(stdDB, "pgx")

	database.SetMaxOpenConns(100)
	database.SetMaxIdleConns(20)
	database.SetConnMaxLifetime(5 * time.Minute)
	database.SetConnMaxIdleTime(2 * time.Minute)

	if err := database.Ping(); err != nil {
		return nil, err
	}

	return database, nil
}

// Tx wraps a function in a DB transaction.
func Tx(db *sqlx.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
