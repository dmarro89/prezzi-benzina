package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	appapi "github.com/dmarro89/prezzi-benzina/backend/internal/api"
	"github.com/dmarro89/prezzi-benzina/backend/internal/mimit"
	"github.com/dmarro89/prezzi-benzina/backend/internal/store"
)

func main() {
	addr := env("ADDR", ":8080")
	refreshEvery := durationEnv("REFRESH_INTERVAL", 6*time.Hour)
	dataStore := store.New()
	client := mimit.NewClient()
	if v := os.Getenv("MIMIT_STATIONS_URL"); v != "" { client.StationsURL = v }
	if v := os.Getenv("MIMIT_PRICES_URL"); v != "" { client.PricesURL = v }

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if err := refresh(ctx, client, dataStore); err != nil { log.Printf("initial MIMIT load failed: %v", err) }
	go func() {
		ticker := time.NewTicker(refreshEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done(): return
			case <-ticker.C:
				if err := refresh(ctx, client, dataStore); err != nil { log.Printf("MIMIT refresh failed: %v", err) }
			}
		}
	}()

	server := &http.Server{Addr: addr, Handler: appapi.Handler{Store: dataStore}.Routes(), ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		log.Printf("API listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed { log.Fatalf("serve: %v", err) }
	}()
	<-ctx.Done()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = server.Shutdown(shutdownCtx)
}

func refresh(ctx context.Context, client *mimit.Client, s *store.Store) error {
	loadCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	data, err := client.Load(loadCtx)
	if err != nil { return err }
	s.Replace(data)
	stations, prices, extracted, _ := s.Stats()
	log.Printf("MIMIT dataset loaded: stations=%d prices=%d extracted=%s", stations, prices, extracted.Format("2006-01-02"))
	return nil
}

func env(name, fallback string) string { if v := os.Getenv(name); v != "" { return v }; return fallback }
func durationEnv(name string, fallback time.Duration) time.Duration { if v := os.Getenv(name); v != "" { if d, err := time.ParseDuration(v); err == nil { return d } }; return fallback }
