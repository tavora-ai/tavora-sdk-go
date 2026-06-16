// Package crud holds the HTTP handlers that translate
// json-server-style request shapes into store operations. Forked from
// `github.com/jryannel/json-server-go`'s handlers/rest.go.
//
// Query syntax for List:
//   ?_page=2&_limit=10        // pagination (1-indexed)
//   ?_start=20&_end=30        // alternative slice
//   ?_sort=title,createdAt&_order=asc,desc
//   ?q=needle                 // case-insensitive substring across string fields
//   ?<field>=<value>          // equality filter on field
package crud

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/tavora-ai/tavora-sdk-go/cli/internal/fakeback/store"
)

const maxBodySize = 1 << 20 // 1 MiB

// REST mounts CRUD endpoints for json-server-style collections.
type REST struct {
	Store *store.Store
}

// NewREST returns a handler bound to the supplied store.
func NewREST(s *store.Store) *REST { return &REST{Store: s} }

// Resources returns the index of available resources at "/".
func (h *REST) Resources(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"resources": h.Store.Resources(),
	})
}

// List returns a filtered/sorted/paged view of a collection, or the
// singleton value if the resource isn't an array.
func (h *REST) List(w http.ResponseWriter, r *http.Request) {
	resource := chi.URLParam(r, "resource")
	isColl, exists := h.Store.IsCollection(resource)
	if !exists {
		writeError(w, http.StatusNotFound, "unknown resource: "+resource)
		return
	}
	if !isColl {
		v, _ := h.Store.Singleton(resource)
		writeJSON(w, http.StatusOK, v)
		return
	}
	q := parseListQuery(r)
	res, err := h.Store.List(resource, q)
	if err != nil {
		mapStoreError(w, err)
		return
	}
	w.Header().Set("X-Total-Count", strconv.Itoa(res.Total))
	if q.Limit > 0 {
		w.Header().Set("X-Page", strconv.Itoa(maxInt(q.Page, 1)))
		w.Header().Set("X-Limit", strconv.Itoa(q.Limit))
	}
	writeJSON(w, http.StatusOK, res.Items)
}

// Get returns one record by id.
func (h *REST) Get(w http.ResponseWriter, r *http.Request) {
	resource := chi.URLParam(r, "resource")
	id := chi.URLParam(r, "id")
	rec, err := h.Store.Get(resource, id)
	if err != nil {
		mapStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

// Create POSTs a new record to the collection.
func (h *REST) Create(w http.ResponseWriter, r *http.Request) {
	resource := chi.URLParam(r, "resource")
	rec, err := readRecord(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	created, err := h.Store.Create(resource, rec)
	if err != nil {
		mapStoreError(w, err)
		return
	}
	w.Header().Set("Location", fmt.Sprintf("/%s/%v", resource, created["id"]))
	writeJSON(w, http.StatusCreated, created)
}

// Replace overwrites a record (PUT).
func (h *REST) Replace(w http.ResponseWriter, r *http.Request) {
	resource := chi.URLParam(r, "resource")
	id := chi.URLParam(r, "id")
	rec, err := readRecord(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated, err := h.Store.Replace(resource, id, rec)
	if err != nil {
		mapStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// Patch merges fields into a record (PATCH).
func (h *REST) Patch(w http.ResponseWriter, r *http.Request) {
	resource := chi.URLParam(r, "resource")
	id := chi.URLParam(r, "id")
	rec, err := readRecord(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated, err := h.Store.Patch(resource, id, rec)
	if err != nil {
		mapStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// Delete removes a record by id.
func (h *REST) Delete(w http.ResponseWriter, r *http.Request) {
	resource := chi.URLParam(r, "resource")
	id := chi.URLParam(r, "id")
	if err := h.Store.Delete(resource, id); err != nil {
		mapStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- helpers ----

var reservedQueryKeys = map[string]struct{}{
	"_page": {}, "_limit": {}, "_sort": {}, "_order": {},
	"_start": {}, "_end": {}, "q": {},
}

func parseListQuery(r *http.Request) store.ListQuery {
	values := r.URL.Query()
	q := store.ListQuery{Filters: map[string]string{}}
	q.Page, _ = strconv.Atoi(values.Get("_page"))
	q.Limit, _ = strconv.Atoi(values.Get("_limit"))
	q.Start, _ = strconv.Atoi(values.Get("_start"))
	q.End, _ = strconv.Atoi(values.Get("_end"))
	q.Q = values.Get("q")
	if s := values.Get("_sort"); s != "" {
		q.Sort = strings.Split(s, ",")
	}
	if o := values.Get("_order"); o != "" {
		q.Order = strings.Split(o, ",")
	}
	for k, vs := range values {
		if _, reserved := reservedQueryKeys[k]; reserved {
			continue
		}
		if len(vs) > 0 {
			q.Filters[k] = vs[0]
		}
	}
	return q
}

func readRecord(r *http.Request) (store.Record, error) {
	limited := http.MaxBytesReader(nil, r.Body, maxBodySize)
	defer limited.Close()
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if len(raw) == 0 {
		return store.Record{}, nil
	}
	var rec store.Record
	if err := json.Unmarshal(raw, &rec); err != nil {
		return nil, fmt.Errorf("invalid json body: %w", err)
	}
	return rec, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{
		"error":  msg,
		"status": status,
	})
}

func mapStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, store.ErrBadResource):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, store.ErrNotCollection):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
