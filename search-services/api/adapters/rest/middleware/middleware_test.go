package middleware

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"github.com/stretchr/testify/require"
)

type fakeVerifier struct {
	err error
}

func (f fakeVerifier) Verify(token string) error {
	return f.err
}

func nextHandler(called *bool, status int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		*called = true
		w.WriteHeader(status)
	}
}

func TestAuth(t *testing.T) {
	testCases := []struct {
		name           string
		header         string
		verifyErr      error
		expectedStatus int
		expectedNext   bool
	}{
		{
			name:           "missing token returns unauthorized",
			header:         "",
			verifyErr:      nil,
			expectedStatus: http.StatusUnauthorized,
			expectedNext:   false,
		},
		{
			name:           "empty token returns unauthorized",
			header:         "Token ",
			verifyErr:      nil,
			expectedStatus: http.StatusUnauthorized,
			expectedNext:   false,
		},
		{
			name:           "invalid token returns unauthorized",
			header:         "Token bad-token",
			verifyErr:      errors.New("bad token"),
			expectedStatus: http.StatusUnauthorized,
			expectedNext:   false,
		},
		{
			name:           "valid token calls next handler",
			header:         "Token valid-token",
			verifyErr:      nil,
			expectedStatus: http.StatusOK,
			expectedNext:   true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			verifier := fakeVerifier{err: tc.verifyErr}
			nextCalled := false
			next := nextHandler(&nextCalled, tc.expectedStatus)

			handler := Auth(next, verifier)
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", tc.header)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			require.Equal(t, tc.expectedStatus, w.Code)
			require.Equal(t, tc.expectedNext, nextCalled)
		})
	}
}

func TestRate(t *testing.T) {
	testCases := []struct {
		name           string
		rps            int
		cd             time.Duration
		maxWait        time.Duration
		requests       int
		expectedCalls  int32
		expectedStatus int
	}{
		{
			name:           "defaults non positive rate to one",
			rps:            0,
			cd:             0,
			maxWait:        0,
			requests:       1,
			expectedCalls:  1,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "defaults non positive rate to one",
			rps:            -5,
			cd:             0,
			maxWait:        0,
			requests:       1,
			expectedCalls:  1,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "allows request within limit",
			rps:            1,
			cd:             0,
			maxWait:        0,
			requests:       1,
			expectedCalls:  1,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "limiter limits requests",
			rps:            10,
			cd:             50 * time.Millisecond,
			maxWait:        300 * time.Millisecond,
			requests:       2,
			expectedCalls:  2,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var nextCalls atomic.Int32
			next := func(w http.ResponseWriter, r *http.Request) {
				nextCalls.Add(1)
				w.WriteHeader(tc.expectedStatus)
			}
			handler := Rate(next, tc.rps)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			w1 := httptest.NewRecorder()
			handler.ServeHTTP(w1, req)

			require.Equal(t, tc.expectedStatus, w1.Code)

			if tc.requests == 2 {
				done := make(chan int, 1)
				go func() {
					w2 := httptest.NewRecorder()
					handler.ServeHTTP(w2, req)
					done <- w2.Code
				}()

				select {
				case <-done:
					t.Fatal("second request passed too early")
				case <-time.After(tc.cd):
				}

				select {
				case status := <-done:
					require.Equal(t, tc.expectedStatus, status)
				case <-time.After(tc.maxWait):
					t.Fatal("second request did not pass after rate limit delay")
				}
			}

			require.Equal(t, tc.expectedCalls, nextCalls.Load())
		})
	}
}

func TestConcurrency(t *testing.T) {
	testCases := []struct {
		name           string
		limit          int
		occupied       bool
		expectedStatus int
		expectedCalls  int32
	}{
		{
			name:           "allows request within limit",
			limit:          1,
			occupied:       false,
			expectedStatus: http.StatusOK,
			expectedCalls:  1,
		},
		{
			name:           "rejects request when limit is exceeded",
			limit:          1,
			occupied:       true,
			expectedStatus: http.StatusServiceUnavailable,
			expectedCalls:  1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var nextCalls atomic.Int32
			release := make(chan struct{})
			entered := make(chan struct{})

			next := func(w http.ResponseWriter, r *http.Request) {
				nextCalls.Add(1)
				if tc.occupied {
					entered <- struct{}{}
					<-release
				}
				w.WriteHeader(http.StatusOK)
			}

			handler := Concurrency(next, tc.limit)
			req := httptest.NewRequest(http.MethodGet, "/", nil)

			if tc.occupied {
				go func() {
					w := httptest.NewRecorder()
					handler.ServeHTTP(w, req)
				}()

				select {
				case <-entered:
				case <-time.After(100 * time.Millisecond):
					t.Fatal("request did not occupy concurrency slot")
				}

				defer close(release)
			}

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			require.Equal(t, tc.expectedStatus, w.Code)
			require.Equal(t, tc.expectedCalls, nextCalls.Load())
		})
	}
}

func TestWithMetrics(t *testing.T) {
	metrics.UnregisterAllMetrics()
	defer metrics.UnregisterAllMetrics()

	testCases := []struct {
		name           string
		path           string
		handlerStatus  int
		expectedMetric string
	}{
		{
			name:           "writes metric for 200",
			path:           "/ok",
			handlerStatus:  http.StatusOK,
			expectedMetric: `http_request_duration_seconds_count{status="200",url="/ok"} 1`,
		},
		{
			name:           "writes metric for 500",
			path:           "/error",
			handlerStatus:  http.StatusInternalServerError,
			expectedMetric: `http_request_duration_seconds_count{status="500",url="/error"} 1`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.handlerStatus)
			})
			handler := WithMetrics(next)

			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			require.Equal(t, tc.handlerStatus, w.Code)

			var buf bytes.Buffer
			metrics.WritePrometheus(&buf, false)
			require.Contains(t, buf.String(), tc.expectedMetric)
		})
	}
}
