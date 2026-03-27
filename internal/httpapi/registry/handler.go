package registry

import (
	"context"
)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

// --- note schema ---

type NoteSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]SchemaProperty `json:"properties"`
	Required   []string                  `json:"required"`
}

type SchemaProperty struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

var noteSchema = NoteSchema{
	Type: "object",
	Properties: map[string]SchemaProperty{
		"note": {Type: "string", Description: "Free-text note content"},
	},
	Required: []string{"note"},
}

// --- registry types ---

type SchemaEntry struct {
	Type   string `json:"type"`
	Schema any    `json:"schema"`
}

type ListOutput struct {
	Body struct {
		Schemas []SchemaEntry `json:"schemas"`
	}
}

func (h *Handler) ListSchemas(_ context.Context, _ *struct{}) (*ListOutput, error) {
	out := &ListOutput{}
	out.Body.Schemas = []SchemaEntry{
		{Type: "note", Schema: noteSchema},
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
	schemas := map[string]any{
		"note": noteSchema,
	}

	s, ok := schemas[input.Type]
	if !ok {
		return nil, nil
	}

	out := &GetOutput{}
	out.Body = SchemaEntry{Type: input.Type, Schema: s}
	return out, nil
}
