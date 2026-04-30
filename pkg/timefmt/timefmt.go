package timefmt

import "time"

// RFC3339Milli is a fixed-width UTC millisecond RFC3339 layout.
const RFC3339Milli = "2006-01-02T15:04:05.000Z"

// Normalize converts t to UTC and truncates it to millisecond precision.
func Normalize(t time.Time) time.Time {
	return t.UTC().Truncate(time.Millisecond)
}

// Format returns t as fixed-width UTC milliseconds.
func Format(t time.Time) string {
	return Normalize(t).Format(RFC3339Milli)
}

// Parse accepts RFC3339 timestamps and normalizes the result to UTC milliseconds.
func Parse(value string) (time.Time, error) {
	t, err := time.Parse(RFC3339Milli, value)
	if err == nil {
		return t, nil
	}
	t, err = time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, err
	}
	return Normalize(t), nil
}
