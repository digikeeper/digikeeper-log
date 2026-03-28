package tests

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"encoding/json/v2"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	sloghttp "github.com/samber/slog-http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gitrus/digikeeper-log/internal/domain/command"
	"github.com/gitrus/digikeeper-log/internal/domain/query"
	"github.com/gitrus/digikeeper-log/internal/httpapi"
	apicmd "github.com/gitrus/digikeeper-log/internal/httpapi/command"
	apiqry "github.com/gitrus/digikeeper-log/internal/httpapi/query"
	apireg "github.com/gitrus/digikeeper-log/internal/httpapi/registry"
	store "github.com/gitrus/digikeeper-log/internal/infrastructure"
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

func setupTestServer(t *testing.T, clientSources map[string]int) *httptest.Server {
	t.Helper()

	logStore, err := store.NewStore(t.TempDir())
	require.NoError(t, err, "init store")
	t.Cleanup(func() { _ = logStore.Close() })

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	cmdSvc := command.NewService(logStore, logger, clientSources)
	qrySvc := query.NewService(logStore, logStore, logger)

	resolveSrc := httpapi.NewSourceResolver(clientSources)
	cmdHandler := apicmd.NewHandler(cmdSvc, resolveSrc)
	qryHandler := apiqry.NewHandler(qrySvc, resolveSrc)
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

func TestPostEntry(t *testing.T) {
	srv := setupTestServer(t, nil)

	body := `{"type":"note","timestamp":"2026-03-08T10:00:00Z","tags":["work"],"data":{"note":"test"}}`
	// act
	resp, err := http.Post(srv.URL+"/v1/logs", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, "application/vnd.api+json", resp.Header.Get("Content-Type"))

	// assert
	var got singleResponse
	require.NoError(t, json.UnmarshalRead(resp.Body, &got))

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
	srv := setupTestServer(t, nil)

	// act POST
	for _, postBody := range []string{
		`{"type":"note","timestamp":"2026-03-08T14:30:00Z","tags":["fitness","health"],"data":{"exercise":"running"}}`,
		`{"type":"note","timestamp":"2026-03-08T14:29:00Z","tags":["health"],"data":{"exercise":"pre-running"}}`,
	} {
		postResp, err := http.Post(srv.URL+"/v1/logs", "application/json", strings.NewReader(postBody))
		require.NoError(t, err)
		defer postResp.Body.Close()
		require.Equal(t, http.StatusCreated, postResp.StatusCode)

		var postResult singleResponse
		require.NoError(t, json.UnmarshalRead(postResp.Body, &postResult))
	}

	// act GET
	getResp, err := http.Get(srv.URL + "/v1/logs?tag=fitness")
	require.NoError(t, err)
	defer getResp.Body.Close()

	require.Equal(t, http.StatusOK, getResp.StatusCode)
	assert.Equal(t, "application/vnd.api+json", getResp.Header.Get("Content-Type"))

	// assert
	var getResult listResponse
	require.NoError(t, json.UnmarshalRead(getResp.Body, &getResult))

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
	clientSources := map[string]int{"mobile": 1, "web": 2}
	srv := setupTestServer(t, clientSources)

	body := `{"type":"note","timestamp":"2026-03-08T10:00:00Z","tags":["work"],"data":{"note":"test"}}`

	tests := []struct {
		name       string
		clientID   string
		wantSource string
	}{
		{"known client", "mobile", "mobile"},
		{"another known client", "web", "web"},
		{"unknown client", "desktop", ""},
		{"empty client", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, srv.URL+"/v1/logs", strings.NewReader(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			if tc.clientID != "" {
				req.Header.Set("X-Client-Id", tc.clientID)
			}

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, http.StatusCreated, resp.StatusCode)

			var got singleResponse
			require.NoError(t, json.UnmarshalRead(resp.Body, &got))
			assert.Equal(t, tc.wantSource, got.Data.Attributes.Meta.Source)
		})
	}
}

func TestAppendPassesRequestID(t *testing.T) {
	srv := setupTestServer(t, nil)

	body := `{"type":"note","timestamp":"2026-03-08T10:00:00Z","tags":["work"],"data":{"note":"test"}}`
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/v1/logs", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", "test-req-123")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var got singleResponse
	require.NoError(t, json.UnmarshalRead(resp.Body, &got))
	assert.Equal(t, "test-req-123", got.Data.Attributes.RequestID)
}

func TestAppendGeneratesRequestIDWhenMissing(t *testing.T) {
	srv := setupTestServer(t, nil)

	body := `{"type":"note","timestamp":"2026-03-08T10:00:00Z","tags":["work"],"data":{"note":"test"}}`
	resp, err := http.Post(srv.URL+"/v1/logs", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var got singleResponse
	require.NoError(t, json.UnmarshalRead(resp.Body, &got))
	assert.NotEmpty(t, got.Data.Attributes.RequestID)
}

func TestRegistryListSchemas(t *testing.T) {
	srv := setupTestServer(t, nil)

	resp, err := http.Get(srv.URL + "/v1/registry")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got struct {
		Schemas []struct {
			Type   string `json:"type"`
			Schema any    `json:"schema"`
		} `json:"schemas"`
	}
	require.NoError(t, json.UnmarshalRead(resp.Body, &got))
	require.Len(t, got.Schemas, 1)
	assert.Equal(t, "note", got.Schemas[0].Type)
}

func TestRegistryGetSchema(t *testing.T) {
	srv := setupTestServer(t, nil)

	resp, err := http.Get(srv.URL + "/v1/registry/note")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got struct {
		Type   string `json:"type"`
		Schema any    `json:"schema"`
	}
	require.NoError(t, json.UnmarshalRead(resp.Body, &got))
	assert.Equal(t, "note", got.Type)
}
