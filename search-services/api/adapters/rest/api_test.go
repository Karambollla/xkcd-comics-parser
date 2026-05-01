package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Karambollla/course/api/core"
	"github.com/stretchr/testify/require"
)

type fakeAuth struct {
	token string
	err   error
}

func (f fakeAuth) Login(_, _ string) (string, error) {
	return f.token, f.err
}

type fakePinger struct {
	err error
}

func (f fakePinger) Ping(context.Context) error {
	return f.err
}

type fakeUpdater struct {
	updateErr error
	stats     core.UpdateStats
	statsErr  error
	status    core.UpdateStatus
	statusErr error
	dropErr   error
}

func (f fakeUpdater) Update(context.Context) error {
	return f.updateErr
}

func (f fakeUpdater) Stats(context.Context) (core.UpdateStats, error) {
	return f.stats, f.statsErr
}

func (f fakeUpdater) Status(context.Context) (core.UpdateStatus, error) {
	return f.status, f.statusErr
}

func (f fakeUpdater) Drop(context.Context) error {
	return f.dropErr
}

type fakeSearcher struct {
	comics []core.Comics
	total  int
	err    error
	phrase string
	limit  int
}

func (f *fakeSearcher) Search(_ context.Context, phrase string, limit int) ([]core.Comics, int, error) {
	f.phrase = phrase
	f.limit = limit
	return f.comics, f.total, f.err
}

func (f *fakeSearcher) SearchIndex(_ context.Context, phrase string, limit int) ([]core.Comics, int, error) {
	f.phrase = phrase
	f.limit = limit
	return f.comics, f.total, f.err
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNewLoginHandler(t *testing.T) {
	testCases := []struct {
		name           string
		body           string
		authErr        error
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "returns token",
			body:           `{"name":"user","password":"pass"}`,
			expectedStatus: http.StatusOK,
			expectedBody:   "token",
		},
		{
			name:           "returns bad request for invalid json",
			body:           `{`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "returns unauthorized for invalid credentials",
			body:           `{"name":"user","password":"bad"}`,
			authErr:        errors.New("invalid credentials"),
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewLoginHandler(testLogger(), fakeAuth{token: "token", err: tc.authErr})
			req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(tc.body))
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			require.Equal(t, tc.expectedStatus, w.Code)
			if tc.expectedBody != "" {
				require.Equal(t, tc.expectedBody, w.Body.String())
			}
		})
	}
}

func TestNewPingHandler(t *testing.T) {
	handler := NewPingHandler(testLogger(), map[string]core.Pinger{
		"ok":   fakePinger{},
		"fail": fakePinger{err: errors.New("down")},
	})
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	var resp PingResponse
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.Equal(t, "ok", resp.Replies["ok"])
	require.Equal(t, "unavailable", resp.Replies["fail"])
}

func TestNewMetricsHandler(t *testing.T) {
	handler := NewMetricsHandler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotEmpty(t, w.Body.String())
}

func TestNewUpdateHandler(t *testing.T) {
	testCases := []struct {
		name           string
		err            error
		expectedStatus int
	}{
		{
			name:           "returns ok",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "returns accepted when update already exists",
			err:            core.ErrAlreadyExists,
			expectedStatus: http.StatusAccepted,
		},
		{
			name: "returns internal server error",
			err:  errors.New("boom"), expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewUpdateHandler(testLogger(), fakeUpdater{updateErr: tc.err})
			req := httptest.NewRequest(http.MethodPost, "/update", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			require.Equal(t, tc.expectedStatus, w.Code)
		})
	}
}

