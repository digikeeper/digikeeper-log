package tests

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gitrus/digikeeper-log/internal/jsonx"
)

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
