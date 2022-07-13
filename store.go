package fcache

import (
	"context"
	"errors"
	"io"
	"time"
)

// ErrNotFound represents a not found error.
var ErrNotFound = errors.New("not found")

//go:generate rm -f store_mock.go
//go:generate moq -out store_mock.go -fmt goimports . Store

// GetURLParams describes additional parameters, besides key, to form a signed URL.
type GetURLParams struct {
	Filename string
	Expires  time.Duration
}

// Store defines methods that the backend store should implement
type Store interface {
	Meta(ctx context.Context, key string) (FileMeta, error)
	Get(ctx context.Context, key string) (rd io.ReadCloser, err error)
	GetURL(ctx context.Context, key string, params GetURLParams) (url string, err error)
	Put(ctx context.Context, key string, meta FileMeta, rd io.ReadCloser) error
	Remove(ctx context.Context, key string) error
	Stat(ctx context.Context) (StoreStats, error)
	Keys(ctx context.Context) ([]string, error)
	List(ctx context.Context) ([]FileMeta, error)
}

// StoreStats represents stats of the backend store.
type StoreStats struct {
	Keys int
	Size int64
}

// FileMeta represent information about the file.
type FileMeta struct {
	Name string
	Mime string
	// Size might not be provided when loading file, though it might be useful
	// for some cache implementations, like S3 as it runs streaming multipart
	// method, if size is provided
	Size int64

	// store stat fields
	Key       string
	CreatedAt time.Time
}
