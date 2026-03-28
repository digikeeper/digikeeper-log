package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	_ "net/http/pprof" // /debug/pprof/* and /debug/vars
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	sloghttp "github.com/samber/slog-http"

	"github.com/gitrus/digikeeper-log/internal/domain/command"
	"github.com/gitrus/digikeeper-log/internal/domain/model"
	"github.com/gitrus/digikeeper-log/internal/domain/query"
	"github.com/gitrus/digikeeper-log/internal/httpapi"
	apicmd "github.com/gitrus/digikeeper-log/internal/httpapi/command"
	apiqry "github.com/gitrus/digikeeper-log/internal/httpapi/query"
	apireg "github.com/gitrus/digikeeper-log/internal/httpapi/registry"
	store "github.com/gitrus/digikeeper-log/internal/infrastructure"
	"github.com/gitrus/digikeeper-log/pkg/chain"
	"github.com/gitrus/digikeeper-log/pkg/healthz"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	cfg := configure()

	level := slog.LevelInfo
	if cfg.IsDevEnv() {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	// Storage
	dataPath := cfg.LogStorage.Path
	logStore, err := store.NewStore(dataPath)
	if err != nil {
		return fmt.Errorf("init storage: %w", err)
	}
	defer func() { _ = logStore.Close() }()

	// Services
	cmdSvc := command.NewService(logStore, logger, cfg.ClientSources)
	qrySvc := query.NewService(logStore, logStore, logger)

	// Handlers
	resolveSrc := model.NewSourceResolver(cfg.ClientSources)
	cmdHandler := apicmd.NewHandler(cmdSvc, resolveSrc)
	qryHandler := apiqry.NewHandler(qrySvc, resolveSrc)
	regHandler, err := apireg.NewHandler()
	if err != nil {
		return fmt.Errorf("init registry: %w", err)
	}

	// API
	mux := http.NewServeMux()
	api := humago.New(mux, httpapi.NewHumaConfig("Digikeeper Log", "1.0.0"))
	httpapi.InitHumaErrors()

	huma.Register(api, huma.Operation{
		OperationID:   "list-logs",
		Method:        http.MethodGet,
		Path:          "/v1/logs",
		Summary:       "Search log entries",
		DefaultStatus: http.StatusOK,
	}, qryHandler.QueryLogs)
	huma.Register(api, huma.Operation{
		OperationID:   "append-log",
		Method:        http.MethodPost,
		Path:          "/v1/logs",
		Summary:       "Append a log entry",
		DefaultStatus: http.StatusCreated,
	}, cmdHandler.AppendLog)
	huma.Register(api, huma.Operation{
		OperationID:   "list-schemas",
		Method:        http.MethodGet,
		Path:          "/v1/registry",
		Summary:       "List all entry type schemas",
		DefaultStatus: http.StatusOK,
	}, regHandler.ListSchemas)
	huma.Register(api, huma.Operation{
		OperationID:   "get-schema",
		Method:        http.MethodGet,
		Path:          "/v1/registry/{type}",
		Summary:       "Get schema for an entry type",
		DefaultStatus: http.StatusOK,
	}, regHandler.GetSchema)

	mux.HandleFunc("GET /healthz", healthz.Handle)
	mux.Handle("/debug/", http.DefaultServeMux)

	sloghttp.RequestIDHeaderKey = "X-Request-ID"
	handler := chain.New(
		httpapi.Recovery,
		sloghttp.NewWithConfig(logger, sloghttp.Config{
			WithRequestID: true,
		}),
	).Then(mux)

	addr := fmt.Sprintf("%s:%s", cfg.API.LocalHost, cfg.API.LocalPort)
	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server starting",
			slog.String("addr", addr),
			slog.String("data_path", dataPath),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		stop()
		logger.Info("shutdown signal received", slog.String("cause", context.Cause(ctx).Error()))
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", slog.Any("error", err))
	}

	logger.Info("server stopped")
	return nil
}
