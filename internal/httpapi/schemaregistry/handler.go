package schemaregistry

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

//go:embed schemas/*.json
var schemasFS embed.FS

type SchemaEntry struct {
	Type       string          `json:"type"`
	Version    int             `json:"version"`
	JSONSchema json.RawMessage `json:"schema"`
}

type SchemaSummary struct {
	Type          string `json:"type"`
	LatestVersion int    `json:"latest_version"`
	Versions      []int  `json:"versions"`
}

type Handler struct {
	schemas map[string]map[int]json.RawMessage
	order   []string
}

func NewHandler() (*Handler, error) {
	entries, err := schemasFS.ReadDir("schemas")
	if err != nil {
		return nil, fmt.Errorf("schemaregistry: read embedded schemas: %w", err)
	}

	schemas := make(map[string]map[int]json.RawMessage, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}

		typeName, version, err := parseSchemaFilename(e.Name())
		if err != nil {
			return nil, err
		}
		data, err := schemasFS.ReadFile("schemas/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("schemaregistry: read %s: %w", e.Name(), err)
		}

		versions := schemas[typeName]
		if versions == nil {
			versions = make(map[int]json.RawMessage)
			schemas[typeName] = versions
		}
		if _, exists := versions[version]; exists {
			return nil, fmt.Errorf("schemaregistry: duplicate schema %s version %d", typeName, version)
		}
		versions[version] = json.RawMessage(data)
	}

	order := make([]string, 0, len(schemas))
	for typeName := range schemas {
		order = append(order, typeName)
	}
	slices.Sort(order)

	return &Handler{schemas: schemas, order: order}, nil
}

func parseSchemaFilename(name string) (string, int, error) {
	stem := strings.TrimSuffix(name, ".json")
	idx := strings.LastIndex(stem, "_v")
	if idx <= 0 || idx == len(stem)-2 {
		return "", 0, fmt.Errorf(
			"schemaregistry: invalid schema filename %q: expected <type>_v<positive-integer>.json",
			name,
		)
	}

	version, err := strconv.Atoi(stem[idx+2:])
	if err != nil || version < 1 {
		return "", 0, fmt.Errorf(
			"schemaregistry: invalid schema filename %q: expected <type>_v<positive-integer>.json",
			name,
		)
	}
	return stem[:idx], version, nil
}

type ListOutput struct {
	Body struct {
		Schemas []SchemaSummary `json:"schemas"`
	}
}

func (h *Handler) ListSchemas(_ context.Context, _ *struct{}) (*ListOutput, error) {
	out := &ListOutput{}
	out.Body.Schemas = make([]SchemaSummary, 0, len(h.schemas))
	for _, typeName := range h.order {
		versions := h.versions(typeName)
		out.Body.Schemas = append(out.Body.Schemas, SchemaSummary{
			Type:          typeName,
			LatestVersion: versions[len(versions)-1],
			Versions:      versions,
		})
	}
	return out, nil
}

type GetInput struct {
	Type string `path:"type" doc:"Schema type name"`
}

type GetVersionInput struct {
	Type    string `path:"type" doc:"Schema type name"`
	Version int    `path:"version" minimum:"1" doc:"Immutable schema version"`
}

type GetOutput struct {
	Body SchemaEntry
}

func (h *Handler) GetSchema(_ context.Context, input *GetInput) (*GetOutput, error) {
	versions := h.versions(input.Type)
	if len(versions) == 0 {
		return nil, huma.Error404NotFound("schema type not found: " + input.Type)
	}
	return h.getSchema(input.Type, versions[len(versions)-1])
}

func (h *Handler) GetSchemaVersion(_ context.Context, input *GetVersionInput) (*GetOutput, error) {
	return h.getSchema(input.Type, input.Version)
}

func (h *Handler) getSchema(typeName string, version int) (*GetOutput, error) {
	raw, ok := h.schemas[typeName][version]
	if !ok {
		return nil, huma.Error404NotFound(fmt.Sprintf("schema not found: %s version %d", typeName, version))
	}

	out := &GetOutput{}
	out.Body = SchemaEntry{Type: typeName, Version: version, JSONSchema: raw}
	return out, nil
}

func (h *Handler) versions(typeName string) []int {
	versions := make([]int, 0, len(h.schemas[typeName]))
	for version := range h.schemas[typeName] {
		versions = append(versions, version)
	}
	slices.Sort(versions)
	return versions
}
