package tests

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	sloghttp "github.com/samber/slog-http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	command "github.com/gitrus/digikeeper-log/internal/domain/command/append"
	"github.com/gitrus/digikeeper-log/internal/domain/query"
	"github.com/gitrus/digikeeper-log/internal/httpapi"
	apicmd "github.com/gitrus/digikeeper-log/internal/httpapi/command"
	apiqry "github.com/gitrus/digikeeper-log/internal/httpapi/query"
	apireg "github.com/gitrus/digikeeper-log/internal/httpapi/registry"
	store "github.com/gitrus/digikeeper-log/internal/infrastructure/commandstore"
	"github.com/gitrus/digikeeper-log/internal/infrastructure/index"
	"github.com/gitrus/digikeeper-log/internal/infrastructure/querystore"
	"github.com/gitrus/digikeeper-log/internal/infrastructure/sourcerepo"
	"github.com/gitrus/digikeeper-log/internal/jsonx"
)

// --- test-local JSON:API response types ---

type singleResponse struct {
	Meta responseMeta   `json:"meta"`
	Data resourceObject `json:"data"`
}

type listResponse struct {
	Meta responseMeta     `json:"meta"`
	Data []resourceObject `json:"data"`
}

type responseMeta struct {
	Type string `json:"type"`
}

type resourceObject struct {
	ID         string     `json:"id"`
	Attributes entryAttrs `json:"attributes"`
}

type entryAttrs struct {
	Type      string         `json:"type"`
	Meta      entryMeta      `json:"meta"`
	RequestID string         `json:"request_id"`
	CreatedAt string         `json:"created_at"`
	Timestamp string         `json:"timestamp"`
	Tags      []string       `json:"tags"`
	Data      map[string]any `json:"data"`
}

type entryMeta struct {
	Version int    `json:"version"`
	Source  string `json:"source"`
}

func setupTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	dir := t.TempDir()

	idx, err := index.New(filepath.Join(dir, "index.db"))
	require.NoError(t, err, "init index")
	t.Cleanup(func() { _ = idx.Close() })

	logStore, err := store.NewStore(dir, idx)
	require.NoError(t, err, "init store")
	t.Cleanup(func() { _ = logStore.Close() })

	qryStore := querystore.NewStore(filepath.Join(dir, "dk_logs"), idx)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	srcRepo := sourcerepo.New()

	cmdSvc := command.NewService(logStore, srcRepo, logger)
	qrySvc := query.NewService(qryStore, qryStore, logger)

	cmdHandler := apicmd.NewHandler(cmdSvc, srcRepo.ResolveName)
	qryHandler := apiqry.NewHandler(qrySvc, srcRepo.ResolveName)
	regHandler, err := apireg.NewHandler()
	require.NoError(t, err, "init registry")

	mux := http.NewServeMux()
	api := humago.New(mux, httpapi.NewHumaConfig("Digikeeper Log", "1.0.0"))
	httpapi.InitHumaErrors()

	huma.Register(api, huma.Operation{
		OperationID:   "list-logs",
		Method:        http.MethodGet,
		Path:          "/v1/logs",
		Summary:       "Search log entries",
		DefaultStatus: http.StatusOK,
	}, qryHandler.QueryLogs)

	huma.Register(api, huma.Operation{
		OperationID:   "append-log",
		Method:        http.MethodPost,
		Path:          "/v1/logs",
		Summary:       "Append a log entry",
		DefaultStatus: http.StatusCreated,
	}, cmdHandler.AppendLog)
	huma.Register(api, huma.Operation{
		OperationID:   "list-schemas",
		Method:        http.MethodGet,
		Path:          "/v1/registry",
		Summary:       "List all entry type schemas",
		DefaultStatus: http.StatusOK,
	}, regHandler.ListSchemas)
	huma.Register(api, huma.Operation{
		OperationID:   "get-schema",
		Method:        http.MethodGet,
		Path:          "/v1/registry/{type}",
		Summary:       "Get schema for an entry type",
		DefaultStatus: http.StatusOK,
	}, regHandler.GetSchema)

	sloghttp.RequestIDHeaderKey = "X-Request-ID"
	handler := sloghttp.NewWithConfig(logger, sloghttp.Config{
		WithRequestID: true,
	})(mux)

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func newTestRequest(t *testing.T, method, url string, body io.Reader) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, url, body)
	require.NoError(t, err)
	return req
}

func doTestRequest(t *testing.T, req *http.Request) *http.Response {
	t.Helper()
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func postJSON(t *testing.T, url, body string) *http.Response {
	t.Helper()
	req := newTestRequest(t, http.MethodPost, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return doTestRequest(t, req)
}

func getURL(t *testing.T, url string) *http.Response {
	t.Helper()
	return doTestRequest(t, newTestRequest(t, http.MethodGet, url, nil))
}

func closeResponseBody(t *testing.T, resp *http.Response) {
	t.Helper()
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("close response body: %v", err)
	}
}

func TestPostEntry(t *testing.T) {
	srv := setupTestServer(t)

	body := `{"type":"note","timestamp":"2026-03-08T10:00:00Z","tags":["work"],"data":{"note":"test"}}`
	// act
	resp := postJSON(t, srv.URL+"/v1/logs", body)
	defer closeResponseBody(t, resp)

	require.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, "application/vnd.api+json", resp.Header.Get("Content-Type"))

	// assert
	var got singleResponse
	require.NoError(t, jsonx.UnmarshalRead(resp.Body, &got))

	assert.Equal(t, "logs", got.Meta.Type)
	assert.NotEmpty(t, got.Data.ID)
	assert.Equal(t, "note", got.Data.Attributes.Type)
	assert.Equal(t, "2026-03-08T10:00:00Z", got.Data.Attributes.Timestamp)
	assert.Equal(t, []string{"work"}, got.Data.Attributes.Tags)
	assert.Equal(t, "test", got.Data.Attributes.Data["note"])
	assert.Equal(t, 1, got.Data.Attributes.Meta.Version)
	assert.Equal(t, "", got.Data.Attributes.Meta.Source)
	assert.NotEmpty(t, got.Data.Attributes.CreatedAt)
}

