package db

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"regexp"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var dbName = regexp.MustCompile(`^[A-Za-z0-9_]{1,63}$`)

// EnsureDatabase crea la base de datos de la URL si el servidor responde pero la base no existe
// (SQLSTATE 3D000). Es una comodidad de desarrollo: se conecta a la base «postgres» con las mismas credenciales.
// Si la conexión falla por otra razón, o el usuario no puede crear bases, devuelve el error original.
func EnsureDatabase(ctx context.Context, url string) (created bool, err error) {
	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		return false, err
	}
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err == nil {
		conn.Close(ctx)
		return false, nil
	}
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != "3D000" {
		return false, err
	}
	name := cfg.Database
	if !dbName.MatchString(name) {
		return false, fmt.Errorf("el nombre de base de datos %q no se puede crear automáticamente: %w", name, err)
	}
	admin := cfg.Copy()
	admin.Database = "postgres"
	ac, aerr := pgx.ConnectConfig(ctx, admin)
	if aerr != nil {
		return false, fmt.Errorf("la base %q no existe y no se pudo crear (%v): %w", name, aerr, err)
	}
	defer ac.Close(ctx)
	if _, xerr := ac.Exec(ctx, `CREATE DATABASE "`+name+`"`); xerr != nil {
		return false, fmt.Errorf("la base %q no existe y no se pudo crear (%v): %w", name, xerr, err)
	}
	return true, nil
}

//go:embed migrations/*.sql
var migrations embed.FS

func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// Migrate aplica en orden las migraciones que aún no se han ejecutado.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		name TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		var done bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE name = $1)`, name).Scan(&done); err != nil {
			return err
		}
		if done {
			continue
		}
		sqlText, err := migrations.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(sqlText)); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("migración %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (name) VALUES ($1)`, name); err != nil {
			tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}
