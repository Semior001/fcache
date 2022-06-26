# fcache [![Go](https://github.com/Semior001/fcache/actions/workflows/go.yaml/badge.svg)](https://github.com/Semior001/fcache/actions/workflows/go.yaml) [![codecov](https://codecov.io/gh/Semior001/fcache/branch/master/graph/badge.svg?token=nLxLt9Vdyo)](https://codecov.io/gh/Semior001/fcache) [![go report card](https://goreportcard.com/badge/github.com/Semior001/fcache)](https://goreportcard.com/report/github.com/Semior001/fcache) [![Go Reference](https://pkg.go.dev/badge/github.com/Semior001/fcache.svg)](https://pkg.go.dev/github.com/Semior001/fcache)
Package `fcache` introduces file cache implementation for caching files.

### s3
**Note:** s3 file cache doesn't expire files by its own, for doing that you
have to set lifecycle policy for the bucket, that will be used for caching
