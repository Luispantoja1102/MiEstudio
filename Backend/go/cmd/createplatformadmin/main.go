// Crea o restablece un administrador de la plataforma.
//
//	go run ./cmd/createplatformadmin -email tu@correo.com -password '...' [-name 'Tu nombre']
//
// Sin banderas usa PLATFORM_ADMIN_EMAIL, PLATFORM_ADMIN_PASSWORD y PLATFORM_ADMIN_NAME del entorno.
// Si el correo ya es de una persona de plataforma, le restablece la contraseña.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"miestudio/backend-go/internal/api"
	"miestudio/backend-go/internal/config"
	"miestudio/backend-go/internal/db"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	email := flag.String("email", env("PLATFORM_ADMIN_EMAIL", ""), "correo del administrador")
	password := flag.String("password", env("PLATFORM_ADMIN_PASSWORD", ""), "contraseña (8 a 72 caracteres)")
	name := flag.String("name", env("PLATFORM_ADMIN_NAME", "Administrador"), "nombre")
	flag.Parse()

	if *email == "" || *password == "" {
		log.Fatal("indica -email y -password (o PLATFORM_ADMIN_EMAIL y PLATFORM_ADMIN_PASSWORD)")
	}
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

	a := &api.API{DB: pool, Secret: cfg.JWTSecret, Production: cfg.Production}
	created, err := a.UpsertPlatformAdmin(ctx, *name, *email, *password)
	if err != nil {
		log.Fatal(err)
	}
	if created {
		fmt.Printf("Administrador de plataforma creado: %s\n", *email)
	} else {
		fmt.Printf("Contraseña restablecida para: %s\n", *email)
	}
	fmt.Println("Entra en /login: te llevará a /plataforma.")
}
