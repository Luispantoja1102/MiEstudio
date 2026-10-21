package db

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestEnsureDatabase(t *testing.T) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL no definida")
	}
	ctx := context.Background()
	u, _ := url.Parse(base)
	name := fmt.Sprintf("mie_ensure_%d", time.Now().UnixNano())
	u.Path = "/" + name
	t.Cleanup(func() {
		u.Path = "/postgres"
		if c, err := pgx.Connect(ctx, u.String()); err == nil {
			c.Exec(ctx, `DROP DATABASE IF EXISTS "`+name+`"`)
			c.Close(ctx)
		}
	})
	u.Path = "/" + name
	created, err := EnsureDatabase(ctx, u.String())
	if err != nil || !created {
		t.Fatalf("debía crearse: %v %v", created, err)
	}
	if created, err = EnsureDatabase(ctx, u.String()); err != nil || created {
		t.Fatalf("segunda vez no crea: %v %v", created, err)
	}
	// servidor inalcanzable: el error original, sin intentar crear nada
	bad := *u
	bad.Host = "127.0.0.1:1"
	if _, err := EnsureDatabase(ctx, bad.String()); err == nil {
		t.Fatal("un servidor caído debe fallar")
	}
	// nombre no seguro: no se interpola en SQL
	weird := *u
	weird.Path = "/no-existe-con-guion"
	if created, err := EnsureDatabase(ctx, weird.String()); err == nil || created {
		t.Fatalf("un nombre con guion no se crea solo: %v %v", created, err)
	}
}

func TestMigrateConcurrentStartsAreSafe(t *testing.T) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL no definida")
	}
	ctx := context.Background()
	u, _ := url.Parse(base)
	name := fmt.Sprintf("mie_migrate_%d", time.Now().UnixNano())
	u.Path = "/" + name
	t.Cleanup(func() {
		u.Path = "/postgres"
		if c, err := pgx.Connect(ctx, u.String()); err == nil {
			c.Exec(ctx, `DROP DATABASE IF EXISTS "`+name+`" WITH (FORCE)`)
			c.Close(ctx)
		}
	})
	if _, err := EnsureDatabase(ctx, u.String()); err != nil {
		t.Fatal(err)
	}
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		go func() {
			pool, err := Connect(ctx, u.String())
			if err != nil {
				errs <- err
				return
			}
			defer pool.Close()
			errs <- Migrate(ctx, pool)
		}()
	}
	for i := 0; i < 4; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("arranque simultáneo: %v", err)
		}
	}
}
