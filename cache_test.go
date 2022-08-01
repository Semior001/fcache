package fcache

import (
	"context"
	"io"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadingCache_GetFile(t *testing.T) {
	t.Run("hit", func(t *testing.T) {
		now := time.Now()

		svc := &LoadingCache{
			Store: &StoreMock{
				MetaFunc: func(ctx context.Context, key string) (FileMeta, error) {
					assert.Equal(t, "key", key)
					return FileMeta{
						Name:      "a.txt",
						Mime:      "text/plain",
						Size:      17,
						Key:       "key",
						CreatedAt: now,
					}, nil
				},
				GetFunc: func(ctx context.Context, key string) (io.ReadCloser, error) {
					assert.Equal(t, "key", key)
					return io.NopCloser(strings.NewReader("some file data")), nil
				},
			},
		}

		rd, meta, err := svc.GetFile(context.Background(), GetRequest{Key: "key", TTL: 30 * time.Minute, Loader: nil})
		require.NoError(t, err)
		assert.Equal(t, FileMeta{
			Name:      "a.txt",
			Mime:      "text/plain",
			Size:      17,
			Key:       "key",
			CreatedAt: now,
		}, meta)
		bts, err := io.ReadAll(rd)
		require.NoError(t, err)
		assert.Equal(t, []byte("some file data"), bts)
		assert.Equal(t, CacheStats{Hits: 1}, svc.CacheStats)
	})

	t.Run("miss", func(t *testing.T) {
		now := time.Now()

		svc := &LoadingCache{
			now: func() time.Time { return now },
			Store: &StoreMock{
				MetaFunc: func(ctx context.Context, key string) (FileMeta, error) {
					assert.Equal(t, "key", key)
					return FileMeta{}, ErrNotFound
				},
				PutFunc: func(ctx context.Context, key string, meta FileMeta, rd io.ReadCloser) error {
					assert.Equal(t, "key", key)
					assert.Equal(t, FileMeta{
						Name: "a.txt",
						Mime: "text/plain",
						Size: 17,
						Meta: map[string]string{
							metaInvalidateAtKey: now.Add(30 * time.Minute).Format(metaTimeFormat),
						},
						Key:       "key",
						CreatedAt: now,
					}, meta)
					bts, err := io.ReadAll(rd)
					require.NoError(t, err)
					assert.Equal(t, []byte("some file data"), bts)
					return nil
				},
			},
		}

		rd, meta, err := svc.GetFile(context.Background(), GetRequest{
			Key: "key",
			TTL: 30 * time.Minute,
			Loader: func(ctx context.Context) (io.ReadCloser, FileMeta, error) {
				return io.NopCloser(strings.NewReader("some file data")), FileMeta{
					Name:      "a.txt",
					Mime:      "text/plain",
					Size:      17,
					Key:       "key",
					CreatedAt: now,
				}, nil
			},
		})
		require.NoError(t, err)
		assert.Equal(t, FileMeta{
			Name:      "a.txt",
			Mime:      "text/plain",
			Meta:      map[string]string{metaInvalidateAtKey: now.Add(30 * time.Minute).Format(metaTimeFormat)},
			Size:      17,
			Key:       "key",
			CreatedAt: now,
		}, meta)
		bts, err := io.ReadAll(rd)
		require.NoError(t, err)
		require.NoError(t, rd.Close())
		assert.Equal(t, []byte("some file data"), bts)
		assert.Equal(t, CacheStats{Misses: 1}, svc.CacheStats)
	})
}

func TestLoadingCache_GetURL(t *testing.T) {
	t.Run("hit", func(t *testing.T) {
		now := time.Now()

		svc := &LoadingCache{
			Store: &StoreMock{
				MetaFunc: func(ctx context.Context, key string) (FileMeta, error) {
					assert.Equal(t, "key", key)
					return FileMeta{
						Name:      "a.txt",
						Mime:      "text/plain",
						Size:      17,
						Key:       "key",
						CreatedAt: now,
					}, nil
				},
				GetURLFunc: func(ctx context.Context, key string, params GetURLParams) (string, error) {
					assert.Equal(t, "key", key)
					assert.Equal(t, 15*time.Minute, params.Expires)
					assert.Equal(t, "somefile.txt", params.Filename)
					return "file-url", nil
				},
			},
		}

		url, meta, err := svc.GetURL(context.Background(),
			GetRequest{Key: "key"},
			GetURLParams{Filename: "somefile.txt", Expires: 15 * time.Minute},
		)
		require.NoError(t, err)
		assert.Equal(t, FileMeta{
			Name:      "a.txt",
			Mime:      "text/plain",
			Size:      17,
			Key:       "key",
			CreatedAt: now,
		}, meta)
		assert.Equal(t, "file-url", url)
		assert.Equal(t, CacheStats{Hits: 1}, svc.CacheStats)
	})

	t.Run("miss", func(t *testing.T) {
		now := time.Now()

		svc := &LoadingCache{
			now: func() time.Time { return now },
			Store: &StoreMock{
				MetaFunc: func(ctx context.Context, key string) (FileMeta, error) {
					assert.Equal(t, "key", key)
					return FileMeta{}, ErrNotFound
				},
				PutFunc: func(ctx context.Context, key string, meta FileMeta, rd io.ReadCloser) error {
					assert.Equal(t, "key", key)
					assert.Equal(t, FileMeta{
						Name: "a.txt",
						Mime: "text/plain",
						Size: 17,
						Meta: map[string]string{
							metaInvalidateAtKey: now.Add(30 * time.Minute).Format(metaTimeFormat),
						},
						Key:       "key",
						CreatedAt: now,
					}, meta)
					bts, err := io.ReadAll(rd)
					require.NoError(t, err)
					assert.Equal(t, []byte("some file data"), bts)
					return nil
				},
				GetURLFunc: func(ctx context.Context, key string, params GetURLParams) (string, error) {
					assert.Equal(t, "key", key)
					assert.Equal(t, 15*time.Minute, params.Expires)
					assert.Empty(t, params.Filename)
					return "file-url", nil
				},
			},
		}

		url, meta, err := svc.GetURL(context.Background(), GetRequest{
			Key: "key",
			TTL: 30 * time.Minute,
			Loader: func(ctx context.Context) (io.ReadCloser, FileMeta, error) {
				return io.NopCloser(strings.NewReader("some file data")), FileMeta{
					Name:      "a.txt",
					Mime:      "text/plain",
					Size:      17,
					Key:       "key",
					CreatedAt: now,
				}, nil
			},
		}, GetURLParams{Expires: 15 * time.Minute})
		require.NoError(t, err)
		assert.Equal(t, FileMeta{
			Name:      "a.txt",
			Mime:      "text/plain",
			Size:      17,
			Meta:      map[string]string{metaInvalidateAtKey: now.Add(30 * time.Minute).Format(metaTimeFormat)},
			Key:       "key",
			CreatedAt: now,
		}, meta)
		assert.Equal(t, "file-url", url)
		assert.Equal(t, CacheStats{Misses: 1}, svc.CacheStats)
	})
}

func TestLoadingCache_Stat(t *testing.T) {
	svc := &LoadingCache{
		Store: &StoreMock{StatFunc: func(ctx context.Context) (StoreStats, error) {
			return StoreStats{Keys: 14, Size: 213456}, nil
		}},
		CacheStats: CacheStats{
			Hits:   12,
			Misses: 14,
			Errors: 15,
		},
	}
	stat, err := svc.Stat(context.Background())
	require.NoError(t, err)
	assert.Equal(t, CacheStats{
		Hits:   12,
		Misses: 14,
		Errors: 15,
		StoreStats: StoreStats{
			Keys: 14,
			Size: 213456,
		},
	}, stat)
}

func TestLoadingCache_Invalidation(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		now := time.Date(2022, time.July, 5, 6, 51, 21, 0, time.UTC)

		invalidationMeta := func(tm time.Time) map[string]string {
			return map[string]string{metaInvalidateAtKey: tm.Format(metaTimeFormat)}
		}

		ctx, cancel := context.WithCancel(context.Background())
		store := &StoreMock{
			ListFunc: func(ctx context.Context) ([]FileMeta, error) {
				cancel()
				return []FileMeta{
					{Key: "key", Meta: invalidationMeta(now.Add(-15 * time.Minute))},   // will be removed
					{Key: "key-1", Meta: invalidationMeta(now.Add(15 * time.Minute))},  // will NOT be removed
					{Key: "key-2", Meta: invalidationMeta(now.Add(-30 * time.Minute))}, // will be removed
					{Key: "key-3"}, // will NOT be removed
				}, nil
			},
			RemoveFunc: func(ctx context.Context, key string) error { return nil },
		}
		svc := &LoadingCache{
			now: func() time.Time { return now },
			Options: Options{
				InvalidatePeriod: time.Millisecond,
				Log:              NopLogger(),
			},
			Store: store,
		}
		err := svc.Run(ctx)
		assert.Equal(t, context.Canceled, err)

		removeCalls := store.RemoveCalls()
		sort.Slice(removeCalls, func(i, j int) bool { return removeCalls[i].Key < removeCalls[j].Key })
		assert.Equal(t, []struct {
			Ctx context.Context
			Key string
		}{
			{Ctx: ctx, Key: "key"},
			{Ctx: ctx, Key: "key-2"},
		}, removeCalls)
		assert.Equal(t, 1, len(store.ListCalls()))
	})
}
