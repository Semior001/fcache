package fcache

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"time"
	"net/url"
)

func TestS3_GetFile(t *testing.T) {
	t.Run("hit", func(t *testing.T) {
		expectedObj := &minio.Object{}
		client := &s3clientMock{
			GetObjectFunc: func(ctx context.Context,
				bkt, key string,
				opts minio.GetObjectOptions,
			) (*minio.Object, error) {
				assert.Equal(t, "bucket", bkt)
				assert.Equal(t, "prefix!!key", key)
				assert.Equal(t, minio.GetObjectOptions{}, opts)
				return expectedObj, nil
			},
		}
		svc := &S3{
			bucket: "bucket",
			prefix: "prefix",
			cl:     client,
			getObjectInfo: func(obj *minio.Object) (minio.ObjectInfo, error) {
				assert.True(t, expectedObj == obj, "pointer to s3 object")
				return minio.ObjectInfo{
					Metadata:    http.Header{"X-Amz-Meta-Filename": []string{"a.txt"}},
					ContentType: "text/plain",
					Size:        15,
				}, nil
			},
		}

		rd, file, err := svc.GetFile(context.Background(), "key", nil)
		require.NoError(t, err)
		assert.Equal(t, FileMeta{
			Name:        "a.txt",
			ContentType: "text/plain",
			Size:        15,
		}, file)
		assert.True(t, expectedObj == rd, "pointer to s3 object")
		assert.Equal(t, Stats{Hits: 1}, svc.Stats)
	})

	t.Run("error", func(t *testing.T) {
		expectedErr := errors.New("some error")
		client := &s3clientMock{
			GetObjectFunc: func(context.Context, string, string, minio.GetObjectOptions) (*minio.Object, error) {
				return nil, expectedErr
			},
		}
		svc := &S3{
			bucket: "bucket",
			prefix: "prefix",
			cl:     client,
		}

		rd, file, err := svc.GetFile(context.Background(), "key", nil)
		assert.ErrorIs(t, err, expectedErr)
		assert.Empty(t, file)
		assert.Nil(t, rd)
		assert.Equal(t, Stats{Errors: 1}, svc.Stats)
	})

	t.Run("miss", func(t *testing.T) {
		client := &s3clientMock{
			GetObjectFunc: func(context.Context, string, string, minio.GetObjectOptions) (*minio.Object, error) {
				return nil, minio.ErrorResponse{StatusCode: http.StatusNotFound}
			},
			PutObjectFunc: func(ctx context.Context,
				bkt, key string, rd io.Reader, sz int64,
				opts minio.PutObjectOptions,
			) (minio.UploadInfo, error) {
				assert.Equal(t, "bucket", bkt)
				assert.Equal(t, "prefix!!key", key)
				bts, err := io.ReadAll(rd)
				require.NoError(t, err)
				assert.Equal(t, "some file content", string(bts))
				assert.Equal(t, int64(17), sz)
				assert.Equal(t, minio.PutObjectOptions{
					UserMetadata: map[string]string{
						"X-Amz-Meta-Filename": "a.txt",
					},
				}, opts)
				return minio.UploadInfo{}, nil
			},
		}
		svc := &S3{
			bucket: "bucket",
			prefix: "prefix",
			cl:     client,
		}
		rd, file, err := svc.GetFile(context.Background(), "key", func(ctx context.Context) (io.WriterTo, FileMeta, error) {
			return WriterToFunc(func(w io.Writer) (int64, error) {
					n, err := w.Write([]byte("some file content"))
					return int64(n), err
				}), FileMeta{
					Name:        "a.txt",
					ContentType: "text/plain",
					Size:        17,
				}, nil
		})
		require.NoError(t, err)
		assert.Equal(t, "a.txt", file.Name)
		assert.Equal(t, "text/plain", file.ContentType)
		assert.Equal(t, int64(17), file.Size)

		bts, err := io.ReadAll(rd)
		require.NoError(t, err)
		assert.Equal(t, "some file content", string(bts))

		assert.Equal(t, Stats{Misses: 1}, svc.Stats)
	})
}

