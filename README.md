# fcache
Package `fcache` introduces file cache implementation for caching files.

### s3
**Note:** s3 file cache doesn't expire files by its own, for doing that you
have to set lifecycle policy for the bucket, that will be used for caching
