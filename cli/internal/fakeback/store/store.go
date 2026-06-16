// Package store is the in-memory backing for tavora-fake-backend's
// CRUD endpoints. It's forked from `github.com/jryannel/json-server-go`'s
// store package (~600 lines as of fork time) — almost verbatim, with
// the original Config dependency replaced by a small local struct and
// the persistence story simplified for the v0.1 "edit db.json, restart
// to reload" workflow.
//
// Top-level keys in db.json become resources. Array-valued resources
// are collections (CRUD-able); scalar / object values are singletons
// (GET-able). Identity inside a collection is the "id" field; missing
// ids are auto-assigned per the configured IDStrategy.
package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrConflict      = errors.New("conflict")
	ErrBadResource   = errors.New("unknown resource")
	ErrNotCollection = errors.New("resource is not a collection")
)

// Config controls the store. Persist=false (default) keeps mutations
// in memory only — restart the binary and the seed db.json reloads.
// Persist=true writes mutations back to the same file, debounced by
// DebounceWindow (defaults to 500ms).
type Config struct {
	File           string
	IDStrategy     string // "int" or "uuid"
	Persist        bool
	DebounceWindow time.Duration
}

// Record is one row in a collection.
type Record = map[string]any

// Store wraps a JSON file. Goroutine-safe.
type Store struct {
	cfg Config
	log *slog.Logger

	mu   sync.RWMutex
	data map[string]any
	seed []byte // snapshot for Reset()

	dirty bool
	timer *time.Timer
}

// Open reads cfg.File into memory and returns a ready Store.
func Open(cfg Config, log *slog.Logger) (*Store, error) {
	if cfg.IDStrategy == "" {
		cfg.IDStrategy = "int"
	}
	s := &Store{cfg: cfg, log: log}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	path := s.cfg.File
	if path == "" {
		// Empty store — useful for tests that want to drive everything
		// through Create.
		s.data = map[string]any{}
		s.seed = []byte("{}")
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			s.log.Warn("db file does not exist; starting empty", "file", path)
			s.data = map[string]any{}
			s.seed = []byte("{}")
			return nil
		}
		return fmt.Errorf("read db: %w", err)
	}
	if len(raw) == 0 {
		s.data = map[string]any{}
		s.seed = []byte("{}")
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("parse db json: %w", err)
	}
	// Normalize: ensure every record in arrays has an "id" field.
	for k, v := range m {
		if arr, ok := v.([]any); ok {
			for i := range arr {
				if rec, ok := arr[i].(map[string]any); ok {
					if _, has := rec["id"]; !has {
						rec["id"] = s.nextID(arr)
					}
				}
			}
			m[k] = arr
		}
	}
	s.data = m
	s.seed = append([]byte(nil), raw...)
	return nil
}

// Close flushes pending writes when persistence is enabled.
func (s *Store) Close() error {
	s.mu.Lock()
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	dirty := s.dirty
	s.mu.Unlock()
	if dirty {
		return s.persistNow()
	}
	return nil
}

// Resources lists the top-level keys, sorted.
func (s *Store) Resources() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.data))
	for k := range s.data {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// IsCollection reports whether the resource is an array.
func (s *Store) IsCollection(resource string) (isCollection, exists bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[resource]
	if !ok {
		return false, false
	}
	_, isArr := v.([]any)
	return isArr, true
}

// Singleton returns the value of a non-collection resource.
func (s *Store) Singleton(resource string) (any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[resource]
	if !ok {
		return nil, ErrBadResource
	}
	if _, isArr := v.([]any); isArr {
		return nil, fmt.Errorf("%w: %s is a collection", ErrNotCollection, resource)
	}
	return v, nil
}

// ListQuery describes filter/sort/page parameters for List.
type ListQuery struct {
	Filters map[string]string
	Q       string
	Sort    []string
	Order   []string
	Page    int
	Limit   int
	Start   int
	End     int
}

type ListResult struct {
	Items []Record
	Total int
	Page  int
	Limit int
}

// List returns a filtered/sorted/paged view over a collection.
func (s *Store) List(resource string, q ListQuery) (*ListResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	arr, err := s.collection(resource)
	if err != nil {
		return nil, err
	}
	items := make([]Record, 0, len(arr))
	for _, v := range arr {
		if r, ok := v.(map[string]any); ok {
			items = append(items, r)
		}
	}
	items = applyFilters(items, q.Filters, q.Q)
	total := len(items)
	if len(q.Sort) > 0 {
		applySort(items, q.Sort, q.Order)
	}
	items = applyPaging(items, q)
	return &ListResult{Items: items, Total: total, Page: q.Page, Limit: q.Limit}, nil
}

// Get returns a single record by id.
func (s *Store) Get(resource, id string) (Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	arr, err := s.collection(resource)
	if err != nil {
		return nil, err
	}
	for _, v := range arr {
		if r, ok := v.(map[string]any); ok && idEquals(r["id"], id) {
			return r, nil
		}
	}
	return nil, ErrNotFound
}

// Create appends a new record. Missing id is auto-assigned.
func (s *Store) Create(resource string, rec Record) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.data[resource]
	if !ok {
		v = []any{}
		s.data[resource] = v
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotCollection, resource)
	}
	if rec == nil {
		rec = Record{}
	}
	if _, has := rec["id"]; !has {
		rec["id"] = s.nextID(arr)
	} else {
		for _, ex := range arr {
			if r, ok := ex.(map[string]any); ok && idEquals(r["id"], fmt.Sprint(rec["id"])) {
				return nil, ErrConflict
			}
		}
	}
	arr = append(arr, rec)
	s.data[resource] = arr
	s.markDirty()
	return rec, nil
}

