package main

import (
	"sync"
)

// knownCapsCache memoizes the server's capability catalog for the
// process lifetime. The dev/deploy/run watchers may call into
// validate many times; we don't want a network hop per file save.
// A new CLI invocation re-fetches, which is exactly the granularity
// we want — restart picks up server-side additions.
var (
	knownCapsOnce sync.Once
	knownCapsMap  map[string]bool // nil → validate.Project falls back to FallbackCapabilities
)

// loadKnownCapabilities returns the server's capability registry as
// a name set, or nil if no API key is configured / the fetch fails.
// Callers pass the result to validate.Project as the second arg.
//
// Best-effort by contract: any failure (no client, server pre-
// /capabilities, network down, 4xx) returns nil so the offline
// FallbackCapabilities list kicks in. The CLI's primary job is to
// run; a richer linter is a non-essential upgrade.
func loadKnownCapabilities() map[string]bool {
	knownCapsOnce.Do(func() {
		if client == nil {
			return
		}
		caps, err := client.ListCapabilities(globalCtx())
		if err != nil {
			// Soft-fail. Older servers return 404 here, offline returns
			// a dial error — either way the offline list still catches
			// the common typos.
			return
		}
		m := make(map[string]bool, len(caps))
		for _, c := range caps {
			m[c.Name] = true
		}
		knownCapsMap = m
	})
	return knownCapsMap
}
