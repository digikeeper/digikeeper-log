# Registry Handlers

## Description

Registry handlers expose supported log entry schemas over HTTP.
They live in `internal/httpapi/registry` and are wired from `cmd/server`.

Current endpoints:
- `GET /v1/registry` returns all known entry schemas.
- `GET /v1/registry/{type}` returns one schema by entry type.

Schemas are JSON files in `internal/httpapi/registry/schemas`.
The handler embeds them with `go:embed`, loads them at startup, and keeps each schema as `json.RawMessage`.
The filename without `.json` becomes the public entry type name.

Example:
- `schemas/note.json` becomes registry type `note`.

## Why It Exists

Clients need a stable way to discover which entry types the service accepts and how each entry payload is shaped.
Serving schemas from the running service keeps clients aligned with the deployed version instead of relying only on external documentation.
It cna be used by agentic client as well.

## Boundaries

The registry is read-only application metadata, not user data.
Schema changes should go through code review and deployment.

The handler stays in `internal/httpapi` because it has no business workflow or mutable storage.
If schemas become editable, user-specific, or etc, this package should be revised

The response shape is intentionally small:
- `type` identifies the entry type
- `schema` contains the raw schema document
