package chain

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tagMiddleware(tag string) Constructor {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("X-Order", tag)
			next.ServeHTTP(w, r)
		})
	}
}

func finalHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Order", "handler")
		w.WriteHeader(http.StatusOK)
	})
}

func serveAndGetOrder(t *testing.T, h http.Handler) []string {
	t.Helper()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	return rec.Header().Values("X-Order")
}

func TestThen(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		constructors []Constructor
		want         []string
	}{
		"no middleware": {
			constructors: nil,
			want:         []string{"handler"},
		},
		"single middleware": {
			constructors: []Constructor{tagMiddleware("m1")},
			want:         []string{"m1", "handler"},
		},
		"three middleware in order": {
			constructors: []Constructor{tagMiddleware("m1"), tagMiddleware("m2"), tagMiddleware("m3")},
			want:         []string{"m1", "m2", "m3", "handler"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := serveAndGetOrder(t, New(tt.constructors...).Then(finalHandler()))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestThen_NilHandler(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		handler func(Chain) http.Handler
	}{
		"Then(nil)":     {handler: func(c Chain) http.Handler { return c.Then(nil) }},
		"ThenFunc(nil)": {handler: func(c Chain) http.Handler { return c.ThenFunc(nil) }},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := tt.handler(New())
			require.NotNil(t, h, "should fall back to http.DefaultServeMux")
		})
	}
}

func TestThenFunc(t *testing.T) {
	t.Parallel()

	called := false
	h := New(tagMiddleware("m1")).ThenFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Header().Add("X-Order", "fn")
	})

	got := serveAndGetOrder(t, h)
	assert.True(t, called)
	assert.Equal(t, []string{"m1", "fn"}, got)
}

func TestAppend(t *testing.T) {
	t.Parallel()

	base := New(tagMiddleware("m1"))
	extended := base.Append(tagMiddleware("m2"), tagMiddleware("m3"))

	t.Run("original chain unchanged", func(t *testing.T) {
		t.Parallel()
		got := serveAndGetOrder(t, base.Then(finalHandler()))
		assert.Equal(t, []string{"m1", "handler"}, got)
	})

	t.Run("extended chain has all", func(t *testing.T) {
		t.Parallel()
		got := serveAndGetOrder(t, extended.Then(finalHandler()))
		assert.Equal(t, []string{"m1", "m2", "m3", "handler"}, got)
	})
}

func TestChain_Reuse(t *testing.T) {
	t.Parallel()

	c := New(tagMiddleware("m1"))

	tests := map[string]struct {
		tag  string
		want []string
	}{
		"first handler":  {tag: "a", want: []string{"m1", "a"}},
		"second handler": {tag: "b", want: []string{"m1", "b"}},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tag := tt.tag
			h := c.ThenFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Add("X-Order", tag)
			})
			got := serveAndGetOrder(t, h)
			assert.Equal(t, tt.want, got)
		})
	}
}