// Replace overwrites the whole record (PUT). Keeps id.
func (s *Store) Replace(resource, id string, rec Record) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	arr, err := s.collection(resource)
	if err != nil {
		return nil, err
	}
	for i, v := range arr {
		if r, ok := v.(map[string]any); ok && idEquals(r["id"], id) {
			if rec == nil {
				rec = Record{}
			}
			rec["id"] = r["id"]
			arr[i] = rec
			s.data[resource] = arr
			s.markDirty()
			return rec, nil
		}
	}
	return nil, ErrNotFound
}

// Patch merges fields into the existing record (PATCH).
func (s *Store) Patch(resource, id string, patch Record) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	arr, err := s.collection(resource)
	if err != nil {
		return nil, err
	}
	for _, v := range arr {
		if r, ok := v.(map[string]any); ok && idEquals(r["id"], id) {
			for k, val := range patch {
				if k == "id" {
					continue
				}
				r[k] = val
			}
			s.markDirty()
			return r, nil
		}
	}
	return nil, ErrNotFound
}

// Delete removes a record by id.
func (s *Store) Delete(resource, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	arr, err := s.collection(resource)
	if err != nil {
		return err
	}
	for i, v := range arr {
		if r, ok := v.(map[string]any); ok && idEquals(r["id"], id) {
			arr = append(arr[:i], arr[i+1:]...)
			s.data[resource] = arr
			s.markDirty()
			return nil
		}
	}
	return ErrNotFound
}

// Snapshot returns the current state as pretty JSON.
func (s *Store) Snapshot() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.MarshalIndent(s.data, "", "  ")
}

// Reload re-reads the db.json file from disk and replaces the
// in-memory state. Used by the fsnotify-driven hot-reload path. The
// seed snapshot is also refreshed so a subsequent Reset() returns to
// the most-recent on-disk content, not the original startup state —
// which matches what the skill author expects: "I edited the file,
// reset should put me back to what I just wrote."
func (s *Store) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

// loadLocked replays load() under an already-held write lock.
func (s *Store) loadLocked() error {
	path := s.cfg.File
	if path == "" {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			s.data = map[string]any{}
			s.seed = []byte("{}")
			return nil
		}
		return fmt.Errorf("read db: %w", err)
	}
	if len(raw) == 0 {
		s.data = map[string]any{}
		s.seed = []byte("{}")
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("parse db json: %w", err)
	}
	for k, v := range m {
		if arr, ok := v.([]any); ok {
			for i := range arr {
				if rec, ok := arr[i].(map[string]any); ok {
					if _, has := rec["id"]; !has {
						rec["id"] = s.nextID(arr)
					}
				}
			}
			m[k] = arr
		}
	}
	s.data = m
	s.seed = append([]byte(nil), raw...)
	return nil
}

// Reset restores the seed snapshot captured at Open (or refreshed by
// the most recent Reload).
func (s *Store) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var m map[string]any
	if len(s.seed) > 0 {
		if err := json.Unmarshal(s.seed, &m); err != nil {
			return fmt.Errorf("reset: parse seed: %w", err)
		}
	} else {
		m = map[string]any{}
	}
	s.data = m
	if s.cfg.Persist {
		s.markDirty()
	}
	return nil
}

// ---- internals ----

func (s *Store) collection(resource string) ([]any, error) {
	v, ok := s.data[resource]
	if !ok {
		return nil, ErrBadResource
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotCollection, resource)
	}
	return arr, nil
}

