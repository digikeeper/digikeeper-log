package registry

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

//go:embed schemas/*.json
var schemasFS embed.FS

type SchemaEntry struct {
	Type   string          `json:"type"`
	Schema json.RawMessage `json:"schema"`
}

type Handler struct {
	schemas map[string]json.RawMessage
}

func NewHandler() (*Handler, error) {
	entries, err := schemasFS.ReadDir("schemas")
	if err != nil {
		return nil, fmt.Errorf("registry: read embedded schemas: %w", err)
	}

	schemas := make(map[string]json.RawMessage, len(entries))
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := schemasFS.ReadFile("schemas/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("registry: read %s: %w", e.Name(), err)
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		schemas[name] = json.RawMessage(data)
	}

	return &Handler{schemas: schemas}, nil
}

type ListOutput struct {
	Body struct {
		Schemas []SchemaEntry `json:"schemas"`
	}
}

func (h *Handler) ListSchemas(_ context.Context, _ *struct{}) (*ListOutput, error) {
	out := &ListOutput{}
	out.Body.Schemas = make([]SchemaEntry, 0, len(h.schemas))
	for name, raw := range h.schemas {
		out.Body.Schemas = append(out.Body.Schemas, SchemaEntry{Type: name, Schema: raw})
	}
	return out, nil
}

type GetInput struct {
	Type string `path:"type" doc:"Schema type name"`
}

type GetOutput struct {
	Body SchemaEntry
}

func (h *Handler) GetSchema(_ context.Context, input *GetInput) (*GetOutput, error) {
	raw, ok := h.schemas[input.Type]
	if !ok {
		return nil, nil
	}

	out := &GetOutput{}
	out.Body = SchemaEntry{Type: input.Type, Schema: raw}
	return out, nil
}
