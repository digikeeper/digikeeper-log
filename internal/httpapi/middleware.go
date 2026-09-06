package httpapi

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"

	"github.com/digikeeper/digikeeper-journal/internal/jsonx"
)

// Recovery returns middleware that catches panics, logs them with slog,
// and responds with a JSON:API 500 error.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if err, ok := v.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					return
				}

				buf := make([]byte, 4096)
				n := runtime.Stack(buf, false)

				slog.ErrorContext(r.Context(), "panic recovered",
					slog.Any("error", v),
					slog.String("stack", string(buf[:n])),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
				)

				w.Header().Set("Content-Type", ContentType)
				w.WriteHeader(http.StatusInternalServerError)
				_ = jsonx.MarshalWrite(w, struct {
					Errors []ErrorDetail `json:"errors"`
				}{
					Errors: []ErrorDetail{{
						Status: fmt.Sprintf("%d", http.StatusInternalServerError),
						Title:  "Internal Server Error",
					}},
				})
			}
		}()

		next.ServeHTTP(w, r)
	})
}
