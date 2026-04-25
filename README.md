# digikeeper-log

`digikeeper-log` is the storage layer for the Digikeeper personal logging app, responsible for persisting and indexing log entries.

Tech stack: HTTP service + JSONL storage + SQLite index.

## API
[https://huma.rocks/features/openapi-generation/](https://huma.rocks/features/openapi-generation/)

## Contributors guide

Requires Go 1.26+,
[just](https://github.com/casey/just).
Copy `.env.example` → `.env`.

```bash
just build   # → ./bin/server
just run     # go run
just lint    # golangci-lint
just fmt     # golangci-lint --fix
```

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for main concept decisions.
See docs/ for design decisions.
