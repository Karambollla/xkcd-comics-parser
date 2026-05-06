package xkcd

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/Karambollla/course/update/core"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r) // ne idem v set
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testClient(t *testing.T, expectedPath string, status int, body string) Client {
	t.Helper()

	return Client{
		log: testLogger(),
		client: http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				require.Equal(t, expectedPath, r.URL.Path)
				return &http.Response{
					StatusCode: status,
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			}),
		},
		url: "https://xkcd.test",
	}
}

func TestNewClient(t *testing.T) {
	t.Run("returns client", func(t *testing.T) {
		client, err := NewClient("https://xkcd.com", time.Second, testLogger())

		require.NoError(t, err)
		require.NotNil(t, client)
		require.NoError(t, client.Close())
	})

	t.Run("returns error for empty url", func(t *testing.T) {
		client, err := NewClient("", time.Second, testLogger())

		require.Error(t, err)
		require.Nil(t, client)
	})
}

func TestGet(t *testing.T) {
	testCases := []struct {
		name        string
		status      int
		body        string
		expected    core.XKCDInfo
		expectedErr error
		wantErr     bool
	}{
		{
			name:   "returns comics",
			status: http.StatusOK,
			body:   `{"num":42,"img":"https://imgs.xkcd.com/comics/test.png","title":"Title","safe_title":"Safe","transcript":"Text","alt":"Alt"}`,
			expected: core.XKCDInfo{
				ID:          42,
				URL:         "https://imgs.xkcd.com/comics/test.png",
				Description: "Title Safe Text Alt",
			},
		},
		{
			name:        "returns not found",
			status:      http.StatusNotFound,
			expectedErr: core.ErrNotFound,
		},
		{
			name:    "returns error for unexpected status",
			status:  http.StatusInternalServerError,
			wantErr: true,
		},
		{
			name:    "returns error for invalid json",
			status:  http.StatusOK,
			body:    `{`,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := testClient(t, "/42/info.0.json", tc.status, tc.body)

			info, err := client.Get(context.Background(), 42)

			if tc.expectedErr != nil {
				require.ErrorIs(t, err, tc.expectedErr)
				return
			}
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expected, info)
		})
	}
}

func TestGetRequestError(t *testing.T) {
	client := Client{
		log: testLogger(),
		client: http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("boom")
			}),
		},
		url: "https://xkcd.test",
	}

	_, err := client.Get(context.Background(), 42)

	require.Error(t, err)
}

func TestLastID(t *testing.T) {
	testCases := []struct {
		name        string
		status      int
		body        string
		expectedID  int
		expectedErr error
		wantErr     bool
	}{
		{
			name:       "returns last id",
			status:     http.StatusOK,
			body:       `{"num":3000}`,
			expectedID: 3000,
		},
		{
			name:    "returns error for unexpected status",
			status:  http.StatusInternalServerError,
			wantErr: true,
		},
		{
			name:    "returns error for invalid json",
			status:  http.StatusOK,
			body:    `{`,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := testClient(t, "/info.0.json", tc.status, tc.body)

			id, err := client.LastID(context.Background())

			if tc.expectedErr != nil {
				require.ErrorIs(t, err, tc.expectedErr)
				return
			}
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expectedID, id)
		})
	}
}

func TestLastIDRequestError(t *testing.T) {
	client := Client{
		log: testLogger(),
		client: http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("boom")
			}),
		},
		url: "https://xkcd.test",
	}

	_, err := client.LastID(context.Background())

	require.Error(t, err)
}

func TestClose(t *testing.T) {
	client, err := NewClient("https://xkcd.com", time.Second, testLogger())
	require.NoError(t, err)

	err = client.Close()

	require.NoError(t, err)
}

func TestGetCanceledContext(t *testing.T) {
	client, err := NewClient("https://xkcd.com", time.Second, testLogger())
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = client.Get(ctx, 42)

	require.True(t, errors.Is(err, context.Canceled))
}
