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
	domainCandidate "github.com/gitrus/digikeeper-log/internal/domain/command/candidate"
	domainCompaction "github.com/gitrus/digikeeper-log/internal/domain/command/compaction"
	"github.com/gitrus/digikeeper-log/internal/domain/query"
	"github.com/gitrus/digikeeper-log/internal/httpapi"
	apicmd "github.com/gitrus/digikeeper-log/internal/httpapi/command"
	apiqry "github.com/gitrus/digikeeper-log/internal/httpapi/query"
<<<<<<< HEAD
	apireg "github.com/gitrus/digikeeper-log/internal/httpapi/registry"
	"github.com/gitrus/digikeeper-log/internal/infrastructure/candidatestore"
=======
	apisreg "github.com/gitrus/digikeeper-log/internal/httpapi/schemaregistry"
>>>>>>> af9c8c8 (Rename registry to schema registry in handlers and docs)
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

type candidateResponse struct {
	Meta responseMeta            `json:"meta"`
	Data candidateResourceObject `json:"data"`
}

type candidateListResponse struct {
	Meta responseMeta              `json:"meta"`
	Data []candidateResourceObject `json:"data"`
}

type candidateResourceObject struct {
	ID         string         `json:"id"`
	Attributes candidateAttrs `json:"attributes"`
}

type candidateAttrs struct {
	EntryID           string         `json:"entry_id"`
	OriginalTimestamp string         `json:"original_timestamp"`
	Entry             candidateEntry `json:"entry"`
	CreatedAt         string         `json:"created_at"`
	Action            string         `json:"action"`
	ResolvedBy        string         `json:"resolved_by"`
	Reason            string         `json:"reason"`
	ClientID          string         `json:"client_id"`
}

type candidateEntry struct {
	ID   string         `json:"id"`
	Type string         `json:"type"`
	Tags []string       `json:"tags"`
	Data map[string]any `json:"d"`
}

func setupTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	dir := t.TempDir()

	idx, err := index.New(filepath.Join(dir, "index.db"), index.Config{})
	require.NoError(t, err, "init index")
	t.Cleanup(func() { _ = idx.Close() })

	logStore, err := store.NewStore(dir, idx)
	require.NoError(t, err, "init store")
	t.Cleanup(func() { _ = logStore.Close() })

	candidateStore, err := candidatestore.New(dir)
	require.NoError(t, err, "init candidate store")

	qryStore := querystore.NewStore(filepath.Join(dir, "dk_logs"), idx)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	srcRepo := sourcerepo.New()

	cmdSvc := command.NewService(logStore, srcRepo, logger)
	candidateSvc := domainCandidate.NewService(candidateStore, logStore, logger)
	compactionSvc := domainCompaction.NewService(logStore, candidateStore, idx, logger)
	qrySvc := query.NewService(qryStore, qryStore, logger)

	cmdHandler := apicmd.NewHandler(cmdSvc, srcRepo.ResolveName)
	candidateHandler := apicmd.NewCandidateHandler(candidateSvc)
	compactionHandler := apicmd.NewCompactionHandler(compactionSvc)
	qryHandler := apiqry.NewHandler(qrySvc, srcRepo.ResolveName)
	sregHandler, err := apisreg.NewHandler()
	require.NoError(t, err, "init schema registry")

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
		OperationID:   "submit-candidate",
		Method:        http.MethodPost,
		Path:          "/v1/candidates",
		Summary:       "Submit a candidate replacement",
		DefaultStatus: http.StatusCreated,
	}, candidateHandler.SubmitCandidate)
	huma.Register(api, huma.Operation{
		OperationID:   "list-pending-candidates",
		Method:        http.MethodGet,
		Path:          "/v1/candidates/pending",
		Summary:       "List pending candidates",
		DefaultStatus: http.StatusOK,
	}, candidateHandler.ListPendingCandidates)
	huma.Register(api, huma.Operation{
		OperationID:   "resolve-candidates",
		Method:        http.MethodPost,
		Path:          "/v1/candidates/resolve",
		Summary:       "Resolve pending candidates for a partition",
		DefaultStatus: http.StatusOK,
	}, candidateHandler.ResolveCandidates)
	huma.Register(api, huma.Operation{
		OperationID:   "compact-partition",
		Method:        http.MethodPost,
		Path:          "/v1/compaction",
		Summary:       "Compact applied candidates into a log partition",
		DefaultStatus: http.StatusOK,
	}, compactionHandler.CompactPartition)
	huma.Register(api, huma.Operation{
		OperationID:   "list-schemas",
		Method:        http.MethodGet,
		Path:          "/v1/registry",
		Summary:       "List all entry type schemas",
		DefaultStatus: http.StatusOK,
	}, sregHandler.ListSchemas)
	huma.Register(api, huma.Operation{
		OperationID:   "get-schema",
		Method:        http.MethodGet,
		Path:          "/v1/registry/{type}",
		Summary:       "Get schema for an entry type",
		DefaultStatus: http.StatusOK,
	}, sregHandler.GetSchema)

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

