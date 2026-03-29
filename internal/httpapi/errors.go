package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/danielgtaylor/huma/v2"

	"github.com/gitrus/digikeeper-log/internal/domain/errs"
)

type jsonAPIError struct {
	status int
	Errors []ErrorDetail `json:"errors"`
}

func (e *jsonAPIError) Error() string {
	if len(e.Errors) == 0 {
		return "unknown error"
	}
	return e.Errors[0].Title
}
func (e *jsonAPIError) GetStatus() int { return e.status }

// DomainError maps a domain error to the appropriate huma HTTP error.
// Unknown/storage errors are logged and returned as 500.
func DomainError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, errs.ErrInvalidInput):
		return huma.Error422UnprocessableEntity(err.Error())
	case errors.Is(err, errs.ErrNotFound):
		return huma.Error404NotFound(err.Error())
	case errors.Is(err, errs.ErrConflict):
		return huma.Error409Conflict(err.Error())
	case errors.Is(err, errs.ErrCommonDomain):
		return huma.Error422UnprocessableEntity(err.Error())
	default:
		slog.Default().ErrorContext(ctx, "unexpected domain error", slog.Any("error", err))
		return huma.Error500InternalServerError("internal error")
	}
}

// InitHumaErrors overrides huma.NewError to produce JSON:API-shaped error responses.
func InitHumaErrors() {
	huma.NewError = func(status int, msg string, errs ...error) huma.StatusError {
		details := make([]ErrorDetail, 0, max(1, len(errs)))
		if len(errs) == 0 {
			details = append(details, ErrorDetail{
				Status: fmt.Sprintf("%d", status),
				Title:  msg,
			})
		} else {
			for _, err := range errs {
				detail := ErrorDetail{
					Status: fmt.Sprintf("%d", status),
					Title:  msg,
				}
				d := &huma.ErrorDetail{}
				if errors.As(err, &d) {
					detail.Detail = d.Message
				} else {
					detail.Detail = err.Error()
				}
				details = append(details, detail)
			}
		}
		return &jsonAPIError{
			status: status,
			Errors: details,
		}
	}
}
