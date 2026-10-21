package main

import (
	"context"
	"log"
	"net/http"
	_ "time/tzdata" // zonas horarias dentro del binario (imágenes mínimas no las traen)

	"miestudio/backend-go/internal/api"
	"miestudio/backend-go/internal/config"
	"miestudio/backend-go/internal/db"
	"miestudio/backend-go/internal/mail"
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

	a := &api.API{DB: pool, Secret: cfg.JWTSecret, Limiter: ratelimit.New(), StaticDir: cfg.StaticDir, Production: cfg.Production, EncKey: cfg.EncKey, OAuthState: cfg.OAuthState, MPBase: cfg.MPBase,
		MPClientID: cfg.MPClientID, MPClientSecret: cfg.MPClientSecret, MPRedirect: cfg.MPRedirect, MPAuthBase: cfg.MPAuthBase}
	smtp := &mail.SMTP{Host: cfg.SMTPHost, Port: cfg.SMTPPort, User: cfg.SMTPUser, Pass: cfg.SMTPPass, From: cfg.MailFrom, Secure: cfg.SMTPSecure}
	a.AppURL = cfg.AppURL
	if smtp.Enabled() {
		a.Mailer = smtp
	} else {
		log.Printf("correo: SMTP no configurado; no se enviarán confirmaciones ni recordatorios")
	}
	go a.RunNotifications(ctx)
	if err := a.BootstrapPlatformAdmin(ctx, cfg.PlatformAdminName, cfg.PlatformAdminEmail, cfg.PlatformAdminPassword); err != nil {
		log.Fatal(err)
	}
	log.Printf("go api on :%s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, a.Routes()))
}