func postJSONWithHeaders(t *testing.T, url, body string, headers map[string]string) *http.Response {
	t.Helper()
	req := newTestRequest(t, http.MethodPost, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
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

func TestCandidateResolveAndCompactFlow(t *testing.T) {
	srv := setupTestServer(t)

	appendResp := postJSON(t, srv.URL+"/v1/logs",
		`{"type":"note","timestamp":"2026-03-08T10:00:00Z","tags":["work"],"data":{"note":"original"}}`)
	defer closeResponseBody(t, appendResp)
	require.Equal(t, http.StatusCreated, appendResp.StatusCode)
	var appended singleResponse
	require.NoError(t, jsonx.UnmarshalRead(appendResp.Body, &appended))

	submitBody := `{"entry_id":"` + appended.Data.ID + `","original_timestamp":"2026-03-08T10:00:00Z","type":"note","tags":["corrected"],"data":{"note":"corrected text"}}`
	submitResp := postJSONWithHeaders(t, srv.URL+"/v1/candidates", submitBody, map[string]string{
		"X-Client-Id": "mobile",
	})
	defer closeResponseBody(t, submitResp)
	require.Equal(t, http.StatusCreated, submitResp.StatusCode)
	var submitted candidateResponse
	require.NoError(t, jsonx.UnmarshalRead(submitResp.Body, &submitted))
	assert.Equal(t, "candidates", submitted.Meta.Type)
	assert.NotEmpty(t, submitted.Data.ID)
	assert.Equal(t, appended.Data.ID, submitted.Data.Attributes.EntryID)
	assert.Equal(t, "corrected text", submitted.Data.Attributes.Entry.Data["note"])

	batchResp := getURL(t, srv.URL+"/v1/candidates/pending?partition=2026-03-08")
	defer closeResponseBody(t, batchResp)
	require.Equal(t, http.StatusOK, batchResp.StatusCode)
	var batch candidateListResponse
	require.NoError(t, jsonx.UnmarshalRead(batchResp.Body, &batch))
	require.Len(t, batch.Data, 1)
	assert.Equal(t, submitted.Data.ID, batch.Data[0].ID)
	assert.Equal(t, appended.Data.ID, batch.Data[0].Attributes.EntryID)

	resolveBody := `{"partition":"2026-03-08","resolutions":[{"candidate_id":"` + batch.Data[0].ID + `","action":"apply","reason":"best correction"}]}`
	resolveResp := postJSONWithHeaders(t, srv.URL+"/v1/candidates/resolve", resolveBody, map[string]string{
		"X-Resolved-By": "tester",
	})
	defer closeResponseBody(t, resolveResp)
	require.Equal(t, http.StatusOK, resolveResp.StatusCode)
	var resolved candidateListResponse
	require.NoError(t, jsonx.UnmarshalRead(resolveResp.Body, &resolved))
	require.Len(t, resolved.Data, 1)
	assert.Equal(t, "apply", resolved.Data[0].Attributes.Action)
	assert.Equal(t, "tester", resolved.Data[0].Attributes.ResolvedBy)

	compactResp := postJSON(t, srv.URL+"/v1/compaction", `{"partition":"2026-03-08"}`)
	defer closeResponseBody(t, compactResp)
	require.Equal(t, http.StatusOK, compactResp.StatusCode)

	queryResp := getURL(t, srv.URL+"/v1/logs?tag=corrected")
	defer closeResponseBody(t, queryResp)
	require.Equal(t, http.StatusOK, queryResp.StatusCode)
	var queried listResponse
	require.NoError(t, jsonx.UnmarshalRead(queryResp.Body, &queried))
	require.Len(t, queried.Data, 1)
	assert.Equal(t, appended.Data.ID, queried.Data[0].ID)
	assert.Equal(t, "corrected text", queried.Data[0].Attributes.Data["note"])

	compactAgainResp := postJSON(t, srv.URL+"/v1/compaction", `{"partition":"2026-03-08"}`)
	defer closeResponseBody(t, compactAgainResp)
	require.Equal(t, http.StatusOK, compactAgainResp.StatusCode)
}

func TestResolveCandidatesValidationErrors(t *testing.T) {
	srv := setupTestServer(t)

	resp := postJSONWithHeaders(t, srv.URL+"/v1/candidates/resolve", `{"resolutions":[]}`, map[string]string{
		"X-Resolved-By": "tester",
	})
	defer closeResponseBody(t, resp)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

func TestListPendingCandidatesValidationErrors(t *testing.T) {
	srv := setupTestServer(t)

	resp := getURL(t, srv.URL+"/v1/candidates/pending")
	defer closeResponseBody(t, resp)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

func TestResolveCandidatesIncompleteResolution(t *testing.T) {
	srv := setupTestServer(t)
	entryID := appendTestEntry(t, srv)
	candidateA := submitTestCandidate(t, srv, entryID, "a", []string{"a"})
	_ = submitTestCandidate(t, srv, entryID, "b", []string{"b"})

	resolveBody := `{"partition":"2026-03-08","resolutions":[{"candidate_id":"` + candidateA + `","action":"deny"}]}`
	resp := postJSONWithHeaders(t, srv.URL+"/v1/candidates/resolve", resolveBody, map[string]string{
		"X-Resolved-By": "tester",
	})
	defer closeResponseBody(t, resp)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

func TestResolveCandidatesRejectsMultipleApplyForEntry(t *testing.T) {
	srv := setupTestServer(t)
	entryID := appendTestEntry(t, srv)
	candidateA := submitTestCandidate(t, srv, entryID, "a", []string{"a"})
	candidateB := submitTestCandidate(t, srv, entryID, "b", []string{"b"})

	resolveBody := `{"partition":"2026-03-08","resolutions":[{"candidate_id":"` + candidateA + `","action":"apply"},{"candidate_id":"` + candidateB + `","action":"apply"}]}`
	resp := postJSONWithHeaders(t, srv.URL+"/v1/candidates/resolve", resolveBody, map[string]string{
		"X-Resolved-By": "tester",
	})
	defer closeResponseBody(t, resp)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

func TestSubmitCandidateForMissingEntry(t *testing.T) {
	srv := setupTestServer(t)

	resp := postJSON(t, srv.URL+"/v1/candidates",
		`{"entry_id":"missing","original_timestamp":"2026-03-08T10:00:00Z","type":"note","tags":["corrected"],"data":{"note":"corrected"}}`)
	defer closeResponseBody(t, resp)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
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

func TestSchemaRegistryListSchemas(t *testing.T) {
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

func TestSchemaRegistryGetSchema(t *testing.T) {
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

func appendTestEntry(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	resp := postJSON(t, srv.URL+"/v1/logs",
		`{"type":"note","timestamp":"2026-03-08T10:00:00Z","tags":["work"],"data":{"note":"original"}}`)
	defer closeResponseBody(t, resp)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var appended singleResponse
	require.NoError(t, jsonx.UnmarshalRead(resp.Body, &appended))
	return appended.Data.ID
}

func submitTestCandidate(t *testing.T, srv *httptest.Server, entryID, note string, tags []string) string {
	t.Helper()
	body := `{"entry_id":"` + entryID + `","original_timestamp":"2026-03-08T10:00:00Z","type":"note","tags":["` +
		strings.Join(tags, `","`) + `"],"data":{"note":"` + note + `"}}`
	resp := postJSON(t, srv.URL+"/v1/candidates", body)
	defer closeResponseBody(t, resp)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var submitted candidateResponse
	require.NoError(t, jsonx.UnmarshalRead(resp.Body, &submitted))
	return submitted.Data.ID
}
