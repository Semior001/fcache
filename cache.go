package fcache

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-multierror"
)

// Loader is a function to load a file in case if it's missing in cache.
type Loader func(ctx context.Context) (io.ReadCloser, FileMeta, error)

// LoadingCache is a wrapper for Store, which removes file at their TTL
type LoadingCache struct {
	Store
	Options
	CacheStats

	// mockable fields
	now func() time.Time
}

// NewLoadingCache makes new instance of LoadingCache.
func NewLoadingCache(backend Store, opts ...Option) *LoadingCache {
	res := &LoadingCache{
		Store: backend,
		Options: Options{
			TTL:              30 * time.Minute,
			InvalidatePeriod: 15 * time.Minute,
			Log:              stdLogger{},
		},
		now: time.Now,
	}

	for _, opt := range opts {
		opt(&res.Options)
	}

	return res
}

// GetFile gets the file from cache or loads it, if absent.
func (l *LoadingCache) GetFile(ctx context.Context, key string, fn Loader) (rd io.Reader, meta FileMeta, err error) {
	if meta, err = l.Store.Meta(ctx, key); err == nil {
		// cache hit
		atomic.AddInt64(&l.Hits, 1)

		if rd, err = l.Store.Get(ctx, key); err != nil {
			atomic.AddInt64(&l.Errors, 1)
			return rd, meta, fmt.Errorf("get file reader: %w", err)
		}

		return rd, meta, nil
	}

	if err != nil && !errors.Is(err, ErrNotFound) {
		// store returned unexpected error
		atomic.AddInt64(&l.Errors, 1)
		return nil, FileMeta{}, fmt.Errorf("get file from storage: %w", err)
	}

	// miss
	atomic.AddInt64(&l.Misses, 1)

	originalRd, meta, err := fn(ctx)
	if err != nil {
		return nil, FileMeta{}, fmt.Errorf("loader returned error: %w", err)
	}

	// duplicating reader to still return file content, when reader is emptied
	// fixme: probably this part needs to be limited, or file should be saved in
	// tmp, so a limited amount of files would be in memory
	buf := &bytes.Buffer{}
	putRd := io.TeeReader(originalRd, buf)
	rd = io.NopCloser(buf)

	if err = l.Store.Put(ctx, key, meta, io.NopCloser(putRd)); err != nil {
		return rd, meta, fmt.Errorf("put file into storage: %w", err)
	}

	if err = originalRd.Close(); err != nil {
		return rd, meta, fmt.Errorf("close reader, received from loader: %w", err)
	}

	return rd, meta, nil
}

// GetURL returns the URL from the cache backend.
func (l *LoadingCache) GetURL(ctx context.Context, key string, req GetURLParams, fn Loader) (url string, meta FileMeta, err error) {
	getURL := func(meta FileMeta) (string, FileMeta, error) {
		u, err := l.Store.GetURL(ctx, key, req)
		if err != nil {
			atomic.AddInt64(&l.Errors, 1)
			return "", FileMeta{}, fmt.Errorf("get url from storage: %w", err)
		}

		return u, meta, nil
	}

	if meta, err = l.Store.Meta(ctx, key); err == nil {
		// cache hit
		atomic.AddInt64(&l.Hits, 1)
		return getURL(meta)
	}

	if err != nil && !errors.Is(err, ErrNotFound) {
		// store returned unexpected error
		atomic.AddInt64(&l.Errors, 1)
		return "", FileMeta{}, fmt.Errorf("get file meta from storage: %w", err)
	}

	// miss
	atomic.AddInt64(&l.Misses, 1)

	rd, meta, err := fn(ctx)
	if err != nil {
		atomic.AddInt64(&l.Errors, 1)
		return "", FileMeta{}, fmt.Errorf("loader returned error: %w", err)
	}

	if err = l.Store.Put(ctx, key, meta, rd); err != nil {
		atomic.AddInt64(&l.Errors, 1)
		return "", FileMeta{}, fmt.Errorf("put file into storage: %w", err)
	}

	return getURL(meta)
}

// CacheStats represent stat values.
type CacheStats struct {
	Hits   int64
	Misses int64
	Errors int64
	StoreStats
}

// Stat returns cache stats
func (l *LoadingCache) Stat(ctx context.Context) (CacheStats, error) {
	res := CacheStats{
		Hits:   l.Hits,
		Misses: l.Misses,
		Errors: l.Errors,
	}

	storeStats, err := l.Store.Stat(ctx)
	if err != nil {
		return res, fmt.Errorf("get store stats: %w", err)
	}

	res.Keys = storeStats.Keys
	res.Size = storeStats.Size

	return res, nil
}

// Run runs invalidation goroutine. It will check for files TTL expiration
// and, if it expires, removes it manually.
func (l *LoadingCache) Run(ctx context.Context) error {
	if l.InvalidatePeriod == 0 {
		return errors.New("invalidation period cannot be zero")
	}

	ticker := time.NewTicker(l.InvalidatePeriod)
	for {
		select {
		case <-ticker.C:
			if err := l.invalidate(ctx); err != nil {
				l.Log.Printf("[WARN] failed to invalidate cache items: %v", err)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (l *LoadingCache) invalidate(ctx context.Context) error {
	metas, err := l.Store.List(ctx)
	if err != nil {
		return fmt.Errorf("list objects from store: %w", err)
	}

	errs := &multierror.Error{}

	for _, meta := range metas {
		if meta.CreatedAt.Add(l.TTL).Before(l.now()) {
			if err = l.Store.Remove(ctx, meta.Key); err != nil {
				errs = multierror.Append(err, fmt.Errorf("remove file under key %q: %w", meta.Key, err))
				continue
			}
		}
		l.Log.Printf("[DEBUG] removed file with key %q", meta.Key)
	}

	return errs.ErrorOrNil()
}
