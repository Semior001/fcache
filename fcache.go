package fcache

import (
	"context"
	"io"
)

// Loader is a function to load a file in case if it's missing in cache.
// WriterTo accepts io.Pipe, and WriteTo will be called in a goroutine, thus
// the content of the file will be effectively copied directly to the storage.
// The number of bytes written, returned by WriterTo won't be used, as most
// implementations, file size in FileMeta will be used instead.
type Loader func(ctx context.Context) (io.WriterTo, FileMeta, error)

// WriterToFunc is an adapter, to use ordinary functions as io.WriterTo.
type WriterToFunc func(w io.Writer) (n int64, err error)

// WriteTo implements io.WriterTo.
func (f WriterToFunc) WriteTo(w io.Writer) (n int64, err error) { return f(w) }

// Cache defines methods to store and return cached values.
type Cache interface {
	// GetFile gets the file from cache or loads it, if absent.
	GetFile(ctx context.Context, key string, fn Loader) (rd io.ReadCloser, meta FileMeta, err error)
	// GetURL returns the URL from the cache backend.
	GetURL(ctx context.Context, key string, fn Loader) (url string, meta FileMeta, err error)
	// Stat returns cache stats.
	Stat(ctx context.Context) (Stats, error)
	// Keys returns all keys, present in cache.
	Keys(ctx context.Context) ([]string, error)
}

// FileMeta represent information about the file.
type FileMeta struct {
	Name        string
	ContentType string
	// Size might not be provided when loading file, though it might be useful
	// for some cache implementations, like S3 as it runs streaming multipart
	// method, if size is provided
	Size int64
}

// Stats represent stat values.
type Stats struct {
	Hits   int64
	Misses int64
	Keys   int
	Size   int64
	Errors int64
}
