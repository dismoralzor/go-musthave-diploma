// Command gophermart запускает HTTP-сервер накопительной системы лояльности
// «Гофермарт».
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/dismoralzor/go-musthave-diploma/internal/accrual"
	"github.com/dismoralzor/go-musthave-diploma/internal/config"
	"github.com/dismoralzor/go-musthave-diploma/internal/handler"
	"github.com/dismoralzor/go-musthave-diploma/internal/logger"
	"github.com/dismoralzor/go-musthave-diploma/internal/storage"
	"github.com/dismoralzor/go-musthave-diploma/internal/worker"
)

// shutdownTimeout — время, отводимое серверу на завершение уже начатых
// запросов после получения сигнала остановки.
const shutdownTimeout = 10 * time.Second

// Параметры фонового воркера, опрашивающего систему начислений.
const (
	workerCount        = 5
	workerPollInterval = time.Second
)

func main() {
	if err := run(); err != nil {
		logger.Log.Error("service stopped", zap.Error(err))
		os.Exit(1)
	}
}

// run содержит основную логику запуска сервиса: разбор конфигурации,
// инициализацию логгера, подключение к базе данных, применение миграций,
// запуск HTTP-сервера и его корректное завершение по сигналу ОС.
//
// Логика вынесена из main в отдельную функцию, возвращающую error, чтобы её
// было проще тестировать и чтобы os.Exit вызывался только в одном месте —
// в main, после того как все defer в run успели отработать.
func run() error {
	cfg, err := config.Parse(os.Args[1:], os.Getenv)
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	if err := logger.Initialize("info"); err != nil {
		return fmt.Errorf("initialize logger: %w", err)
	}
	defer func() { _ = logger.Log.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := storage.Open(ctx, cfg.DatabaseURI)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Log.Error("close database", zap.Error(err))
		}
	}()

	if err := storage.Migrate(db); err != nil {
		return fmt.Errorf("migrate storage: %w", err)
	}

	s := storage.New(db)
	h := handler.New(s, s, s)
	router := handler.NewRouter(h)

	var workerWG sync.WaitGroup
	if cfg.AccrualAddress != "" {
		client := accrual.New(cfg.AccrualAddress)
		wrk := worker.New(s, client, logger.Log, workerCount, workerPollInterval)

		workerWG.Add(1)
		go func() {
			defer workerWG.Done()
			wrk.Run(ctx)
		}()
	}
	defer workerWG.Wait()

	server := &http.Server{
		Addr:    cfg.RunAddress,
		Handler: router,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Log.Info("starting server", zap.String("address", cfg.RunAddress))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- fmt.Errorf("listen and serve: %w", err)
			return
		}
		serverErr <- nil
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
	}

	logger.Log.Info("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown server: %w", err)
	}

	return <-serverErr
}