func TestNewUpdateStatsHandler(t *testing.T) {
	stats := core.UpdateStats{
		WordsTotal:    10,
		WordsUnique:   7,
		ComicsFetched: 3,
		ComicsTotal:   5,
	}
	handler := NewUpdateStatsHandler(testLogger(), fakeUpdater{stats: stats})
	req := httptest.NewRequest(http.MethodGet, "/update/stats", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	var resp UpdateStatsResponse
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.Equal(t, stats.WordsTotal, resp.WordsTotal)
	require.Equal(t, stats.WordsUnique, resp.WordsUnique)
	require.Equal(t, stats.ComicsFetched, resp.ComicsFetched)
	require.Equal(t, stats.ComicsTotal, resp.ComicsTotal)
}

func TestNewUpdateStatusHandler(t *testing.T) {
	testCases := []struct {
		name           string
		status         core.UpdateStatus
		err            error
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "returns status",
			status:         core.StatusUpdateRunning,
			expectedStatus: http.StatusOK,
			expectedBody:   "running",
		},
		{
			name:           "returns internal server error",
			err:            errors.New("boom"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewUpdateStatusHandler(testLogger(), fakeUpdater{status: tc.status, statusErr: tc.err})
			req := httptest.NewRequest(http.MethodGet, "/update/status", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			require.Equal(t, tc.expectedStatus, w.Code)
			if tc.expectedBody != "" {
				var resp map[string]string
				require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
				require.Equal(t, tc.expectedBody, resp["status"])
			}
		})
	}
}

func TestNewDropHandler(t *testing.T) {
	testCases := []struct {
		name           string
		err            error
		expectedStatus int
	}{
		{
			name:           "returns ok",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "returns internal server error",
			err:            errors.New("boom"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewDropHandler(testLogger(), fakeUpdater{dropErr: tc.err})
			req := httptest.NewRequest(http.MethodDelete, "/drop", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			require.Equal(t, tc.expectedStatus, w.Code)
		})
	}
}

func TestSearchHandlers(t *testing.T) {
	testCases := []struct {
		name           string
		url            string
		index          bool
		searchErr      error
		expectedStatus int
		expectedPhrase string
		expectedLimit  int
	}{
		{
			name:           "search uses default limit",
			url:            "/search?phrase=go",
			expectedStatus: http.StatusOK,
			expectedPhrase: "go",
			expectedLimit:  minLim,
		},
		{
			name:           "index search uses requested limit",
			url:            "/isearch?phrase=go&limit=3",
			index:          true,
			expectedStatus: http.StatusOK,
			expectedPhrase: "go",
			expectedLimit:  3,
		},
		{
			name:           "returns bad request for empty phrase",
			url:            "/search",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "returns bad request for invalid limit",
			url:            "/search?phrase=go&limit=bad",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "returns bad request on search error",
			url:            "/search?phrase=go",
			searchErr:      errors.New("boom"),
			expectedStatus: http.StatusBadRequest,
			expectedPhrase: "go",
			expectedLimit:  minLim,
		},
		{
			name:           "index search returns bad request for empty phrase",
			url:            "/isearch",
			index:          true,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "index search returns bad request for invalid limit",
			url:            "/isearch?phrase=go&limit=0",
			index:          true,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "index search returns bad request on search error",
			url:            "/isearch?phrase=go",
			index:          true,
			searchErr:      errors.New("boom"),
			expectedStatus: http.StatusBadRequest,
			expectedPhrase: "go",
			expectedLimit:  minLim,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			searcher := &fakeSearcher{
				comics: []core.Comics{{ID: 1, URL: "https://example.com", Score: 10}},
				total:  1,
				err:    tc.searchErr,
			}
			handler := NewSearchHandler(testLogger(), searcher)
			if tc.index {
				handler = NewSearchIndexHandler(testLogger(), searcher)
			}

			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			require.Equal(t, tc.expectedStatus, w.Code)
			require.Equal(t, tc.expectedPhrase, searcher.phrase)
			require.Equal(t, tc.expectedLimit, searcher.limit)
			if tc.expectedStatus == http.StatusOK {
				var resp SearchReply
				require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
				require.Equal(t, 1, resp.Total)
				require.Len(t, resp.Comics, 1)
			}
		})
	}
}

func TestNewISearchHandler(t *testing.T) {
	searcher := &fakeSearcher{
		comics: []core.Comics{{ID: 1, URL: "https://example.com", Score: 10}},
		total:  1,
	}
	handler := NewISearchHandler(testLogger(), searcher)
	req := httptest.NewRequest(http.MethodGet, "/isearch?phrase=go&limit=2", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "go", searcher.phrase)
	require.Equal(t, 2, searcher.limit)
}
