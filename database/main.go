package database

//go:generate sqlc generate

import (
	"context"
	"database/sql"
	"embed"

	"github.com/jackc/pgx/v4"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

func MigrateDB(db *sql.DB) error {
	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return err;
	}

	if err := goose.Up(db, "migrations"); err != nil {
		return err;
	}

	return nil
}

func ResetDB(db *sql.DB) error {
	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	if err := goose.Reset(db, "migrations"); err != nil {
		return err
	}
	return nil
}


type TxLike interface {
	DBTX
	Begin(ctx context.Context) (pgx.Tx, error)
	BeginFunc(ctx context.Context, f func(pgx.Tx) error) (err error)
}


