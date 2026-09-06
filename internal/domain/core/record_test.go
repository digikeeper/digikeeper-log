package core

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordMeta_UnmarshalLegacySchemaVersion(t *testing.T) {
	t.Parallel()

	var meta RecordMeta
	require.NoError(t, json.Unmarshal([]byte(`{"v":1,"r":2,"s":3}`), &meta))
	assert.Equal(t, 1, meta.SchemaVersion)
	assert.Equal(t, 2, meta.Revision)
	assert.Equal(t, 3, meta.Src)

	data, err := json.Marshal(meta)
	require.NoError(t, err)
	var stored map[string]any
	require.NoError(t, json.Unmarshal(data, &stored))
	assert.Equal(t, float64(1), stored["sv"])
	assert.NotContains(t, stored, "v")
}
