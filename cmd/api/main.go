package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Mavichy/TestTaskEffectiveMobile/internal/config"
	"github.com/Mavichy/TestTaskEffectiveMobile/internal/httpapi"
	"github.com/Mavichy/TestTaskEffectiveMobile/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	db, err := storage.NewDB(context.Background(), cfg.DBDSN)
	if err != nil {
		logger.Error("db connect", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := storage.ApplyMigrations(context.Background(), db, "migrations"); err != nil {
		logger.Error("migrations", "err", err)
		os.Exit(1)
	}

	repo := storage.NewSubscriptionsRepo(db)

	srv := httpapi.NewServer(cfg.HTTPAddr, logger, repo)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("server start", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil {
			logger.Error("server", "err", err)
			stop()
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	logger.Info("server stopped")
}
