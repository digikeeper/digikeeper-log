package appmetric

import (
	"expvar"
	"time"
)

var (
	RecordsAppended = expvar.NewInt("records_appended")

	sqliteIndexLatencyMs = expvar.NewInt("sqlite_index_latency_ms")
)

func RecordIndexLatency(d time.Duration) {
	sqliteIndexLatencyMs.Set(d.Milliseconds())
}
