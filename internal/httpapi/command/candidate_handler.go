package command

import (
	"context"
	"time"

	"github.com/danielgtaylor/huma/v2"
	sloghttp "github.com/samber/slog-http"

	domainCandidate "github.com/gitrus/digikeeper-log/internal/domain/command/candidate"
	commandmodel "github.com/gitrus/digikeeper-log/internal/domain/command/model"
	"github.com/gitrus/digikeeper-log/internal/domain/core"
	"github.com/gitrus/digikeeper-log/internal/httpapi"
)

type CandidateHandler struct {
	svc *domainCandidate.Service
}

func NewCandidateHandler(svc *domainCandidate.Service) *CandidateHandler {
	return &CandidateHandler{svc: svc}
}

type SubmitCandidateInput struct {
	ClientID string `header:"X-Client-Id" doc:"Client identifier for audit"`
	Body     struct {
		EntryID           string         `json:"entry_id" required:"true"`
		OriginalTimestamp time.Time      `json:"original_timestamp" required:"true"`
		Type              string         `json:"type"`
		Tags              []string       `json:"tags"`
		Data              map[string]any `json:"data"`
	}
}

func (i *SubmitCandidateInput) Resolve(ctx huma.Context) []error {
	var errs []error
	if i.Body.EntryID == "" {
		errs = append(errs, &huma.ErrorDetail{
			Location: "body.entry_id",
			Message:  "'entry_id' is required",
		})
	}
	if i.Body.OriginalTimestamp.IsZero() {
		errs = append(errs, &huma.ErrorDetail{
			Location: "body.original_timestamp",
			Message:  "'original_timestamp' is required",
		})
	}
	if len(i.Body.Tags) == 0 && len(i.Body.Data) == 0 {
		errs = append(errs, &huma.ErrorDetail{
			Message: "at least one of 'tags' or 'data' must be provided",
		})
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

type SubmitCandidateOutput struct {
	Status int
	Body   struct {
		Meta httpapi.ResponseMeta     `json:"meta"`
		Data httpapi.ResourceEnvelope `json:"data"`
	}
}

func (h *CandidateHandler) SubmitCandidate(
	ctx context.Context,
	input *SubmitCandidateInput,
) (*SubmitCandidateOutput, error) {
	c, err := h.svc.Submit(ctx, domainCandidate.SubmitRequest{
		EntryID:           input.Body.EntryID,
		OriginalTimestamp: input.Body.OriginalTimestamp,
		Type:              input.Body.Type,
		Tags:              input.Body.Tags,
		Data:              input.Body.Data,
		ClientID:          input.ClientID,
	}, sloghttp.GetRequestIDFromContext(ctx))
	if err != nil {
		return nil, httpapi.DomainError(ctx, err)
	}

	out := &SubmitCandidateOutput{Status: 201}
	out.Body.Meta = httpapi.ResponseMeta{Type: "candidates"}
	out.Body.Data = httpapi.ToEnvelope(NewCandidateResource(c))
	return out, nil
}

type ResolveCandidatesInput struct {
	ClientID   string `header:"X-Client-Id" doc:"Client identifier for audit"`
	ResolvedBy string `header:"X-Resolved-By" required:"true" doc:"Resolver identifier"`
	Body       struct {
		Partition   string                        `json:"partition" required:"true"`
		Resolutions []domainCandidate.ResolveItem `json:"resolutions" required:"true"`
	}
}

type ListPendingCandidatesInput struct {
	Partition string `query:"partition" required:"true" doc:"Partition to resolve in YYYY-MM-DD format"`
}

func (i *ListPendingCandidatesInput) Resolve(ctx huma.Context) []error {
	if i.Partition == "" {
		return []error{&huma.ErrorDetail{
			Location: "query.partition",
			Message:  "'partition' is required",
		}}
	}
	if _, err := core.ParsePartition(i.Partition); err != nil {
		return []error{&huma.ErrorDetail{
			Location: "query.partition",
			Message:  "'partition' must be YYYY-MM-DD",
			Value:    i.Partition,
		}}
	}
	return nil
}

type ListPendingCandidatesOutput struct {
	Body struct {
		Meta httpapi.ResponseMeta       `json:"meta"`
		Data []httpapi.ResourceEnvelope `json:"data"`
	}
}

func (h *CandidateHandler) ListPendingCandidates(
	ctx context.Context,
	input *ListPendingCandidatesInput,
) (*ListPendingCandidatesOutput, error) {
	partition, err := core.ParsePartition(input.Partition)
	if err != nil {
		return nil, huma.Error422UnprocessableEntity(err.Error())
	}

	candidates, err := h.svc.ListPending(ctx, partition)
	if err != nil {
		return nil, httpapi.DomainError(ctx, err)
	}

	out := &ListPendingCandidatesOutput{}
	out.Body.Meta = httpapi.ResponseMeta{Type: "candidates"}
	out.Body.Data = candidateEnvelopes(candidates)
	return out, nil
}

func (i *ResolveCandidatesInput) Resolve(ctx huma.Context) []error {
	var errs []error
	if i.ResolvedBy == "" {
		errs = append(errs, &huma.ErrorDetail{
			Location: "header.X-Resolved-By",
			Message:  "'X-Resolved-By' is required",
		})
	}
	if i.Body.Partition == "" {
		errs = append(errs, &huma.ErrorDetail{
			Location: "body.partition",
			Message:  "'partition' is required",
		})
	} else if _, err := core.ParsePartition(i.Body.Partition); err != nil {
		errs = append(errs, &huma.ErrorDetail{
			Location: "body.partition",
			Message:  "'partition' must be YYYY-MM-DD",
			Value:    i.Body.Partition,
		})
	}
	if len(i.Body.Resolutions) == 0 {
		errs = append(errs, &huma.ErrorDetail{
			Location: "body.resolutions",
			Message:  "'resolutions' must not be empty",
		})
	}
	for idx, item := range i.Body.Resolutions {
		if item.CandidateID == "" {
			errs = append(errs, &huma.ErrorDetail{
				Location: "body.resolutions",
				Message:  "'candidate_id' is required",
				Value:    idx,
			})
		}
		if !item.Action.IsValid() {
			errs = append(errs, &huma.ErrorDetail{
				Location: "body.resolutions.action",
				Message:  "'action' must be 'apply' or 'deny'",
				Value:    item.Action,
			})
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

type ResolveCandidatesOutput struct {
	Body struct {
		Meta httpapi.ResponseMeta       `json:"meta"`
		Data []httpapi.ResourceEnvelope `json:"data"`
	}
}

func (h *CandidateHandler) ResolveCandidates(
	ctx context.Context,
	input *ResolveCandidatesInput,
) (*ResolveCandidatesOutput, error) {
	partition, err := core.ParsePartition(input.Body.Partition)
	if err != nil {
		return nil, huma.Error422UnprocessableEntity(err.Error())
	}

	candidates, err := h.svc.Resolve(ctx, partition, domainCandidate.ResolveRequest{
		Resolutions: input.Body.Resolutions,
		ResolvedBy:  input.ResolvedBy,
		ClientID:    input.ClientID,
	})
	if err != nil {
		return nil, httpapi.DomainError(ctx, err)
	}

	out := &ResolveCandidatesOutput{}
	out.Body.Meta = httpapi.ResponseMeta{Type: "candidates"}
	out.Body.Data = candidateEnvelopes(candidates)
	return out, nil
}

func candidateEnvelopes(candidates []commandmodel.Candidate) []httpapi.ResourceEnvelope {
	resources := make([]httpapi.ResourceEnvelope, len(candidates))
	for i, c := range candidates {
		resources[i] = httpapi.ToEnvelope(NewCandidateResource(c))
	}
	return resources
}
