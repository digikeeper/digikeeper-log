// Package chain provides a convenient way to chain http handlers.
//
// Vendored from github.com/justinas/alice (MIT License).
// Copyright (c) 2014 Justinas Stankevicius.
package chain

import "net/http"

// Constructor is a function that wraps an http.Handler with middleware.
type Constructor func(http.Handler) http.Handler

// Chain acts as a list of http.Handler constructors.
// Chain is effectively immutable:
// once created, it will always hold
// the same set of constructors in the same order.
type Chain struct {
	constructors []Constructor
}

// New creates a new chain,
// memorizing the given list of middleware constructors.
// New serves no other function,
// constructors are only called upon a call to Then().
func New(constructors ...Constructor) Chain {
	return Chain{append(([]Constructor)(nil), constructors...)}
}

// Then chains the middleware and returns the final http.Handler.
//
//	New(m1, m2, m3).Then(h)
//
// is equivalent to:
//
//	m1(m2(m3(h)))
//
// When the request comes in, it will be passed to m1, then m2, then m3
// and finally, the given handler
// (assuming every middleware calls the following one).
//
// Then() treats nil as http.DefaultServeMux.
func (c Chain) Then(h http.Handler) http.Handler {
	if h == nil {
		h = http.DefaultServeMux
	}

	for i := range c.constructors {
		h = c.constructors[len(c.constructors)-1-i](h)
	}

	return h
}

// ThenFunc works identically to Then, but takes
// a HandlerFunc instead of a Handler.
func (c Chain) ThenFunc(fn http.HandlerFunc) http.Handler {
	if fn == nil {
		return c.Then(nil)
	}
	return c.Then(fn)
}

// Append extends a chain, adding the specified constructors
// as the last ones in the request flow.
//
// Append returns a new chain, leaving the original one untouched.
func (c Chain) Append(constructors ...Constructor) Chain {
	newCons := make([]Constructor, 0, len(c.constructors)+len(constructors))
	newCons = append(newCons, c.constructors...)
	newCons = append(newCons, constructors...)

	return Chain{newCons}
}
