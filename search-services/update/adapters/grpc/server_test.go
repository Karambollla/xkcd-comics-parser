package grpc

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	updatepb "github.com/Karambollla/course/proto/update"
	"github.com/Karambollla/course/update/core"
)

type fakeUpdater struct {
	updateErr error
	stats     core.ServiceStats
	statsErr  error
	status    core.ServiceStatus
	dropErr   error
}

func (f fakeUpdater) Update(context.Context) error {
	return f.updateErr
}

func (f fakeUpdater) Stats(context.Context) (core.ServiceStats, error) {
	return f.stats, f.statsErr
}

func (f fakeUpdater) Status(context.Context) core.ServiceStatus {
	return f.status
}

func (f fakeUpdater) Drop(context.Context) error {
	return f.dropErr
}

func TestPing(t *testing.T) {
	server := NewServer(fakeUpdater{})

	resp, err := server.Ping(context.Background(), &emptypb.Empty{})

	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestStatus(t *testing.T) {
	testCases := []struct {
		name           string
		status         core.ServiceStatus
		expectedStatus updatepb.Status
	}{
		{
			name:           "returns running",
			status:         core.StatusRunning,
			expectedStatus: updatepb.Status_STATUS_RUNNING,
		},
		{
			name:           "returns idle",
			status:         core.StatusIdle,
			expectedStatus: updatepb.Status_STATUS_IDLE,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := NewServer(fakeUpdater{status: tc.status})

			resp, err := server.Status(context.Background(), &emptypb.Empty{})

			require.NoError(t, err)
			require.Equal(t, tc.expectedStatus, resp.Status)
		})
	}
}

func TestUpdate(t *testing.T) {
	testCases := []struct {
		name         string
		err          error
		expectedCode codes.Code
	}{
		{
			name:         "returns ok",
			expectedCode: codes.OK,
		},
		{
			name:         "returns already exists",
			err:          core.ErrAlreadyExists,
			expectedCode: codes.AlreadyExists,
		},
		{
			name:         "returns internal",
			err:          errors.New("boom"),
			expectedCode: codes.Internal,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := NewServer(fakeUpdater{updateErr: tc.err})

			resp, err := server.Update(context.Background(), &emptypb.Empty{})

			require.Equal(t, tc.expectedCode, status.Code(err))
			if tc.expectedCode == codes.OK {
				require.NoError(t, err)
				require.NotNil(t, resp)
			}
		})
	}
}

func TestStats(t *testing.T) {
	stats := core.ServiceStats{
		DBStats: core.DBStats{
			WordsTotal:    10,
			WordsUnique:   7,
			ComicsFetched: 3,
		},
		ComicsTotal: 5,
	}
	server := NewServer(fakeUpdater{stats: stats})

	resp, err := server.Stats(context.Background(), &emptypb.Empty{})

	require.NoError(t, err)
	require.Equal(t, int64(stats.WordsTotal), resp.WordsTotal)
	require.Equal(t, int64(stats.WordsUnique), resp.WordsUnique)
	require.Equal(t, int64(stats.ComicsFetched), resp.ComicsFetched)
	require.Equal(t, int64(stats.ComicsTotal), resp.ComicsTotal)
}

func TestStatsError(t *testing.T) {
	expectedErr := errors.New("boom")
	server := NewServer(fakeUpdater{statsErr: expectedErr})

	resp, err := server.Stats(context.Background(), &emptypb.Empty{})

	require.ErrorIs(t, err, expectedErr)
	require.Nil(t, resp)
}

func TestDrop(t *testing.T) {
	testCases := []struct {
		name string
		err  error
	}{
		{
			name: "returns ok",
		},
		{
			name: "returns error",
			err:  errors.New("boom"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := NewServer(fakeUpdater{dropErr: tc.err})

			resp, err := server.Drop(context.Background(), &emptypb.Empty{})

			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				require.Nil(t, resp)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, resp)
		})
	}
}
