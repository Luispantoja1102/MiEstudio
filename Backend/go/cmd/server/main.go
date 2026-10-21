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
	if err := api.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		log.Fatal(err)
	}
	// primera ejecución: si la base no existe se crea sola (como en MiConsultorio)
	if created, err := db.EnsureDatabase(ctx, cfg.DatabaseURL); err != nil {
		log.Fatal(err)
	} else if created {
		log.Printf("base de datos creada")
	}
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
	a.PlatformMPToken, a.PlatformWebhookSecret, a.PlatformNoticeTo = cfg.PlatformMPToken, cfg.PlatformWebhookSecret, cfg.PlatformNoticeTo
	a.Operator = api.Operator{Name: cfg.LegalName, Address: cfg.LegalAddress, Email: cfg.LegalEmail}
	a.AppURL, a.PublicURL, a.MPWebhookSecret = cfg.AppURL, cfg.PublicURL, cfg.MPWebhookSecret
	if smtp.Enabled() {
		a.Mailer = smtp
	} else {
		log.Printf("correo: SMTP no configurado; no se enviarán confirmaciones ni recordatorios")
	}
	go a.RunNotifications(ctx)
	if err := a.BootstrapPlatformAdmin(ctx, cfg.PlatformAdminName, cfg.PlatformAdminEmail, cfg.PlatformAdminPassword); err != nil {
		log.Fatal(err)
	}
	if err := a.BootstrapDemoBusiness(ctx, cfg.DemoBusiness, cfg.DemoKind, cfg.DemoName, cfg.DemoEmail, cfg.DemoPassword); err != nil {
		log.Printf("negocio de ejemplo: %v", err) // no es grave: la API arranca igual
	}
	log.Printf("go api on :%s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, a.Routes()))
}
