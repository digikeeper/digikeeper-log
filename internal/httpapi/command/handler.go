package command

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/danielgtaylor/huma/v2"
	sloghttp "github.com/samber/slog-http"

	domainCmd "github.com/gitrus/digikeeper-log/internal/domain/command"
	"github.com/gitrus/digikeeper-log/internal/domain/errs"
	"github.com/gitrus/digikeeper-log/internal/domain/model"
	"github.com/gitrus/digikeeper-log/internal/httpapi"
)

type Handler struct {
	svc        *domainCmd.Service
	resolveSrc model.SourceResolver
}

func NewHandler(svc *domainCmd.Service, resolveSrc model.SourceResolver) *Handler {
	return &Handler{svc: svc, resolveSrc: resolveSrc}
}

type AppendInput struct {
	ClientID string `header:"X-Client-Id" doc:"Client identifier for source tracking"`
	Body     struct {
		Type      string         `json:"type" doc:"Entry type"`
		Timestamp time.Time      `json:"timestamp" required:"true" doc:"Event timestamp"`
		Tags      []string       `json:"tags" doc:"Entry tags"`
		Data      map[string]any `json:"data" doc:"Entry data"`
	}
}

func (i *AppendInput) Resolve(ctx huma.Context) []error {
	if len(i.Body.Tags) == 0 && len(i.Body.Data) == 0 {
		return []error{&huma.ErrorDetail{
			Message: "at least one of 'tags' or 'data' must be provided",
		}}
	}
	if i.Body.Timestamp.IsZero() {
		return []error{&huma.ErrorDetail{
			Location: "body.timestamp",
			Message:  "'timestamp' is required",
		}}
	}
	return nil
}

type AppendOutput struct {
	Status int
	Body   struct {
		Meta httpapi.ResponseMeta     `json:"meta"`
		Data httpapi.ResourceEnvelope `json:"data"`
	}
}

func (h *Handler) AppendLog(ctx context.Context, input *AppendInput) (*AppendOutput, error) {
	req := domainCmd.AppendRequest{
		Type:      input.Body.Type,
		Timestamp: input.Body.Timestamp,
		Tags:      input.Body.Tags,
		Data:      input.Body.Data,
		ClientID:  input.ClientID,
	}

	requestID := sloghttp.GetRequestIDFromContext(ctx)
	entry, err := h.svc.AppendEntry(ctx, req, requestID)
	switch {
	case err == nil:
		out := &AppendOutput{}
		out.Status = 201
		out.Body.Meta = httpapi.ResponseMeta{Type: "logs"}
		out.Body.Data = httpapi.ToEnvelope(httpapi.NewEntryResource(entry, h.resolveSrc))
		return out, nil

	case errors.Is(err, errs.IndexFailed):
		out := &AppendOutput{}
		out.Status = 202
		out.Body.Meta = httpapi.ResponseMeta{Type: "logs"}
		out.Body.Data = httpapi.ToEnvelope(httpapi.NewEntryResource(entry, h.resolveSrc))
		return out, nil

	default:
		slog.Default().ErrorContext(ctx, "append failed", slog.Any("error", err))
		return nil, huma.Error500InternalServerError("Storage Failure")
	}
}
