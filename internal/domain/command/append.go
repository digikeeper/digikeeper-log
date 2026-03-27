package command

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/gitrus/digikeeper-log/internal/domain/errs"
	"github.com/gitrus/digikeeper-log/internal/domain/model"
)

func (s *Service) AppendEntry(
	ctx context.Context,
	req AppendRequest,
	requestID string,
) (model.Entry, error) {
	entry := model.Entry{
		ID: uuid.NewString(),
		Type: req.Type,
		Meta: model.EntryMeta{
			Version: 1,
			Src:     s.ResolveSrc(req.ClientID),
		},
		RequestID: requestID,
		CreatedAt: time.Now().UTC(),
		Timestamp: req.Timestamp,
		Tags:      req.Tags,
		Data:      req.Data,
	}
	if entry.Tags == nil {
		entry.Tags = []string{}
	}
	if entry.Data == nil {
		entry.Data = map[string]any{}
	}

	if err := s.storage.Append(ctx, entry); err != nil {
		if errors.Is(err, errs.IndexFailed) {
			s.logger.ErrorContext(ctx, "meta index failed — entry is durable in storage",
				slog.String("entry_id", entry.ID),
				slog.String("request_id", requestID),
				slog.Any("error", err),
			)
			return entry, err
		}
		return model.Entry{}, err
	}

	s.logger.InfoContext(ctx, "entry appended and indexed",
		slog.String("entry_id", entry.ID),
		slog.String("request_id", requestID),
	)

	return entry, nil
}
