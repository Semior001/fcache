package fcache

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/minio/minio-go/v7"
)

//go:generate moq -out s3_mock.go -fmt goimports . s3client

type s3client interface {
	PutObject(ctx context.Context, bkt, key string, rd io.Reader, sz int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
	GetObject(ctx context.Context, bkt, key string, opts minio.GetObjectOptions) (*minio.Object, error)
	RemoveObject(ctx context.Context, bkt, key string, opts minio.RemoveObjectOptions) error
	ListObjects(ctx context.Context, bkt string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo
}

// S3 implements Cache for S3.
type S3 struct {
	Options
	Stats

	cl s3client

	bucket string
	prefix string

	// mockable fields
	now           func() time.Time
	getObjectInfo func(*minio.Object) (minio.ObjectInfo, error)
}

// NewS3 makes new instance of S3.
func NewS3(ctx context.Context, backend *minio.Client, bucket, prefix string, opts ...Option) *S3 {
	res := &S3{
		Options:       Options{TTL: 30 * time.Minute, Log: stdLogger{}},
		cl:            backend,
		now:           time.Now,
		getObjectInfo: (*minio.Object).Stat,
	}
	for _, opt := range opts {
		opt(&res.Options)
	}

	if res.Options.InvalidatePeriod > 0 {
		go res.run(ctx)
	}

	return res
}

// GetFile gets the file from cache or loads it, if absent.
func (s *S3) GetFile(ctx context.Context, key string, fn func() (File, error)) (File, error) {
	var errResp minio.ErrorResponse

	obj, err := s.cl.GetObject(ctx, s.bucket, s.key(key), minio.GetObjectOptions{})
	if err == nil {
		// cache hit
		atomic.AddInt64(&s.Hits, 1)

		file, err := s.objectToFile(obj)
		if err != nil {
			atomic.AddInt64(&s.Errors, 1)
			return File{}, fmt.Errorf("convert object to file: %w", err)
		}

		return file, nil
	}

	if err != nil && !(errors.As(err, &errResp) && errResp.StatusCode == http.StatusNotFound) {
		// s3 returned unexpected error
		atomic.AddInt64(&s.Errors, 1)
		return File{}, fmt.Errorf("get file from s3: %w", err)
	}

	// miss
	atomic.AddInt64(&s.Misses, 1)

	file, err := s.put(ctx, s.key(key), fn)
	if err != nil {
		atomic.AddInt64(&s.Errors, 1)
		return file, fmt.Errorf("put file to s3: %w", err)
	}

	return file, nil
}

// GetURL returns the URL from the cache backend.
func (s *S3) GetURL(ctx context.Context, key string, fn func() (File, error)) (string, error) {
	return "", nil
}

// Stat returns cache stats.
func (s *S3) Stat(ctx context.Context) (Stats, error) {
	panic("not implemented") // TODO: Implement
}

// Keys returns all keys, present in cache.
func (s *S3) Keys(ctx context.Context) ([]string, error) {
	panic("not implemented") // TODO: Implement
}

// run runs invalidation goroutine
func (s *S3) run(ctx context.Context) {
	ticker := time.NewTimer(s.InvalidatePeriod)
	for {
		select {
		case <-ticker.C:
			if err := s.invalidate(ctx); err != nil {
				s.Log.Printf("[WARN] failed to invalidate s3 cache items: %v", err)
			}
		case <-ctx.Done():
			s.Log.Printf("[WARN] s3 cache invalidating goroutine stopped")
			return
		}
	}
}

// invalidates expired items
func (s *S3) invalidate(ctx context.Context) error {
	ch := s.cl.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{WithMetadata: true})
	for obj := range ch {
		if obj.Err != nil {
			return fmt.Errorf("list s3 objects: %w", obj.Err)
		}

		if s.now().Add(s.TTL).Before(obj.LastModified) {
			if err := s.cl.RemoveObject(ctx, s.bucket, obj.Key, minio.RemoveObjectOptions{}); err != nil {
				return fmt.Errorf("remove s3 object %s: %w", obj.Key, err)
			}
			s.Log.Printf("[DEBUG] removed object with key %q", obj.Key)
		}
	}
	return nil
}

func (s *S3) put(ctx context.Context, key string, fn func() (File, error)) (File, error) {
	file, err := fn()
	if err != nil {
		return file, err
	}

	// duplicating reader to still return file content, when reader is emptied
	// fixme: probably this part needs to be limited, or file should be saved in
	// tmp, so a limited amount of files would be in memory
	buf := &bytes.Buffer{}
	tr := io.TeeReader(file.Reader, buf)
	file.Reader = io.NopCloser(buf)

	_, err = s.cl.PutObject(ctx, s.bucket, key, tr, file.Size, minio.PutObjectOptions{
		UserMetadata: map[string]string{"X-Amz-Meta-Filename": file.Name},
	})
	if err != nil {
		return file, fmt.Errorf("put file in s3: %w", err)
	}

	return file, nil
}

func (s *S3) key(key string) string {
	if s.prefix == "" {
		return key
	}
	return fmt.Sprintf("%s!!%s", s.prefix, key)
}

func (s *S3) objectToFile(obj *minio.Object) (File, error) {
	stat, err := s.getObjectInfo(obj)
	if err != nil {
		return File{}, fmt.Errorf("get stat: %w", err)
	}
	return File{
		Name:        stat.Metadata.Get("X-Amz-Meta-Filename"),
		ContentType: stat.ContentType,
		Reader:      obj,
		Size:        stat.Size,
	}, nil
}
