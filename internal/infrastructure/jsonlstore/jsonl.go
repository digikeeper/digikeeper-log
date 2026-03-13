package jsonlstore

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	json "encoding/json/v2"

	"github.com/tidwall/gjson"

	"github.com/gitrus/digikeeper-log/internal/domain/errs"
	"github.com/gitrus/digikeeper-log/internal/domain/model"
)

const maxEntrySizeBytes = 10 * 1024 * 1024 // 10 MiB

// JSONLWriter manages per-day JSONL log files for appending and reading.
//
// Concurrency model:
//   - files (sync.Map) provides lock-free lookup of open file descriptors.
//   - O_APPEND guarantees atomic positioning for writes.
//   - dirMu serializes directory creation only when a new year-directory is needed.
type JSONLWriter struct {
	dir     string
	logType string
	files   sync.Map   // relPath → *logFD
	dirMu   sync.Mutex // held only during mkdir+open on cache miss
}

type logFD struct {
	fd      *os.File
	relPath string
}

type ReadFilters struct {
	From time.Time
	To   time.Time
	Tags map[string]struct{}
}

type ReadOption func(*ReadFilters)

func WithTimeRange(from, to time.Time) ReadOption {
	return func(f *ReadFilters) { f.From = from; f.To = to }
}

func WithTags(tags ...string) ReadOption {
	return func(f *ReadFilters) {
		f.Tags = make(map[string]struct{}, len(tags))
		for _, t := range tags {
			f.Tags[t] = struct{}{}
		}
	}
}

func NewJSONLWriter(dir, logType string) *JSONLWriter {
	return &JSONLWriter{
		dir:     dir,
		logType: logType,
	}
}

func (w *JSONLWriter) Append(entry model.Entry) (string, error) {
	line, err := json.Marshal(entry)
	if err != nil {
		return "", fmt.Errorf("jsonl: marshal: %w, %w", err, errs.StorageCommonError)
	}
	line = append(line, '\n')

	relPath := w.buildRelPath(entry.Timestamp)
	lfd, err := w.getOrCreate(relPath)
	if err != nil {
		return "", err
	}

	if _, err := lfd.fd.Write(line); err != nil {
		return "", fmt.Errorf("jsonl: write: %w, %w", err, errs.StorageCommonError)
	}
	return relPath, nil
}

// getOrCreate returns the logFD for relPath, creating the file if needed
func (w *JSONLWriter) getOrCreate(relPath string) (*logFD, error) {
	if v, ok := w.files.Load(relPath); ok {
		return v.(*logFD), nil
	}

	w.dirMu.Lock()
	defer w.dirMu.Unlock()

	fpath := filepath.Join(w.dir, relPath)
	dir := filepath.Dir(fpath)
	if _, err := os.Stat(dir); err != nil {
		err := os.MkdirAll(dir, 0o755)
		if err != nil {
			return nil, fmt.Errorf("jsonl: mkdir %s: %w, %w", dir, err, errs.StorageCommonError)
		}
	}

	f, err := os.OpenFile(fpath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("jsonl: open %s: %w, %w", fpath, err, errs.StorageCommonError)
	}

	lfd := &logFD{fd: f, relPath: relPath}
	if actual, loaded := w.files.LoadOrStore(relPath, lfd); loaded {
		_ = f.Close()
		return actual.(*logFD), nil
	}
	return lfd, nil
}

func (w *JSONLWriter) Read(relPath string, opts ...ReadOption) ([]model.Entry, error) {
	var filters ReadFilters
	for _, o := range opts {
		o(&filters)
	}
	hasFilters := len(filters.Tags) > 0 || !filters.From.IsZero() || !filters.To.IsZero()

	fpath := filepath.Join(w.dir, relPath)
	f, err := os.Open(fpath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("jsonl: open %s: %w", relPath, err)
	}
	defer func() { _ = f.Close() }()

	var entries []model.Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, bufio.MaxScanTokenSize), maxEntrySizeBytes)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		if hasFilters && !matchFilters(line, &filters) {
			continue
		}
		var e model.Entry
		if err := json.Unmarshal(line, &e); err != nil {
			return nil, fmt.Errorf("jsonl: unmarshal in %s: %w", relPath, err)
		}
		entries = append(entries, e)
	}
	return entries, sc.Err()
}

func matchFilters(line []byte, f *ReadFilters) bool {
	if !f.From.IsZero() || !f.To.IsZero() {
		ts := gjson.GetBytes(line, "timestamp").Time()
		if !f.From.IsZero() && ts.Before(f.From) {
			return false
		}
		if !f.To.IsZero() && ts.After(f.To) {
			return false
		}
	}
	if len(f.Tags) > 0 {
		tagsResult := gjson.GetBytes(line, "tags")
		if !tagsResult.Exists() {
			return false
		}
		matched := false
		tagsResult.ForEach(func(_, v gjson.Result) bool {
			if _, ok := f.Tags[v.String()]; ok {
				matched = true
				return false
			}
			return true
		})
		if !matched {
			return false
		}
	}
	return true
}

// Close closes all open file handles. Call during graceful shutdown.
func (w *JSONLWriter) Close() error {
	var errs []error
	w.files.Range(func(key, value any) bool {
		lfd := value.(*logFD)
		if err := lfd.fd.Close(); err != nil {
			errs = append(errs, err)
		}
		w.files.Delete(key)
		return true
	})
	return errors.Join(errs...)
}

func (w *JSONLWriter) buildRelPath(ts time.Time) string {
	return fmt.Sprintf(
		"%d/%s_%s.jsonl",
		ts.Year(), ts.Format("2006-01-02"), w.logType,
	)
}
