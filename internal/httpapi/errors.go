package httpapi

import (
	"fmt"

	"github.com/danielgtaylor/huma/v2"
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
				if d, ok := err.(*huma.ErrorDetail); ok {
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