func TestS3_GetURL(t *testing.T) {
	t.Run("hit", func(t *testing.T) {
		client := &s3clientMock{
			PresignHeaderFunc: func(ctx context.Context,
				mtd, bkt, key string,
				expires time.Duration,
				reqParams url.Values,
				extraHeaders http.Header,
			) (*url.URL, error) {
				assert.Equal(t, http.MethodGet, mtd)
				assert.Equal(t, "bucket", bkt)
				assert.Equal(t, "prefix!!key", key)
				assert.Equal(t, 5*time.Minute, expires)
				assert.Empty(t, reqParams)
				assert.Equal(t, http.Header{
					"Content-Disposition": []string{"attachment; filename=a.txt"},
				}, extraHeaders)
				return url.Parse("https://example.com/test/someurl")
			},
			StatObjectFunc: func(ctx context.Context,
				bkt, key string,
				opts minio.GetObjectOptions,
			) (minio.ObjectInfo, error) {
				assert.Equal(t, "bucket", bkt)
				assert.Equal(t, "prefix!!key", key)
				assert.Equal(t, minio.GetObjectOptions{}, opts)
				return minio.ObjectInfo{
					Metadata:    http.Header{"X-Amz-Meta-Filename": []string{"a.txt"}},
					ContentType: "text/plain",
					Size:        15,
				}, nil
			},
		}
		svc := &S3{
			bucket: "bucket",
			prefix: "prefix",
			cl:     client,
		}

		u, file, err := svc.GetURL(context.Background(), "key", 5*time.Minute, nil)
		require.NoError(t, err)
		assert.Equal(t, FileMeta{
			Name:        "a.txt",
			ContentType: "text/plain",
			Size:        15,
		}, file)
		assert.Equal(t, "https://example.com/test/someurl", u, "pointer to s3 object")
		assert.Equal(t, Stats{Hits: 1}, svc.Stats)
	})

	t.Run("error", func(t *testing.T) {
		expectedErr := errors.New("some error")
		client := &s3clientMock{
			StatObjectFunc: func(ctx context.Context,
				bkt string, key string,
				opts minio.GetObjectOptions) (minio.ObjectInfo, error) {
				assert.Equal(t, "bucket", bkt)
				assert.Equal(t, "prefix!!key", key)
				assert.Empty(t, opts)
				return minio.ObjectInfo{}, expectedErr
			},
		}
		svc := &S3{
			bucket: "bucket",
			prefix: "prefix",
			cl:     client,
		}

		u, file, err := svc.GetURL(context.Background(), "key", 5*time.Minute, nil)
		assert.ErrorIs(t, err, expectedErr)
		assert.Empty(t, file)
		assert.Empty(t, u)
		assert.Equal(t, Stats{Errors: 1}, svc.Stats)
	})

	t.Run("miss", func(t *testing.T) {
		client := &s3clientMock{
			StatObjectFunc: func(ctx context.Context,
				bkt, key string,
				opts minio.GetObjectOptions,
			) (minio.ObjectInfo, error) {
				return minio.ObjectInfo{}, minio.ErrorResponse{StatusCode: http.StatusNotFound}
			},
			PresignHeaderFunc: func(ctx context.Context,
				mtd, bkt, key string,
				expires time.Duration,
				reqParams url.Values,
				extraHeaders http.Header,
			) (*url.URL, error) {
				assert.Equal(t, http.MethodGet, mtd)
				assert.Equal(t, "bucket", bkt)
				assert.Equal(t, "prefix!!key", key)
				assert.Equal(t, 5*time.Minute, expires)
				assert.Empty(t, reqParams)
				assert.Equal(t, http.Header{
					"Content-Disposition": []string{"attachment; filename=a.txt"},
				}, extraHeaders)
				return url.Parse("https://example.com/test/someurl")
			},
			PutObjectFunc: func(ctx context.Context,
				bkt, key string, rd io.Reader, sz int64,
				opts minio.PutObjectOptions,
			) (minio.UploadInfo, error) {
				assert.Equal(t, "bucket", bkt)
				assert.Equal(t, "prefix!!key", key)
				bts, err := io.ReadAll(rd)
				require.NoError(t, err)
				assert.Equal(t, "some file content", string(bts))
				assert.Equal(t, int64(17), sz)
				assert.Equal(t, minio.PutObjectOptions{
					UserMetadata: map[string]string{
						"X-Amz-Meta-Filename": "a.txt",
					},
				}, opts)
				return minio.UploadInfo{}, nil
			},
		}
		svc := &S3{
			bucket: "bucket",
			prefix: "prefix",
			cl:     client,
		}
		u, file, err := svc.GetURL(context.Background(), "key", 5*time.Minute,
			func(ctx context.Context) (io.WriterTo, FileMeta, error) {
				return WriterToFunc(func(w io.Writer) (int64, error) {
						n, err := w.Write([]byte("some file content"))
						return int64(n), err
					}), FileMeta{
						Name:        "a.txt",
						ContentType: "text/plain",
						Size:        17,
					}, nil
			})
		require.NoError(t, err)
		assert.Equal(t, "a.txt", file.Name)
		assert.Equal(t, "text/plain", file.ContentType)
		assert.Equal(t, int64(17), file.Size)

		assert.Equal(t, "https://example.com/test/someurl", u)

		assert.Equal(t, Stats{Misses: 1}, svc.Stats)
	})
}
