package execs

import (
	"fmt"
	"regexp"
	"sync"
)

// LazyRegexp provides thread-safe lazy compilation of a regular expression.
// The pattern is compiled at most once, even when accessed concurrently.
type LazyRegexp struct {
	err     error
	regex   *regexp.Regexp
	pattern string
	once    sync.Once
}

// NewLazyRegexp creates a new LazyRegexp that will compile the given pattern
// when Get() is first called.
func NewLazyRegexp(pattern string) *LazyRegexp {
	return &LazyRegexp{
		pattern: pattern,
	}
}

// Get returns the compiled regular expression, compiling it on the first call.
// Subsequent calls return the cached result.
func (lr *LazyRegexp) Get() (*regexp.Regexp, error) {
	lr.once.Do(func() {
		if lr.pattern == "" {
			return
		}

		lr.regex, lr.err = regexp.Compile(lr.pattern)
		if lr.err != nil {
			lr.err = fmt.Errorf("compile pattern %q: %w", lr.pattern, lr.err)
		}
	})

	return lr.regex, lr.err
}

// IsCompiled returns true if the pattern has been compiled.
func (lr *LazyRegexp) IsCompiled() bool {
	return lr.regex != nil || lr.err != nil
}
