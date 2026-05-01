package grpc

import (
	"context"
	"errors"
	"testing"

	searchpb "github.com/Karambollla/course/proto/search"
	"github.com/Karambollla/course/search/core"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
)

type fakeSearcher struct {
	comics         []core.Comics
	total          int
	searchErr      error
	searchIndexErr error
	rebuildErr     error
	query          string
	limit          int
}

func (f *fakeSearcher) Search(_ context.Context, query string, limit int) ([]core.Comics, int, error) {
	f.query = query
	f.limit = limit
	return f.comics, f.total, f.searchErr
}

func (f *fakeSearcher) SearchIndex(_ context.Context, query string, limit int) ([]core.Comics, int, error) {
	f.query = query
	f.limit = limit
	return f.comics, f.total, f.searchIndexErr
}

func (f *fakeSearcher) Rebuild(context.Context) error {
	return f.rebuildErr
}

func TestPing(t *testing.T) {
	server := NewServer(&fakeSearcher{})

	resp, err := server.Ping(context.Background(), &emptypb.Empty{})

	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestRebuild(t *testing.T) {
	testCases := []struct {
		name string
		err  error
	}{
		{
			name: "returns ok",
		},
		{
			name: "returns error",
			err:  errors.New("rebuild failed"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := NewServer(&fakeSearcher{rebuildErr: tc.err})

			resp, err := server.Rebuild(context.Background(), &emptypb.Empty{})

			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				require.NotNil(t, resp)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, resp)
		})
	}
}

func TestSearch(t *testing.T) {
	testCases := []struct {
		name        string
		index       bool
		err         error
		expectedErr bool
	}{
		{
			name: "search returns reply",
		},
		{
			name:  "search index returns reply",
			index: true,
		},
		{
			name:        "search returns error",
			err:         errors.New("search failed"),
			expectedErr: true,
		},
		{
			name:        "search index returns error",
			index:       true,
			err:         errors.New("index search failed"),
			expectedErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			searcher := &fakeSearcher{
				comics: []core.Comics{
					{ID: 1, URL: "url-1"},
					{ID: 2, URL: "url-2"},
				},
				total: 5,
			}
			if tc.index {
				searcher.searchIndexErr = tc.err
			} else {
				searcher.searchErr = tc.err
			}
			server := NewServer(searcher)
			req := &searchpb.SearchRequest{Words: "go", Limit: 10}

			var resp *searchpb.SearchReply
			var err error
			if tc.index {
				resp, err = server.SearchIndex(context.Background(), req)
			} else {
				resp, err = server.Search(context.Background(), req)
			}

			require.Equal(t, "go", searcher.query)
			require.Equal(t, 10, searcher.limit)
			if tc.expectedErr {
				require.ErrorIs(t, err, tc.err)
				require.Nil(t, resp)
				return
			}
			require.NoError(t, err)
			require.Equal(t, int32(5), resp.Total)
			require.Len(t, resp.Comics, 2)
			require.Equal(t, int32(1), resp.Comics[0].Id)
			require.Equal(t, "url-1", resp.Comics[0].Url)
		})
	}
}

func TestMakeSearchReply(t *testing.T) {
	resp := makeSearchReply([]core.Comics{{ID: 7, URL: "url-7"}}, 11)

	require.Equal(t, int32(11), resp.Total)
	require.Len(t, resp.Comics, 1)
	require.Equal(t, int32(7), resp.Comics[0].Id)
	require.Equal(t, "url-7", resp.Comics[0].Url)
}