func (s *Store) nextID(arr []any) any {
	if s.cfg.IDStrategy == "uuid" {
		return randomHexID()
	}
	max := 0
	for _, v := range arr {
		if r, ok := v.(map[string]any); ok {
			switch x := r["id"].(type) {
			case float64:
				if int(x) > max {
					max = int(x)
				}
			case int:
				if x > max {
					max = x
				}
			case string:
				if n, err := strconv.Atoi(x); err == nil && n > max {
					max = n
				}
			}
		}
	}
	return float64(max + 1)
}

func randomHexID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b)
}

func idEquals(idVal any, want string) bool {
	switch v := idVal.(type) {
	case string:
		return v == want
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10) == want
		}
		return strconv.FormatFloat(v, 'f', -1, 64) == want
	case int:
		return strconv.Itoa(v) == want
	case int64:
		return strconv.FormatInt(v, 10) == want
	case bool:
		return strconv.FormatBool(v) == want
	default:
		return fmt.Sprint(v) == want
	}
}

func applyFilters(items []Record, filters map[string]string, q string) []Record {
	if len(filters) == 0 && q == "" {
		return items
	}
	out := items[:0]
outer:
	for _, r := range items {
		for k, v := range filters {
			if !fieldEquals(r[k], v) {
				continue outer
			}
		}
		if q != "" && !fullTextMatch(r, q) {
			continue
		}
		out = append(out, r)
	}
	return out
}

func fieldEquals(val any, want string) bool {
	if val == nil {
		return want == "" || want == "null"
	}
	switch v := val.(type) {
	case string:
		return v == want
	case bool:
		return strconv.FormatBool(v) == want
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10) == want
		}
		return strconv.FormatFloat(v, 'f', -1, 64) == want
	default:
		return fmt.Sprint(v) == want
	}
}

func fullTextMatch(r Record, q string) bool {
	q = strings.ToLower(q)
	for _, v := range r {
		if s, ok := v.(string); ok {
			if strings.Contains(strings.ToLower(s), q) {
				return true
			}
		}
	}
	return false
}

func applySort(items []Record, fields, orders []string) {
	sort.SliceStable(items, func(i, j int) bool {
		for idx, f := range fields {
			desc := false
			if idx < len(orders) && strings.EqualFold(orders[idx], "desc") {
				desc = true
			}
			cmp := compareValues(items[i][f], items[j][f])
			if cmp == 0 {
				continue
			}
			if desc {
				return cmp > 0
			}
			return cmp < 0
		}
		return false
	})
}

func compareValues(a, b any) int {
	switch av := a.(type) {
	case float64:
		if bv, ok := b.(float64); ok {
			switch {
			case av < bv:
				return -1
			case av > bv:
				return 1
			default:
				return 0
			}
		}
	case string:
		if bv, ok := b.(string); ok {
			return strings.Compare(av, bv)
		}
	case bool:
		if bv, ok := b.(bool); ok {
			if av == bv {
				return 0
			}
			if !av {
				return -1
			}
			return 1
		}
	}
	return strings.Compare(fmt.Sprint(a), fmt.Sprint(b))
}

func applyPaging(items []Record, q ListQuery) []Record {
	n := len(items)
	if q.Start > 0 || q.End > 0 {
		start := q.Start
		end := q.End
		if end <= 0 || end > n {
			end = n
		}
		if start < 0 {
			start = 0
		}
		if start > end {
			start = end
		}
		return items[start:end]
	}
	if q.Limit > 0 {
		page := q.Page
		if page <= 0 {
			page = 1
		}
		start := (page - 1) * q.Limit
		if start >= n {
			return []Record{}
		}
		end := start + q.Limit
		if end > n {
			end = n
		}
		return items[start:end]
	}
	return items
}

// ---- persistence (opt-in) ----

func (s *Store) markDirty() {
	if !s.cfg.Persist {
		return
	}
	s.dirty = true
	window := s.cfg.DebounceWindow
	if window <= 0 {
		window = 500 * time.Millisecond
	}
	if s.timer != nil {
		s.timer.Reset(window)
		return
	}
	s.timer = time.AfterFunc(window, func() {
		if err := s.persistNow(); err != nil {
			s.log.Error("persist failed", "err", err)
		}
	})
}

func (s *Store) persistNow() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal db: %w", err)
	}
	path := s.cfg.File
	if path == "" {
		return nil
	}
	if dir := filepath.Dir(path); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("write tmp db: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename db: %w", err)
	}
	s.dirty = false
	s.timer = nil
	return nil
}
