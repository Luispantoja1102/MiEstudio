package main

import (
	"context"
	"log"
	"net/http"

	"miestudio/backend-go/internal/api"
	"miestudio/backend-go/internal/config"
	"miestudio/backend-go/internal/db"
	"miestudio/backend-go/internal/ratelimit"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatal(err)
	}

	a := &api.API{DB: pool, Secret: cfg.JWTSecret, Limiter: ratelimit.New(), StaticDir: cfg.StaticDir}
	log.Printf("go api on :%s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, a.Routes()))
}
