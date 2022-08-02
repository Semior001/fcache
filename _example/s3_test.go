package _example

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/caarlos0/env/v6"

	"github.com/Semior001/fcache"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type config struct {
	S3 struct {
		Endpoint        string `env:"ENDPOINT"`
		AccessKeyID     string `env:"ACCESS_KEY_ID"`
		SecretAccessKey string `env:"SECRET_ACCESS_KEY"`
		UseSSL          bool   `env:"USE_SSL"`
		Token           string `env:"TOKEN"`
		Region          string `env:"REGION"`
		Bucket          string `env:"BUCKET"`
		Prefix          string `env:"PREFIX"`
	} `envPrefix:"S3_"`
	Cache struct {
		TTL                time.Duration `env:"TTL"`
		InvalidationPeriod time.Duration `env:"INVALIDATION_PERIOD"`
	} `envPrefix:"CACHE_"`
	IntegrationTests bool `env:"INTEGRATION_TESTS"`
}

type S3Suite struct {
	cache    *fcache.LoadingCache
	s3Client *minio.Client
	cfg      config
	suite.Suite
}

func (s *S3Suite) SetupSuite() {
	ctx := context.Background()

	var err error
	s.s3Client, err = minio.New(s.cfg.S3.Endpoint, &minio.Options{
		Creds: credentials.NewStatic(
			s.cfg.S3.AccessKeyID,
			s.cfg.S3.SecretAccessKey,
			s.cfg.S3.Token,
			credentials.SignatureDefault,
		),
		Secure: s.cfg.S3.UseSSL,
		Region: s.cfg.S3.Region,
	})
	require.NoError(s.T(), err)

	s.cache = fcache.NewLoadingCache(
		fcache.NewS3(s.s3Client, s.cfg.S3.Bucket, s.cfg.S3.Prefix, (*tLogAdapter)(s.T())),
		fcache.WithLogger((*tLogAdapter)(s.T())),
		fcache.WithInvalidationPeriod(s.cfg.Cache.InvalidationPeriod),
	)

	b, err := s.s3Client.BucketExists(ctx, s.cfg.S3.Bucket)
	require.NoError(s.T(), err)
	require.Truef(s.T(), b, "test bucket %q should exist", s.cfg.S3.Bucket)
}

func (s *S3Suite) TearDownSuite() {
	// cleaning up created objects in the bucket
	objects := s.s3Client.ListObjects(context.Background(), s.cfg.S3.Bucket, minio.ListObjectsOptions{
		Prefix: s.cfg.S3.Prefix,
	})
	for obj := range objects {
		require.NoError(s.T(), obj.Err)
		require.NoError(s.T(), s.s3Client.RemoveObject(context.Background(),
			s.cfg.S3.Bucket, obj.Key,
			minio.RemoveObjectOptions{
				ForceDelete: true,
			}))
	}
}

func (s *S3Suite) TestIntegration() {
	ctx := context.Background()

	params := fcache.GetURLParams{Filename: "something.txt", Expires: 5 * time.Minute}

	url, meta, err := s.cache.GetURL(ctx, fcache.GetRequest{
		Key: "test.txt",
		TTL: 0, // expire file immediately
		Loader: func(ctx context.Context) (io.ReadCloser, fcache.FileMeta, error) {
			meta := fcache.FileMeta{
				Name: "a.txt",
				Mime: "text/plain",
				Size: 14,
				Meta: map[string]string{
					"someMetadata": "someValue",
				},
			}
			return io.NopCloser(strings.NewReader("some test data")), meta, nil
		},
	}, params)
	s.NoError(err)
	s.NotEmpty(url)

	stats, err := s.cache.Stat(ctx)
	s.Equal(int64(1), stats.Misses)

	s.Equal("a.txt", meta.Name)
	s.Equal("text/plain", meta.Mime)
	s.Equal(int64(14), meta.Size)
	s.NotNil(meta.Meta)
	s.Equal("someValue", meta.Meta["someMetadata"])

	s.T().Logf("url: %s, meta: %v", url, meta)

	invalidated, err := s.cache.Invalidate(ctx)
	s.NoError(err)
	s.Equal(int64(1), invalidated)

	meta, err = s.cache.Store.Meta(ctx, "test.txt")
	s.Empty(meta)
	s.ErrorIs(err, fcache.ErrNotFound)
}

func TestS3(t *testing.T) {
	cfg := config{}
	require.NoError(t, env.Parse(&cfg))

	if !cfg.IntegrationTests {
		t.Skip()
	}

	suite.Run(t, &S3Suite{cfg: cfg})
}
