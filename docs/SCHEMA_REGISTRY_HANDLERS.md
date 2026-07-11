# Schema Registry Handlers

## Description

Schema registry handlers expose supported log entry schemas over HTTP.
They live in `internal/httpapi/schemaregistry` and are wired from `cmd/server`.

Endpoints:

- `GET /v1/registry` returns schema type summaries with their latest and available versions.
- `GET /v1/registry/{type}` returns the latest schema for an entry type.
- `GET /v1/registry/{type}/{version}` returns one immutable schema version.

Schemas are JSON files in `internal/httpapi/schemaregistry/schemas`. The handler embeds
and loads them at startup, retaining each schema as `json.RawMessage`.

Schema filenames use this required format: `<type>_v<positive-integer>.json`

```text
schemas/note_v1.json  → type: note, version: 1
schemas/note_v2.json  → type: note, version: 2
```


## Versioning Contract

A schema identity is `(type, version)`. Published schema files are immutable == add a new
file for a changed schema instead of modifying an existing version.

An entry persists its schema version in `m.v`. This identifies the exact registry schema
needed to interpret that entry; it never means "latest".

Entry metadata also contains `m.r`, its logical revision:

- a new entry starts at revision `1`;
- applying an approved candidate increments the revision;
- a storage-only compaction rewrite does not increment it.

## Why It Exists

Clients need a stable way to discover supported entry types and the schema version used
by persisted entries. Serving schemas from the running service keeps clients aligned with
the deployed version instead of relying only on external documentation.

## Boundaries

The schema registry is read-only application metadata, not user data. Schema changes go
through code review and deployment.

The handler stays in `internal/httpapi` because it has no business workflow or mutable
storage. If schemas become editable or user-specific, this package should be revised.
