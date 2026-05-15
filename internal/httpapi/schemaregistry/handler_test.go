package registry_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gitrus/digikeeper-log/internal/httpapi"
	apireg "github.com/gitrus/digikeeper-log/internal/httpapi/registry"
)

func setupRegistryServer(t *testing.T) *httptest.Server {
	t.Helper()

	h, err := apireg.NewHandler()
	require.NoError(t, err)

	mux := http.NewServeMux()
	api := humago.New(mux, httpapi.NewHumaConfig("test", "0.0.0"))
	httpapi.InitHumaErrors()

	huma.Register(api, huma.Operation{
		OperationID:   "get-schema",
		Method:        http.MethodGet,
		Path:          "/v1/registry/{type}",
		DefaultStatus: http.StatusOK,
	}, h.GetSchema)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func get(t *testing.T, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func TestGetSchema_UnknownType_Returns404(t *testing.T) {
	t.Parallel()
	srv := setupRegistryServer(t)

	resp := get(t, srv.URL+"/v1/registry/missing")
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	var body struct {
		Errors []any `json:"errors"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.NotEmpty(t, body.Errors)
}

func TestGetSchema_KnownType_Returns200(t *testing.T) {
	t.Parallel()
	srv := setupRegistryServer(t)

	resp := get(t, srv.URL+"/v1/registry/note")
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
