package fcache

import (
	"context"
	"errors"
	"io"
)

// ErrNotFound represents a Not Found error.
var ErrNotFound = errors.New("file not found in cache")

// Cache defines methods to store and return cached values.
type Cache interface {
	// GetFile gets the file from cache or loads it, if absent.
	GetFile(ctx context.Context, key string, fn func() (File, error)) (File, error)
	// GetURL returns the URL from the cache backend.
	GetURL(ctx context.Context, key string, fn func() (File, error)) (string, error)
	// Stat returns cache stats.
	Stat(ctx context.Context) (Stats, error)
	// Keys returns all keys, present in cache.
	Keys(ctx context.Context) ([]string, error)
}

// File represent file metadata and its content via Reader.
type File struct {
	Name        string
	ContentType string
	Reader      io.ReadCloser
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
