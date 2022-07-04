package fcache

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/minio/minio-go/v7"
	"golang.org/x/sync/errgroup"
)

//go:generate rm -f s3_mock.go
//go:generate moq -out s3_mock.go -fmt goimports . s3client

type s3client interface {
	PutObject(ctx context.Context, bkt, key string, rd io.Reader, sz int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
	GetObject(ctx context.Context, bkt, key string, opts minio.GetObjectOptions) (*minio.Object, error)
	RemoveObject(ctx context.Context, bkt, key string, opts minio.RemoveObjectOptions) error
	ListObjects(ctx context.Context, bkt string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo
	StatObject(ctx context.Context, bkt, key string, opts minio.StatObjectOptions) (minio.ObjectInfo, error)
	PresignHeader(
		ctx context.Context,
		method, bkt, key string,
		expires time.Duration,
		reqParams url.Values,
		extraHeaders http.Header,
	) (u *url.URL, err error)
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
func (s *S3) GetFile(ctx context.Context, key string, ld Loader) (io.ReadCloser, FileMeta, error) {
	var errResp minio.ErrorResponse

	obj, err := s.cl.GetObject(ctx, s.bucket, s.key(key), minio.GetObjectOptions{})
	if err == nil {
		// cache hit
		atomic.AddInt64(&s.Hits, 1)

		oi, err := s.getObjectInfo(obj)
		if err != nil {
			atomic.AddInt64(&s.Errors, 1)
			return nil, FileMeta{}, fmt.Errorf("get object info: %w", err)
		}

		return obj, s.objectInfoToFile(oi), nil
	}

	if err != nil && !(errors.As(err, &errResp) && errResp.StatusCode == http.StatusNotFound) {
		// s3 returned unexpected error
		atomic.AddInt64(&s.Errors, 1)
		return nil, FileMeta{}, fmt.Errorf("get file from s3: %w", err)
	}

	// miss
	atomic.AddInt64(&s.Misses, 1)

	rd, file, err := s.put(ctx, s.key(key), ld, true)
	if err != nil {
		atomic.AddInt64(&s.Errors, 1)
		return nil, file, fmt.Errorf("put file to s3: %w", err)
	}

	return rd, file, nil
}

// GetURL returns the URL from the cache backend.
func (s *S3) GetURL(ctx context.Context, key string, expires time.Duration, ld Loader) (string, FileMeta, error) {
	var errResp minio.ErrorResponse

	getURL := func(file FileMeta) (string, FileMeta, error) {
		u, err := s.cl.PresignHeader(ctx, http.MethodGet, s.bucket, s.key(key), expires, url.Values{},
			http.Header{"Content-Disposition": []string{fmt.Sprintf("attachment; filename=%s", file.Name)}})
		if err != nil {
			atomic.AddInt64(&s.Errors, 1)
			return "", FileMeta{}, fmt.Errorf("get presigned URL from s3")
		}
		return u.String(), file, nil
	}

	oi, err := s.cl.StatObject(ctx, s.bucket, s.key(key), minio.StatObjectOptions{})
	if err == nil {
		// cache hit
		atomic.AddInt64(&s.Hits, 1)
		return getURL(s.objectInfoToFile(oi))
	}

	if err != nil && !(errors.As(err, &errResp) && errResp.StatusCode == http.StatusNotFound) {
		// s3 returned unexpected error
		atomic.AddInt64(&s.Errors, 1)
		return "", FileMeta{}, fmt.Errorf("get presigned URL from s3: %w", err)
	}

	// miss
	atomic.AddInt64(&s.Misses, 1)

	_, file, err := s.put(ctx, s.key(key), ld, false)
	if err != nil {
		atomic.AddInt64(&s.Errors, 1)
		return "", FileMeta{}, fmt.Errorf("put file to s3: %w", err)
	}

	return getURL(file)
}

// Stat returns cache stats.
func (s *S3) Stat(ctx context.Context) (Stats, error) {
	var (
		res = Stats{
			Hits:   s.Hits,
			Misses: s.Misses,
			Errors: s.Errors,
		}
		err error
	)

	if res.Keys, res.Size, err = s.calcStats(ctx); err != nil {
		return res, fmt.Errorf("calc stats: %w", err)
	}

	return res, nil
}

func (s *S3) calcStats(ctx context.Context) (keys int, size int64, err error) {
	ch := s.cl.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: s.key("")})

	for obj := range ch {
		if obj.Err != nil {
			return 0, 0, fmt.Errorf("list objects: %w", obj.Err)
		}

		size += obj.Size
		keys++
	}

	return keys, size, nil
}

// Keys returns all keys, present in cache.
func (s *S3) Keys(ctx context.Context) ([]string, error) {
	ch := s.cl.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: s.key("")})

	var res []string
	for obj := range ch {
		if obj.Err != nil {
			return nil, fmt.Errorf("list objects: %w", obj.Err)
		}

		res = append(res, obj.Key)
	}

	return res, nil
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

// load and put the object into storage, if copy is set to false - it simply
// returns emptied file reader
func (s *S3) put(ctx context.Context, key string, ld Loader, copy bool) (rd io.ReadCloser, file FileMeta, err error) {
	pipeRd, pipeWr := io.Pipe()
	rd = pipeRd

	// duplicating reader to still return file content, when reader is emptied
	// fixme: probably this part needs to be limited, or file should be saved in
	// tmp, so a limited amount of files would be in memory
	putRd := io.Reader(rd)
	if copy {
		buf := &bytes.Buffer{}
		putRd = io.TeeReader(rd, buf)
		rd = io.NopCloser(buf)
	}

	fileWr, file, err := ld(ctx)
	if err != nil {
		return nil, file, fmt.Errorf("loader returned error: %w", err)
	}

	ewg, ctx := errgroup.WithContext(ctx)

	ewg.Go(func() error {
		if _, lderr := fileWr.WriteTo(pipeWr); lderr != nil {
			pipeWr.CloseWithError(lderr)
			return fmt.Errorf("write file to pipe: %w", lderr)
		}
		pipeWr.Close()
		return nil
	})

	ewg.Go(func() error {
		_, perr := s.cl.PutObject(ctx, s.bucket, key, putRd, file.Size, minio.PutObjectOptions{
			UserMetadata: map[string]string{"X-Amz-Meta-Filename": file.Name},
		})
		if perr != nil {
			pipeRd.CloseWithError(perr)
			return fmt.Errorf("put file in s3: %w", perr)
		}
		pipeRd.Close()
		return nil
	})

	if err = ewg.Wait(); err != nil {
		return nil, file, fmt.Errorf("load and put file: %w", err)
	}

	return rd, file, nil
}

func (s *S3) key(key string) string {
	if s.prefix == "" {
		return key
	}
	return fmt.Sprintf("%s!!%s", s.prefix, key)
}

func (s *S3) objectInfoToFile(oi minio.ObjectInfo) FileMeta {
	return FileMeta{
		Name:        oi.Metadata.Get("X-Amz-Meta-Filename"),
		ContentType: oi.ContentType,
		Size:        oi.Size,
	}
}
