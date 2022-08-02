package fcache

import (
	"context"
	"io"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestS3_Meta(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		now := time.Now()
		svc := &S3{
			cl: &s3clientMock{
				StatObjectFunc: func(ctx context.Context, bkt, key string, opts minio.GetObjectOptions) (minio.ObjectInfo, error) {
					assert.Equal(t, "bucket", bkt)
					assert.Equal(t, "prefix!!key", key)
					assert.Empty(t, opts)
					return minio.ObjectInfo{
						UserMetadata: map[string]string{filenameMetaHeader: "a.txt"},
						ContentType:  "text/plain",
						Size:         123,
						LastModified: now,
						Key:          "prefix!!key",
					}, nil
				},
			},
			bucket: "bucket",
			prefix: "prefix",
		}

		meta, err := svc.Meta(context.Background(), "key")
		require.NoError(t, err)
		assert.Equal(t, FileMeta{
			Name:      "a.txt",
			Mime:      "text/plain",
			Meta:      map[string]string{},
			Size:      123,
			Key:       "key",
			CreatedAt: now,
		}, meta)
	})
}

func TestS3_Get(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		obj := &minio.Object{}
		svc := &S3{
			bucket: "bucket",
			prefix: "prefix",
			cl: &s3clientMock{
				GetObjectFunc: func(ctx context.Context,
					bkt, key string,
					opts minio.GetObjectOptions,
				) (*minio.Object, error) {
					assert.Equal(t, "bucket", bkt)
					assert.Equal(t, "prefix!!key", key)
					assert.Empty(t, opts)
					return obj, nil
				},
			},
		}
		ro, err := svc.Get(context.Background(), "key")
		require.NoError(t, err)
		assert.True(t, obj == ro)
	})
}

func TestS3_GetURL(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		now := time.Now()
		svc := &S3{
			cl: &s3clientMock{
				StatObjectFunc: func(ctx context.Context, bkt, key string, opts minio.GetObjectOptions) (minio.ObjectInfo, error) {
					assert.Equal(t, "bucket", bkt)
					assert.Equal(t, "prefix!!key", key)
					assert.Empty(t, opts)
					return minio.ObjectInfo{
						UserMetadata: map[string]string{filenameMetaHeader: "a.txt"},
						ContentType:  "text/plain",
						Size:         123,
						LastModified: now,
						Key:          "prefix!!key",
					}, nil
				},
				PresignedGetObjectFunc: func(ctx context.Context,
					bkt, key string,
					expires time.Duration,
					reqParams url.Values,
				) (*url.URL, error) {
					assert.Equal(t, "bucket", bkt)
					assert.Equal(t, "prefix!!key", key)
					assert.Equal(t, 15*time.Minute, expires)
					assert.Equal(t, url.Values{
						"response-content-disposition": []string{"attachment; filename=a.txt"},
					}, reqParams)
					return url.Parse("https://example.com/somefile.txt?somekey=somevalue")
				},
			},
			bucket: "bucket",
			prefix: "prefix",
		}

		meta, err := svc.GetURL(context.Background(), "key", GetURLParams{Expires: 15 * time.Minute})
		require.NoError(t, err)
		assert.Equal(t, "https://example.com/somefile.txt?somekey=somevalue", meta)
	})
}

func TestS3_Put(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &S3{
			cl: &s3clientMock{
				PutObjectFunc: func(ctx context.Context,
					bkt, key string,
					rd io.Reader, sz int64,
					opts minio.PutObjectOptions,
				) (minio.UploadInfo, error) {
					assert.Equal(t, "bucket", bkt)
					assert.Equal(t, "prefix!!key", key)
					assert.Equal(t, int64(17), sz)
					assert.Equal(t, minio.PutObjectOptions{
						ContentType:  "text/plain",
						UserMetadata: map[string]string{filenameMetaHeader: "a.txt"},
					}, opts)

					bts, err := io.ReadAll(rd)
					require.NoError(t, err)
					assert.Equal(t, []byte("some file data"), bts)

					return minio.UploadInfo{}, nil
				},
			},
			bucket: "bucket",
			prefix: "prefix",
		}

		err := svc.Put(context.Background(), "key", FileMeta{
			Name: "a.txt",
			Mime: "text/plain",
			Size: 17,
		}, io.NopCloser(strings.NewReader("some file data")))
		require.NoError(t, err)
	})
}

