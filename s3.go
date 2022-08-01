package fcache

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
)

//go:generate rm -f s3_mock.go
//go:generate moq -out s3_mock.go -fmt goimports . s3client

const filenameMetaHeader = "_fcache-S3-Meta-Filename"

type s3client interface {
	PutObject(ctx context.Context, bkt, key string, rd io.Reader, sz int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
	GetObject(ctx context.Context, bkt, key string, opts minio.GetObjectOptions) (*minio.Object, error)
	RemoveObject(ctx context.Context, bkt, key string, opts minio.RemoveObjectOptions) error
	ListObjects(ctx context.Context, bkt string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo
	StatObject(ctx context.Context, bkt, key string, opts minio.StatObjectOptions) (minio.ObjectInfo, error)
	PresignedGetObject(
		ctx context.Context,
		bkt, key string,
		expires time.Duration,
		reqParams url.Values,
	) (u *url.URL, err error)
}

// S3 implements Cache for S3.
type S3 struct {
	log Logger
	cl  s3client

	bucket string
	prefix string
}

// NewS3 makes new instance of S3.
func NewS3(cl *minio.Client, bucket, prefix string, log Logger) *S3 {
	return &S3{
		log:    log,
		cl:     cl,
		bucket: bucket,
		prefix: prefix,
	}
}

// Meta returns meta information about the file at underlying key.
func (s *S3) Meta(ctx context.Context, key string) (FileMeta, error) {
	var errResp minio.ErrorResponse

	oi, err := s.cl.StatObject(ctx, s.bucket, s.key(key), minio.StatObjectOptions{})
	if errors.As(err, &errResp) && errResp.StatusCode == http.StatusNotFound {
		return FileMeta{}, ErrNotFound
	}
	if err != nil {
		return FileMeta{}, fmt.Errorf("s3 returned error: %w", err)
	}

	return s.objectInfoToFile(oi), nil
}

// Get gets the file from cache or loads it, if absent.
func (s *S3) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	var errResp minio.ErrorResponse

	obj, err := s.cl.GetObject(ctx, s.bucket, s.key(key), minio.GetObjectOptions{})
	if errors.As(err, &errResp) && errResp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("s3 returned error: %w", err)
	}

	return obj, nil
}

// GetURL returns the URL from the cache backend.
func (s *S3) GetURL(ctx context.Context, key string, params GetURLParams) (string, error) {
	var errResp minio.ErrorResponse

	oi, err := s.cl.StatObject(ctx, s.bucket, s.key(key), minio.StatObjectOptions{})
	if errors.As(err, &errResp) && errResp.StatusCode == http.StatusNotFound {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("stat object from s3: %w", err)
	}

	filename := s.objectInfoToFile(oi).Name
	if params.Filename != "" {
		filename = params.Filename
	}

	u, err := s.cl.PresignedGetObject(ctx, s.bucket, s.key(key), params.Expires, url.Values{
		"response-content-disposition": []string{fmt.Sprintf("attachment; filename=%s", filename)},
	})
	if err != nil {
		return "", fmt.Errorf("get presigned URL from s3")
	}
	return u.String(), nil
}

// Put puts file into S3.
func (s *S3) Put(ctx context.Context, key string, meta FileMeta, rd io.ReadCloser) error {
	defer func() {
		if err := rd.Close(); err != nil {
			s.log.Printf("[WARN] failed to close reader: %v", err)
		}
	}()

	if meta.Meta == nil {
		meta.Meta = map[string]string{}
	}
	meta.Meta[filenameMetaHeader] = meta.Name

	_, perr := s.cl.PutObject(ctx, s.bucket, s.key(key), rd, meta.Size, minio.PutObjectOptions{
		ContentType:  meta.Mime,
		UserMetadata: meta.Meta,
	})
	if perr != nil {
		return fmt.Errorf("put file in s3: %w", perr)
	}
	return nil
}

// Remove removes file by its key.
func (s *S3) Remove(ctx context.Context, key string) error {
	var errResp minio.ErrorResponse

	err := s.cl.RemoveObject(ctx, s.bucket, s.key(key), minio.RemoveObjectOptions{})
	if errors.As(err, &errResp) && errResp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("s3 returned")
	}

	return nil
}

// Stat returns cache stats.
func (s *S3) Stat(ctx context.Context) (res StoreStats, err error) {
	ch := s.cl.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: s.key("")})

	for obj := range ch {
		if obj.Err != nil {
			return res, fmt.Errorf("list objects: %w", obj.Err)
		}

		res.Size += obj.Size
		res.Keys++
	}

	return res, nil
}

// List lists object in S3 bucket.
func (s *S3) List(ctx context.Context) ([]FileMeta, error) {
	var result []FileMeta

	ch := s.cl.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{WithMetadata: true, Prefix: s.key("")})
	for obj := range ch {
		if obj.Err != nil {
			return nil, fmt.Errorf("s3 returned error: %w", obj.Err)
		}
		result = append(result, s.objectInfoToFile(obj))
	}

	return result, nil
}

// Keys returns all keys, present in cache.
func (s *S3) Keys(ctx context.Context) ([]string, error) {
	ch := s.cl.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: s.key("")})

	var res []string
	for obj := range ch {
		if obj.Err != nil {
			return nil, fmt.Errorf("list objects: %w", obj.Err)
		}

		res = append(res, s.parseKey(obj.Key))
	}

	return res, nil
}

func (s *S3) key(key string) string {
	if s.prefix == "" {
		return key
	}
	return fmt.Sprintf("%s!!%s", s.prefix, key)
}

func (s *S3) parseKey(key string) string {
	if s.prefix == "" {
		return key
	}
	tkns := strings.Split(key, "!!")
	if len(tkns) != 2 {
		return key
	}
	return tkns[1]
}

func (s *S3) objectInfoToFile(oi minio.ObjectInfo) FileMeta {
	return FileMeta{
		Name: oi.Metadata.Get(filenameMetaHeader),
		Mime: oi.ContentType,
		Size: oi.Size,
		Meta: oi.UserMetadata,
		Key:  s.parseKey(oi.Key),
		// s3 maintains only last modified date, this implementation assumes
		// that files are untouched by external forces
		CreatedAt: oi.LastModified,
	}
}
