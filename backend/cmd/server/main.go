package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"example.com/uptime-app/backend/internal/auth"
	"example.com/uptime-app/backend/internal/config"
	"example.com/uptime-app/backend/internal/server"
	"example.com/uptime-app/backend/internal/storage"
)

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	db, err := storage.Open(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err = sqlDB.PingContext(ctx); err != nil {
		return err
	}
	if err = storage.Migrate(ctx, db); err != nil {
		return err
	}
	service := &auth.Service{DB: db, Secret: []byte(cfg.JWTSecret), AccessTTL: cfg.AccessTTL, RefreshTTL: cfg.RefreshTTL}
	mux := http.NewServeMux()
	controller := &auth.Controller{Service: service, CookieSecure: cfg.CookieSecure, AvatarDir: cfg.AvatarDir, UploadDir: cfg.UploadDir}
	controller.RegisterRoutes(mux)
	server := &http.Server{Addr: cfg.ListenAddr, Handler: server.Middleware(mux, cfg.AllowedOrigin), ReadHeaderTimeout: 5 * time.Second}
	stop, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()
	go func() {
		<-stop.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()
	log.Printf("listening on %s", cfg.ListenAddr)
	err = server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
