package fcache

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestS3_GetFile(t *testing.T) {
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

	// hit
	file, err := svc.GetFile(context.Background(), "key", nil)
	require.NoError(t, err)
	assert.Equal(t, File{
		Name:        "a.txt",
		ContentType: "text/plain",
		Reader:      expectedObj,
		Size:        15,
	}, file)
	assert.True(t, expectedObj == file.Reader, "pointer to s3 object")
	assert.Equal(t, Stats{Hits: 1}, svc.Stats)

	// error
	expectedErr := errors.New("some error")
	client.GetObjectFunc = func(context.Context, string, string, minio.GetObjectOptions) (*minio.Object, error) {
		return nil, expectedErr
	}

	file, err = svc.GetFile(context.Background(), "key", nil)
	assert.ErrorIs(t, err, expectedErr)
	assert.Empty(t, file)
	assert.Equal(t, Stats{Hits: 1, Errors: 1}, svc.Stats)

	// miss
	client.GetObjectFunc = func(context.Context, string, string, minio.GetObjectOptions) (*minio.Object, error) {
		return nil, minio.ErrorResponse{StatusCode: http.StatusNotFound}
	}
	client.PutObjectFunc = func(ctx context.Context,
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
	}
	file, err = svc.GetFile(context.Background(), "key", func() (File, error) {
		return File{
			Name:        "a.txt",
			ContentType: "plain/text",
			Reader:      io.NopCloser(strings.NewReader("some file content")),
			Size:        17,
		}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, "a.txt", file.Name)
	assert.Equal(t, "plain/text", file.ContentType)
	assert.Equal(t, int64(17), file.Size)

	bts, err := io.ReadAll(file.Reader)
	require.NoError(t, err)
	assert.Equal(t, "some file content", string(bts))

	assert.Equal(t, Stats{Hits: 1, Errors: 1, Misses: 1}, svc.Stats)
}
