package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type API struct {
	store *Store
	cache *Cache
}

func NewAPI(store *Store, cache *Cache) *API { return &API{store: store, cache: cache} }

// Routes wires the API and serves the embedded UI at /.
func (a *API) Routes(web fs.FS) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.healthz)
	mux.HandleFunc("GET /readyz", a.readyz)
	mux.HandleFunc("GET /api/services", a.listServices)
	mux.HandleFunc("POST /api/services", a.createService)
	mux.HandleFunc("GET /api/services/{id}", a.getService)
	mux.HandleFunc("GET /api/services/{id}/checks", a.listChecks)
	mux.HandleFunc("GET /api/services/{id}/status", a.getStatus)
	mux.HandleFunc("DELETE /api/services/{id}", a.deleteService)
	mux.Handle("GET /", http.FileServerFS(web))
	return logRequests(mux)
}

func (a *API) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	checks := map[string]string{"postgres": "ok", "redis": "ok"}
	code := http.StatusOK
	if err := a.store.Ping(ctx); err != nil {
		slog.Warn("readyz: postgres", "err", err)
		checks["postgres"], code = "down", http.StatusServiceUnavailable
	}
	if err := a.cache.Ping(ctx); err != nil {
		slog.Warn("readyz: redis", "err", err)
		checks["redis"], code = "down", http.StatusServiceUnavailable
	}

	status := "ready"
	if code != http.StatusOK {
		status = "not ready"
	}
	writeJSON(w, code, map[string]any{"status": status, "checks": checks})
}

// listServices returns every service with its cached status (one Redis MGET).
func (a *API) listServices(w http.ResponseWriter, r *http.Request) {
	svcs, err := a.store.ListServices(r.Context())
	if err != nil {
		fail(w, err)
		return
	}

	ids := make([]int64, len(svcs))
	for i, s := range svcs {
		ids[i] = s.ID
	}
	statuses, err := a.cache.GetStatuses(r.Context(), ids)
	if err != nil {
		slog.Warn("status cache unavailable", "err", err) // degrade to "unknown"
	}

	type item struct {
		Service
		Status Status `json:"status"`
	}
	out := make([]item, len(svcs))
	for i, s := range svcs {
		st, ok := statuses[s.ID]
		if !ok {
			st = Status{State: "unknown"}
		}
		out[i] = item{Service: s, Status: st}
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) createService(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		Name            string `json:"name"`
		URL             string `json:"url"`
		IntervalSeconds int    `json:"interval_seconds"`
		ExpectedStatus  int    `json:"expected_status"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	svc := Service{
		Name:            strings.TrimSpace(req.Name),
		URL:             strings.TrimSpace(req.URL),
		IntervalSeconds: req.IntervalSeconds,
		ExpectedStatus:  req.ExpectedStatus,
	}
	if svc.IntervalSeconds == 0 {
		svc.IntervalSeconds = 30
	}
	if svc.ExpectedStatus == 0 {
		svc.ExpectedStatus = 200
	}
	if err := svc.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := a.store.CreateService(r.Context(), &svc); err != nil {
		fail(w, err)
		return
	}
	w.Header().Set("Location", "/api/services/"+strconv.FormatInt(svc.ID, 10))
	writeJSON(w, http.StatusCreated, svc)
}

func (a *API) getService(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	svc, err := a.store.GetService(r.Context(), id)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, svc)
}

// listChecks returns recent checks, newest first. Query: ?limit=1..500 (default 50).
func (a *API) listChecks(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 500 {
			writeError(w, http.StatusBadRequest, "limit must be between 1 and 500")
			return
		}
		limit = n
	}
	if _, err := a.store.GetService(r.Context(), id); err != nil {
		fail(w, err)
		return
	}
	checks, err := a.store.ListChecks(r.Context(), id, limit)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, checks)
}

// getStatus returns the latest status (Redis, falling back to Postgres) plus 24h uptime.
func (a *API) getStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	if _, err := a.store.GetService(ctx, id); err != nil {
		fail(w, err)
		return
	}

	st, err := a.cache.GetStatus(ctx, id)
	if err != nil {
		slog.Warn("status cache unavailable", "err", err)
	}
	if st == nil {
		st = &Status{State: "unknown"}
		latest, err := a.store.LatestCheck(ctx, id)
		if err != nil {
			fail(w, err)
			return
		}
		if latest != nil {
			s := latest.Status()
			st = &s
		}
	}

	pct, total, err := a.store.Uptime24h(ctx, id)
	if err != nil {
		fail(w, err)
		return
	}
	if total > 0 {
		st.Uptime24h = &pct
		st.Checks24h = total
	}
	writeJSON(w, http.StatusOK, st)
}

func (a *API) deleteService(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := a.store.DeleteService(r.Context(), id); err != nil {
		fail(w, err)
		return
	}
	if err := a.cache.Forget(r.Context(), id); err != nil {
		slog.Warn("clear cache", "service", id, "err", err)
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- helpers ---

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "invalid service id")
		return 0, false
	}
	return id, true
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func fail(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}
	slog.Error("request failed", "err", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

// logRequests logs writes and errors at info level; routine GETs (UI polling) at debug.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, code: http.StatusOK}
		next.ServeHTTP(sw, r)

		level := slog.LevelDebug
		if r.Method != http.MethodGet || sw.code >= 400 {
			level = slog.LevelInfo
		}
		slog.Log(r.Context(), level, "request",
			"method", r.Method, "path", r.URL.Path, "status", sw.code, "dur", time.Since(start))
	})
}
