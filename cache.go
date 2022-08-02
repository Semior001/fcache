package fcache

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-multierror"
)

const (
	metaTimeFormat      = time.RFC3339Nano
	metaInvalidateAtKey = "_invalidate_at"
)

// Loader is a function to load a file in case if it's missing in cache.
type Loader func(ctx context.Context) (io.ReadCloser, FileMeta, error)

// LoadingCache is a wrapper for Store, which removes file at their TTL.
// Only files, added by GetFile and GetURL methods will be removed.
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
func (l *LoadingCache) GetFile(ctx context.Context, req GetRequest) (rd io.ReadCloser, meta FileMeta, err error) {
	if meta, err = l.Store.Meta(ctx, req.Key); err == nil {
		// cache hit
		atomic.AddInt64(&l.Hits, 1)

		if rd, err = l.Store.Get(ctx, req.Key); err != nil {
			atomic.AddInt64(&l.Errors, 1)
			return rd, meta, fmt.Errorf("get file reader: %w", err)
		}

		if meta, err = l.extendTTL(ctx, req.Key, req.TTL, meta); err != nil {
			return rd, meta, fmt.Errorf("extend file's TTL: %w", err)
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

	originalRd, meta, err := req.Loader(ctx)
	if err != nil {
		return nil, FileMeta{}, fmt.Errorf("loader returned error: %w", err)
	}

	// duplicating reader to still return file content, when reader is emptied
	tmp, err := os.CreateTemp(os.TempDir(), "fcache_*")
	if err != nil {
		return nil, FileMeta{}, fmt.Errorf("create temp file: %w", err)
	}
	putRd := io.TeeReader(originalRd, tmp)
	rd = &tempFile{File: tmp} // wrap file to delete it immediately, when is closed

	if meta.Meta == nil {
		meta.Meta = map[string]string{}
	}
	meta.Meta[metaInvalidateAtKey] = l.now().Add(req.TTL).Format(metaTimeFormat)

	if err = l.Store.Put(ctx, req.Key, meta, io.NopCloser(putRd)); err != nil {
		return rd, meta, fmt.Errorf("put file into storage: %w", err)
	}

	if _, err = tmp.Seek(0, io.SeekStart); err != nil {
		return rd, meta, fmt.Errorf("reset temp file caret to file start: %w", err)
	}

	if err = originalRd.Close(); err != nil {
		return rd, meta, fmt.Errorf("close reader, received from loader: %w", err)
	}

	return rd, meta, nil
}

// GetURL returns the URL from the cache backend.
func (l *LoadingCache) GetURL(ctx context.Context, req GetRequest, params GetURLParams) (url string, meta FileMeta, err error) {
	getURL := func(meta FileMeta) (string, FileMeta, error) {
		u, err := l.Store.GetURL(ctx, req.Key, params)
		if err != nil {
			atomic.AddInt64(&l.Errors, 1)
			return "", FileMeta{}, fmt.Errorf("get url from storage: %w", err)
		}

		return u, meta, nil
	}

	if meta, err = l.Store.Meta(ctx, req.Key); err == nil {
		// cache hit
		atomic.AddInt64(&l.Hits, 1)

		if meta, err = l.extendTTL(ctx, req.Key, req.TTL, meta); err != nil {
			return "", meta, fmt.Errorf("extend file's TTL: %w", err)
		}

		return getURL(meta)
	}

	if err != nil && !errors.Is(err, ErrNotFound) {
		// store returned unexpected error
		atomic.AddInt64(&l.Errors, 1)
		return "", FileMeta{}, fmt.Errorf("get file meta from storage: %w", err)
	}

	// miss
	atomic.AddInt64(&l.Misses, 1)

	rd, meta, err := req.Loader(ctx)
	if err != nil {
		atomic.AddInt64(&l.Errors, 1)
		return "", FileMeta{}, fmt.Errorf("loader returned error: %w", err)
	}

	if meta.Meta == nil {
		meta.Meta = map[string]string{}
	}
	meta.Meta[metaInvalidateAtKey] = l.now().Add(req.TTL).Format(metaTimeFormat)

	if err = l.Store.Put(ctx, req.Key, meta, rd); err != nil {
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
			invalidated, err := l.Invalidate(ctx)
			if err != nil {
				l.Log.Printf("[WARN] failed to invalidate cache items: %v", err)
			}
			l.Log.Printf("[DEBUG] invalidated %d items", invalidated)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Invalidate invalidates expired cache items.
// Used for tests.
func (l *LoadingCache) Invalidate(ctx context.Context) (invalidated int64, err error) {
	files, err := l.Store.List(ctx)
	if err != nil {
		return 0, fmt.Errorf("list objects from store: %w", err)
	}

	errs := &multierror.Error{}

	for _, file := range files {
		if file.Meta == nil {
			continue
		}
		tm, ok := file.Meta[metaInvalidateAtKey]
		if !ok {
			continue
		}
		invalidateAt, err := time.Parse(metaTimeFormat, tm)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("parse invalidate_at time: %w", err))
		}
		if invalidateAt.Before(l.now()) {
			if err = l.Store.Remove(ctx, file.Key); err != nil {
				errs = multierror.Append(err, fmt.Errorf("remove file under key %q: %w", file.Key, err))
				continue
			}
			invalidated++
		}
		l.Log.Printf("[DEBUG] removed file with key %q", file.Key)
	}

	return invalidated, errs.ErrorOrNil()
}

func (l *LoadingCache) extendTTL(ctx context.Context, key string, ttl time.Duration, meta FileMeta) (FileMeta, error) {
	if !l.ExtendTTL {
		return meta, nil
	}

	if meta.Meta == nil {
		meta.Meta = map[string]string{}
	}

	v, ok := meta.Meta[metaInvalidateAtKey]
	if !ok {
		meta.Meta[metaInvalidateAtKey] = l.now().Add(ttl).Format(metaTimeFormat)
	}

	tm, err := time.Parse(metaTimeFormat, v)
	if err != nil {
		return meta, fmt.Errorf("parse invalidate_at time: %w", err)
	}

	meta.Meta[metaInvalidateAtKey] = tm.Add(ttl).Format(metaTimeFormat)

	if err = l.Store.UpdateMeta(ctx, key, meta); err != nil {
		return meta, fmt.Errorf("update file meta: %w", err)
	}

	return meta, nil
}

type tempFile struct{ *os.File }

func (t *tempFile) Close() error {
	if err := t.File.Close(); err != nil {
		return fmt.Errorf("close file: %w", err)
	}
	if err := os.Remove(t.Name()); err != nil {
		return fmt.Errorf("remove file: %w", err)
	}
	return nil
}
