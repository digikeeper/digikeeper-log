package timefmt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatUsesFixedUTCMilliseconds(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 3, 8, 10, 0, 0, 123456789, time.FixedZone("test", 2*60*60))

	assert.Equal(t, "2026-03-08T08:00:00.123Z", Format(ts))
}

func TestParseAcceptsRFC3339NanoAndNormalizesToMilliseconds(t *testing.T) {
	t.Parallel()

	ts, err := Parse("2026-03-08T10:00:00.123456789Z")
	require.NoError(t, err)

	assert.Equal(t, "2026-03-08T10:00:00.123Z", ts.Format(RFC3339Milli))
}

func TestParseAcceptsWholeSecondRFC3339(t *testing.T) {
	t.Parallel()

	ts, err := Parse("2026-03-08T10:00:00Z")
	require.NoError(t, err)

	assert.Equal(t, "2026-03-08T10:00:00.000Z", Format(ts))
}
