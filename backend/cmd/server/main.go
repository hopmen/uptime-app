package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/config"
	"uptime-app/backend/internal/monitor"
	"uptime-app/backend/internal/server"
	"uptime-app/backend/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	db, err := store.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := store.Close(db); err != nil {
			log.Printf("close database: %v", err)
		}
	}()

	repository := store.NewRepository(db)
	authService := auth.NewService(repository, cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	monitorService := monitor.NewService(repository)
	httpServer := &http.Server{Addr: cfg.Address, Handler: server.New(cfg, authService, monitorService), ReadHeaderTimeout: 5 * time.Second}

	go func() {
		log.Printf("listening on %s", cfg.Address)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("shutdown server: %v", err)
	}
}
