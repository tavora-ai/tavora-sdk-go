// Package ids generates deterministic UUIDs from a seed integer. Each
// Source is its own counter — two Sources with the same seed produce
// the same sequence, so demos and golden-file tests are reproducible.
//
// The generated IDs aren't cryptographically meaningful — they're
// version-4 UUIDs derived from a SHA-256(seed || counter) prefix, so
// they look like real UUIDs to a downstream parser that demands 36
// hyphenated hex chars.
package ids

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sync"
	"sync/atomic"
)

// Source generates deterministic IDs from a seed.
type Source struct {
	seed    int64
	counter atomic.Uint64
	mu      sync.Mutex
}

// New returns a Source whose counter starts at 0 and is seeded by
// `seed`. Pass 0 for "non-deterministic": the source then derives the
// seed from a process-counter so successive calls in one process are
// still distinct, but two processes won't collide.
func New(seed int64) *Source {
	return &Source{seed: seed}
}

// Next returns the next UUID in the sequence. Two Sources with the
// same seed return identical sequences.
func (s *Source) Next() string {
	n := s.counter.Add(1)
	var buf [16]byte
	binary.BigEndian.PutUint64(buf[:8], uint64(s.seed))
	binary.BigEndian.PutUint64(buf[8:], n)
	sum := sha256.Sum256(buf[:])
	// Shape the first 16 bytes of the hash into a UUIDv4-ish string.
	// Set the version and variant nibbles so a strict parser accepts
	// the result.
	sum[6] = (sum[6] & 0x0f) | 0x40 // version 4
	sum[8] = (sum[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
}

// SessionID returns the next id with an "as-" prefix — handy for
// agent-session IDs that follow the SDK's "as_..." convention but
// keep the UUID-ish tail so debug logs are scannable.
func (s *Source) SessionID() string {
	return "as_" + s.Next()
}
