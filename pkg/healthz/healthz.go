package healthz

import (
	"net/http"
)

// Handle is a liveness probe handler that returns {"status":"ok"}.
func Handle(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}` + "\n"))
}
