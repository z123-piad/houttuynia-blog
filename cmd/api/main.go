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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/z123-piad/houttuynia-blog/db"
	"github.com/z123-piad/houttuynia-blog/internal/config"
	"github.com/z123-piad/houttuynia-blog/internal/httpapi"
	"github.com/z123-piad/houttuynia-blog/internal/identity"
	"github.com/z123-piad/houttuynia-blog/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if cfg.WebDir != "" {
		info, err := os.Stat(cfg.WebDir)
		if err != nil || !info.IsDir() {
			log.Fatalf("WEB_DIR %s is not a directory", cfg.WebDir)
		}
	}
	ctx := context.Background()
	if err := db.Migrate(cfg.DatabaseURL); err != nil {
		log.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := identity.NewService(pool).SeedSuperAdmin(ctx, cfg.SeedAdminEmail, cfg.SeedAdminPassword); err != nil {
		log.Fatal(err)
	}
	if cfg.SeedAdminEmail != "" {
		log.Printf("super admin seed checked for %s", cfg.SeedAdminEmail)
	}
	driver, err := storage.NewDriver(cfg)
	if err != nil {
		log.Fatal(err)
	}
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.New(cfg, pool, driver),
		ReadHeaderTimeout: 10 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		log.Printf("listening on %s", cfg.HTTPAddr)
		errCh <- srv.ListenAndServe()
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	case <-stop:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Fatal(err)
		}
	}
}