func TestPostThenGet(t *testing.T) {
	srv := setupTestServer(t)

	// act POST
	for _, postBody := range []string{
		`{"type":"note","timestamp":"2026-03-08T14:30:00Z","tags":["fitness","health"],"data":{"exercise":"running"}}`,
		`{"type":"note","timestamp":"2026-03-08T14:29:00Z","tags":["health"],"data":{"exercise":"pre-running"}}`,
	} {
		postResp := postJSON(t, srv.URL+"/v1/logs", postBody)
		defer closeResponseBody(t, postResp)
		require.Equal(t, http.StatusCreated, postResp.StatusCode)

		var postResult singleResponse
		require.NoError(t, jsonx.UnmarshalRead(postResp.Body, &postResult))
	}

	// act GET
	getResp := getURL(t, srv.URL+"/v1/logs?tag=fitness")
	defer closeResponseBody(t, getResp)

	require.Equal(t, http.StatusOK, getResp.StatusCode)
	assert.Equal(t, "application/vnd.api+json", getResp.Header.Get("Content-Type"))

	// assert
	var getResult listResponse
	require.NoError(t, jsonx.UnmarshalRead(getResp.Body, &getResult))

	assert.Equal(t, "logs", getResult.Meta.Type)
	require.NotEmpty(t, getResult.Data)

	var found *resourceObject
	assert.Len(t, getResult.Data, 1)
	found = &getResult.Data[0]

	assert.Contains(t, found.Attributes.Tags, "fitness")
	assert.Equal(t, "running", found.Attributes.Data["exercise"])
	assert.Equal(t, "2026-03-08T14:30:00Z", found.Attributes.Timestamp)
}

func TestAppendWithClientID(t *testing.T) {
	srv := setupTestServer(t)

	body := `{"type":"note","timestamp":"2026-03-08T10:00:00Z","tags":["work"],"data":{"note":"test"}}`

	tests := []struct {
		name       string
		clientID   string
		wantSource string
	}{
		{"known client", "mobile", "mobile"},
		{"unknown client", "desktop", ""},
		{"empty client", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := newTestRequest(t, http.MethodPost, srv.URL+"/v1/logs", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			if tc.clientID != "" {
				req.Header.Set("X-Client-Id", tc.clientID)
			}

			resp := doTestRequest(t, req)
			defer closeResponseBody(t, resp)

			require.Equal(t, http.StatusCreated, resp.StatusCode)

			var got singleResponse
			require.NoError(t, jsonx.UnmarshalRead(resp.Body, &got))
			assert.Equal(t, tc.wantSource, got.Data.Attributes.Meta.Source)
		})
	}
}

func TestAppendPassesRequestID(t *testing.T) {
	srv := setupTestServer(t)

	body := `{"type":"note","timestamp":"2026-03-08T10:00:00Z","tags":["work"],"data":{"note":"test"}}`
	req := newTestRequest(t, http.MethodPost, srv.URL+"/v1/logs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", "test-req-123")

	resp := doTestRequest(t, req)
	defer closeResponseBody(t, resp)

	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var got singleResponse
	require.NoError(t, jsonx.UnmarshalRead(resp.Body, &got))
	assert.Equal(t, "test-req-123", got.Data.Attributes.RequestID)
}

func TestAppendGeneratesRequestIDWhenMissing(t *testing.T) {
	srv := setupTestServer(t)

	body := `{"type":"note","timestamp":"2026-03-08T10:00:00Z","tags":["work"],"data":{"note":"test"}}`
	resp := postJSON(t, srv.URL+"/v1/logs", body)
	defer closeResponseBody(t, resp)

	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var got singleResponse
	require.NoError(t, jsonx.UnmarshalRead(resp.Body, &got))
	assert.NotEmpty(t, got.Data.Attributes.RequestID)
}

func TestRegistryListSchemas(t *testing.T) {
	srv := setupTestServer(t)

	resp := getURL(t, srv.URL+"/v1/registry")
	defer closeResponseBody(t, resp)

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got struct {
		Schemas []struct {
			Type   string `json:"type"`
			Schema any    `json:"schema"`
		} `json:"schemas"`
	}
	require.NoError(t, jsonx.UnmarshalRead(resp.Body, &got))
	require.Len(t, got.Schemas, 1)
	assert.Equal(t, "note", got.Schemas[0].Type)
}

func TestRegistryGetSchema(t *testing.T) {
	srv := setupTestServer(t)

	resp := getURL(t, srv.URL+"/v1/registry/note")
	defer closeResponseBody(t, resp)

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got struct {
		Type   string `json:"type"`
		Schema any    `json:"schema"`
	}
	require.NoError(t, jsonx.UnmarshalRead(resp.Body, &got))
	assert.Equal(t, "note", got.Type)
}
