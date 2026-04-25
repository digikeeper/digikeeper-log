# Architecture

## Overview
An HTTP service for append-based personal logs.
It accepts log entries over a REST API, stores them durably in JSONL files (source of truth), and maintains a SQLite index for fast querying.

The mutex and locks of data changes is based on flock -- prevents concurrency on OS-dir level.

## CQS
Logic of service organically divided into command and query.
- **Command** (`internal/command`): JSONL append + SQLite index update.
- **Query** (`internal/query`): SQLite narrows to candidate files → JSONL scan + entry-level filtering.

Currently synchronous. The `command.Storage` interface is compatible with future async indexing.

## Layering
```
slog-http middleware        ← RequestID, access logs
internal/httpapi/*          ← parse, validate, respond
internal/{command,query}    ← business logic, owns interfaces
internal/infrastructure     ← Store facade → jsonlstore + SQLiteIndex
```
Dependencies flow inward. Interfaces defined at the usage-model-level.

## Observability
- RequestID in every JSONL entry + `X-Request-ID` header
- `slog.JSONHandler` structured logging
- `expvar`: `records_appended`, `sqlite_index_latency_ms` at `/debug/vars`

## API Convention
The API follows the main guidelines of [JSON:API](https://jsonapi.org/) specification.
Before making API design decisions, consult the spec and its addendums first.

## Trade-offs

| Decision | Revisit when |
|----------|-------------|
| Synchronous write+index | Write latency matters |
| expvar metrics | Need Prometheus/OTel |
| No auth | Exposed to untrusted network |
| File-level index (not per-entry) | Need sub-file granularity |
| Concrete types in handlers | Adding handler-level tests |
