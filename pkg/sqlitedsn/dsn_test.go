package sqlitedsn

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFile(t *testing.T) {
	t.Parallel()

	dsn := File(
		"/tmp/index.db",
		Pragma(PragmaJournalMode, "WAL"),
		Pragma(PragmaBusyTimeout, 5000),
	)

	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	assert.Equal(t, "file", parsed.Scheme)
	assert.Equal(t, "/tmp/index.db", parsed.Path)
	assert.Equal(t, []string{"journal_mode(WAL)", "busy_timeout(5000)"}, parsed.Query()[string(ParamPragma)])
}

func TestFileWithoutOptions(t *testing.T) {
	t.Parallel()

	parsed, err := url.Parse(File("/tmp/index.db"))
	require.NoError(t, err)
	assert.Equal(t, "file", parsed.Scheme)
	assert.Equal(t, "/tmp/index.db", parsed.Path)
	assert.Empty(t, parsed.RawQuery)
}

func TestParamValue(t *testing.T) {
	t.Parallel()

	parsed, err := url.Parse(File("/tmp/index.db", Pragma(PragmaForeignKeys, 1)))
	require.NoError(t, err)
	assert.Equal(t, []string{"foreign_keys(1)"}, parsed.Query()[string(ParamPragma)])
}

func TestParamValueAllowsRawQueryParam(t *testing.T) {
	t.Parallel()

	parsed, err := url.Parse(File("/tmp/index.db", ParamValue(ParamPragma, "foreign_keys(1)")))
	require.NoError(t, err)
	assert.Equal(t, []string{"foreign_keys(1)"}, parsed.Query()[string(ParamPragma)])
}
