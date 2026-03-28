package query

import (
	"context"
	"log/slog"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/gitrus/digikeeper-log/internal/domain/model"
	domainQuery "github.com/gitrus/digikeeper-log/internal/domain/query"
	"github.com/gitrus/digikeeper-log/internal/httpapi"
)

type Handler struct {
	svc        *domainQuery.Service
	resolveSrc func(int) string
}

func NewHandler(svc *domainQuery.Service, resolveSrc func(int) string) *Handler {
	return &Handler{svc: svc, resolveSrc: resolveSrc}
}

type QueryInput struct {
	Tag   string    `query:"tag" doc:"Filter by tag"`
	From  time.Time `query:"from" doc:"Start time (RFC3339)"`
	To    time.Time `query:"to" doc:"End time (RFC3339)"`
	Limit int       `query:"limit" minimum:"1" maximum:"1000" default:"100" doc:"Result limit"`
}

func (i *QueryInput) Resolve(ctx huma.Context) []error {
	if !i.From.IsZero() && !i.To.IsZero() && i.From.After(i.To) {
		return []error{&huma.ErrorDetail{
			Location: "query.from",
			Message:  "'from' must not be after 'to'",
			Value:    i.From,
		}}
	}
	return nil
}

type QueryOutput struct {
	Body struct {
		Meta httpapi.ResponseMeta       `json:"meta"`
		Data []httpapi.ResourceEnvelope `json:"data"`
	}
}

func (h *Handler) QueryLogs(ctx context.Context, input *QueryInput) (*QueryOutput, error) {
	params := model.SearchParams{
		Tag:   input.Tag,
		From:  input.From,
		To:    input.To,
		Limit: input.Limit,
	}

	results, err := h.svc.SearchEntries(ctx, params)
	if err != nil {
		slog.Default().ErrorContext(ctx, "search failed", slog.Any("error", err))
		return nil, huma.Error500InternalServerError("Query Failure")
	}

	out := &QueryOutput{}
	out.Body.Meta = httpapi.ResponseMeta{Type: "logs"}
	out.Body.Data = make([]httpapi.ResourceEnvelope, len(results))
	for i, e := range results {
		out.Body.Data[i] = httpapi.ToEnvelope(httpapi.NewEntryResource(e, h.resolveSrc))
	}
	return out, nil
}
