package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	_ "net/http/pprof" // /debug/pprof/* and /debug/vars
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	sloghttp "github.com/samber/slog-http"

	command "github.com/gitrus/digikeeper-log/internal/domain/command/append"
	domainCandidate "github.com/gitrus/digikeeper-log/internal/domain/command/candidate"
	domainCompaction "github.com/gitrus/digikeeper-log/internal/domain/command/compaction"
	"github.com/gitrus/digikeeper-log/internal/domain/query"
	"github.com/gitrus/digikeeper-log/internal/httpapi"
	apicmd "github.com/gitrus/digikeeper-log/internal/httpapi/command"
	apiqry "github.com/gitrus/digikeeper-log/internal/httpapi/query"
	apisreg "github.com/gitrus/digikeeper-log/internal/httpapi/schemaregistry"
	"github.com/gitrus/digikeeper-log/internal/infrastructure/candidatestore"
	store "github.com/gitrus/digikeeper-log/internal/infrastructure/commandstore"
	"github.com/gitrus/digikeeper-log/internal/infrastructure/index"
	"github.com/gitrus/digikeeper-log/internal/infrastructure/querystore"
	"github.com/gitrus/digikeeper-log/internal/infrastructure/sourcerepo"
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
	if err := os.MkdirAll(dataPath, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dataPath, err)
	}

	idx, err := index.New(filepath.Join(dataPath, "index.db"), index.Config{
		JournalMode: cfg.SQLite.JournalMode,
		BusyTimeout: cfg.SQLite.BusyTimeout,
	})
	if err != nil {
		return fmt.Errorf("init index: %w", err)
	}
	defer func() { _ = idx.Close() }()

	logStore, err := store.NewStore(dataPath, idx)
	if err != nil {
		return fmt.Errorf("init storage: %w", err)
	}
	defer func() { _ = logStore.Close() }()

	candidateStore, err := candidatestore.New(dataPath)
	if err != nil {
		return fmt.Errorf("init candidate storage: %w", err)
	}

	qryStore := querystore.NewStore(filepath.Join(dataPath, "dk_logs"), idx)

	// Sources
	srcRepo := sourcerepo.New()

	// Services
	cmdSvc := command.NewService(logStore, srcRepo, logger)
	candidateSvc := domainCandidate.NewService(
		candidateStore, logStore, logger,
	)
	compactionSvc := domainCompaction.NewService(
		logStore, candidateStore, idx, logger,
	)
	qrySvc := query.NewService(qryStore, qryStore, logger)

	// Handlers
	cmdHandler := apicmd.NewHandler(cmdSvc, srcRepo.ResolveName)
	candidateHandler := apicmd.NewCandidateHandler(candidateSvc)
	compactionHandler := apicmd.NewCompactionHandler(compactionSvc)
	qryHandler := apiqry.NewHandler(qrySvc, srcRepo.ResolveName)
	sregHandler, err := apisreg.NewHandler()
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
		OperationID:   "submit-candidate",
		Method:        http.MethodPost,
		Path:          "/v1/candidates",
		Summary:       "Submit a candidate replacement",
		DefaultStatus: http.StatusCreated,
	}, candidateHandler.SubmitCandidate)
	huma.Register(api, huma.Operation{
		OperationID:   "list-pending-candidates",
		Method:        http.MethodGet,
		Path:          "/v1/candidates/pending",
		Summary:       "List pending candidates",
		DefaultStatus: http.StatusOK,
	}, candidateHandler.ListPendingCandidates)
	huma.Register(api, huma.Operation{
		OperationID:   "resolve-candidates",
		Method:        http.MethodPost,
		Path:          "/v1/candidates/resolve",
		Summary:       "Resolve pending candidates for a partition",
		DefaultStatus: http.StatusOK,
	}, candidateHandler.ResolveCandidates)
	huma.Register(api, huma.Operation{
		OperationID:   "compact-partition",
		Method:        http.MethodPost,
		Path:          "/v1/compaction",
		Summary:       "Compact applied candidates into a log partition",
		DefaultStatus: http.StatusOK,
	}, compactionHandler.CompactPartition)
	huma.Register(api, huma.Operation{
		OperationID:   "list-schemas",
		Method:        http.MethodGet,
		Path:          "/v1/registry",
		Summary:       "List all entry type schemas",
		DefaultStatus: http.StatusOK,
	}, sregHandler.ListSchemas)
	huma.Register(api, huma.Operation{
		OperationID:   "get-schema",
		Method:        http.MethodGet,
		Path:          "/v1/registry/{type}",
		Summary:       "Get schema for an entry type",
		DefaultStatus: http.StatusOK,
	}, sregHandler.GetSchema)

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
		ReadTimeout:  6 * time.Second,
		WriteTimeout: 12 * time.Second,
		IdleTimeout:  18 * time.Second,
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