func TestS3_Remove(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &S3{
			cl: &s3clientMock{
				RemoveObjectFunc: func(ctx context.Context,
					bkt, key string,
					opts minio.RemoveObjectOptions,
				) error {
					assert.Equal(t, "bucket", bkt)
					assert.Equal(t, "prefix!!key", key)
					assert.Empty(t, opts)
					return nil
				},
			},
			bucket: "bucket",
			prefix: "prefix",
		}
		err := svc.Remove(context.Background(), "key")
		require.NoError(t, err)
	})
}

func TestS3_Stat(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &S3{
			cl: &s3clientMock{
				ListObjectsFunc: func(ctx context.Context, bkt string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
					assert.Equal(t, minio.ListObjectsOptions{Prefix: "prefix!!"}, opts)
					assert.Equal(t, "bucket", bkt)
					ch := make(chan minio.ObjectInfo, 2)
					ch <- minio.ObjectInfo{Size: 12}
					ch <- minio.ObjectInfo{Size: 16}
					close(ch)
					return ch
				},
			},
			bucket: "bucket",
			prefix: "prefix",
		}
		stat, err := svc.Stat(context.Background())
		require.NoError(t, err)
		assert.Equal(t, StoreStats{Keys: 2, Size: 28}, stat)
	})
}

func TestS3_List(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		now := time.Now()
		svc := &S3{
			cl: &s3clientMock{
				ListObjectsFunc: func(ctx context.Context, bkt string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
					assert.Equal(t, minio.ListObjectsOptions{WithMetadata: true, Prefix: "prefix!!"}, opts)
					assert.Equal(t, "bucket", bkt)
					ch := make(chan minio.ObjectInfo, 2)
					ch <- minio.ObjectInfo{
						UserMetadata: map[string]string{filenameMetaHeader: "a.txt"},
						ContentType:  "text/plain",
						Size:         12,
						Key:          "prefix!!key",
						LastModified: now,
					}
					ch <- minio.ObjectInfo{
						UserMetadata: map[string]string{filenameMetaHeader: "b.txt"},
						ContentType:  "text/plain",
						Size:         16,
						Key:          "prefix!!key-1",
						LastModified: now.Add(15 * time.Minute),
					}
					close(ch)
					return ch
				},
			},
			bucket: "bucket",
			prefix: "prefix",
		}
		objs, err := svc.List(context.Background())
		require.NoError(t, err)
		assert.Equal(t, []FileMeta{
			{
				Name:      "a.txt",
				Mime:      "text/plain",
				Meta:      map[string]string{},
				Size:      12,
				Key:       "key",
				CreatedAt: now,
			},
			{
				Name:      "b.txt",
				Mime:      "text/plain",
				Meta:      map[string]string{},
				Size:      16,
				Key:       "key-1",
				CreatedAt: now.Add(15 * time.Minute),
			},
		}, objs)
	})
}

func TestS3_Keys(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &S3{
			cl: &s3clientMock{
				ListObjectsFunc: func(ctx context.Context, bkt string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
					assert.Equal(t, minio.ListObjectsOptions{Prefix: "prefix!!"}, opts)
					assert.Equal(t, "bucket", bkt)
					ch := make(chan minio.ObjectInfo, 2)
					ch <- minio.ObjectInfo{Key: "prefix!!key-1"}
					ch <- minio.ObjectInfo{Key: "prefix!!key-2"}
					close(ch)
					return ch
				},
			},
			bucket: "bucket",
			prefix: "prefix",
		}
		keys, err := svc.Keys(context.Background())
		require.NoError(t, err)
		assert.Equal(t, []string{"key-1", "key-2"}, keys)
	})
}
