package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/phamtuanha21092003/ecommerce-be/platform/database"
	"github.com/phamtuanha21092003/ecommerce-be/platform/logger"
	"github.com/phamtuanha21092003/ecommerce-be/platform/storage"
)

type (
	App struct {
		cfg    *ApiConfig
		logger *zap.Logger
		db     *database.SqlxDatabase
		s3     *storage.S3Storage

		server *http.Server

		repositories repositoryApi
		services     serviceApi
		controllers  controllerApi
	}
)

func NewApp() (*App, error) {
	if err := loadApiConfig(); err != nil {
		return nil, fmt.Errorf("load api config: %w", err)
	}

	cfg := GetApiConfig()
	if err := logger.SetUpLogger(cfg.App.Debug); err != nil {
		return nil, err
	}

	if cfg.App.Host == "" {
		cfg.App.Host = "0.0.0.0"
	}

	a := &App{
		cfg:    cfg,
		logger: logger.GetLogger(),
		server: &http.Server{
			Addr:              fmt.Sprintf("%s:%d", cfg.App.Host, cfg.App.Port),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
	}

	if err := a.connectInfra(); err != nil {
		return nil, fmt.Errorf("connect infra: %w", err)
	}

	a.server.Handler = newRouter(a.logger, a.db, &a.controllers)
	return a, nil
}

func (a *App) connectInfra() error {
	db, err := database.OpenSQLX("postgres", a.cfg.PostgresqlUri)
	if err != nil {
		return err
	}
	a.db = db

	s3, err := storage.NewS3Storage(a.cfg.S3Endpoint, a.cfg.S3AccessKeyID, a.cfg.S3SecretAccessKey, a.cfg.S3Bucket)
	if err != nil {
		return err
	}
	a.s3 = s3

	repositories := newRepository(db, a.logger)
	a.repositories = *repositories

	services := newService(&a.repositories, a.logger, a.s3, a.cfg.StaticUrl)
	a.services = *services

	controllers := newController(a.logger, &a.services)
	a.controllers = *controllers

	return nil
}

// Run serves HTTP until SIGINT or SIGTERM, then drains requests and closes the pool.
func (a *App) Run() error {
	errCh := make(chan error, 1)
	go func() {
		a.logger.Info("api listening", zap.String("addr", a.server.Addr))
		errCh <- a.server.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-stop:
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return a.shutdown(ctx)
	}
}

func (a *App) shutdown(ctx context.Context) error {
	if a.server != nil {
		if err := a.server.Shutdown(ctx); err != nil {
			return err
		}
	}
	if a.db != nil {
		if err := a.db.Close(); err != nil {
			return err
		}
	}
	_ = logger.Sync()
	return nil
}
