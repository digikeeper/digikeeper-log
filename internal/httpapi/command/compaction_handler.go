package command

import (
	"context"

	"github.com/danielgtaylor/huma/v2"

	domainCompaction "github.com/digikeeper/digikeeper-journal/internal/domain/command/compaction"
	"github.com/digikeeper/digikeeper-journal/internal/domain/core"
	"github.com/digikeeper/digikeeper-journal/internal/httpapi"
)

type CompactionHandler struct {
	svc *domainCompaction.Service
}

func NewCompactionHandler(svc *domainCompaction.Service) *CompactionHandler {
	return &CompactionHandler{svc: svc}
}

type CompactPartitionInput struct {
	ClientID string `header:"X-Client-Id" doc:"Client identifier for audit"`
	Body     struct {
		Partition string `json:"partition" required:"true"`
	}
}

func (i *CompactPartitionInput) Resolve(ctx huma.Context) []error {
	if i.Body.Partition == "" {
		return []error{&huma.ErrorDetail{
			Location: "body.partition",
			Message:  "'partition' is required",
		}}
	}
	if _, err := core.ParsePartition(i.Body.Partition); err != nil {
		return []error{&huma.ErrorDetail{
			Location: "body.partition",
			Message:  "'partition' must be YYYY-MM-DD",
			Value:    i.Body.Partition,
		}}
	}
	return nil
}

type CompactPartitionOutput struct {
	Body struct {
		Partition string `json:"partition"`
		Status    string `json:"status"`
	}
}

func (h *CompactionHandler) CompactPartition(
	ctx context.Context,
	input *CompactPartitionInput,
) (*CompactPartitionOutput, error) {
	partition, err := core.ParsePartition(input.Body.Partition)
	if err != nil {
		return nil, huma.Error422UnprocessableEntity(err.Error())
	}

	if err := h.svc.Compact(ctx, domainCompaction.CompactRequest{Partition: partition}); err != nil {
		return nil, httpapi.DomainError(ctx, err)
	}

	out := &CompactPartitionOutput{}
	out.Body.Partition = partition.String()
	out.Body.Status = "completed"
	return out, nil
}
