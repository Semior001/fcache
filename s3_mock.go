// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package fcache

import (
	"context"
	"io"
	"net/url"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
)

// Ensure, that s3clientMock does implement s3client.
// If this is not the case, regenerate this file with moq.
var _ s3client = &s3clientMock{}

// s3clientMock is a mock implementation of s3client.
//
// 	func TestSomethingThatUsess3client(t *testing.T) {
//
// 		// make and configure a mocked s3client
// 		mockeds3client := &s3clientMock{
// 			GetObjectFunc: func(ctx context.Context, bkt string, key string, opts minio.GetObjectOptions) (*minio.Object, error) {
// 				panic("mock out the GetObject method")
// 			},
// 			ListObjectsFunc: func(ctx context.Context, bkt string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
// 				panic("mock out the ListObjects method")
// 			},
// 			PresignedGetObjectFunc: func(ctx context.Context, bkt string, key string, expires time.Duration, reqParams url.Values) (*url.URL, error) {
// 				panic("mock out the PresignedGetObject method")
// 			},
// 			PutObjectFunc: func(ctx context.Context, bkt string, key string, rd io.Reader, sz int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
// 				panic("mock out the PutObject method")
// 			},
// 			RemoveObjectFunc: func(ctx context.Context, bkt string, key string, opts minio.RemoveObjectOptions) error {
// 				panic("mock out the RemoveObject method")
// 			},
// 		}
//
// 		// use mockeds3client in code that requires s3client
// 		// and then make assertions.
//
// 	}
type s3clientMock struct {
	// GetObjectFunc mocks the GetObject method.
	GetObjectFunc func(ctx context.Context, bkt string, key string, opts minio.GetObjectOptions) (*minio.Object, error)

	// ListObjectsFunc mocks the ListObjects method.
	ListObjectsFunc func(ctx context.Context, bkt string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo

	// PresignedGetObjectFunc mocks the PresignedGetObject method.
	PresignedGetObjectFunc func(ctx context.Context, bkt string, key string, expires time.Duration, reqParams url.Values) (*url.URL, error)

	// PutObjectFunc mocks the PutObject method.
	PutObjectFunc func(ctx context.Context, bkt string, key string, rd io.Reader, sz int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)

	// RemoveObjectFunc mocks the RemoveObject method.
	RemoveObjectFunc func(ctx context.Context, bkt string, key string, opts minio.RemoveObjectOptions) error

	// calls tracks calls to the methods.
	calls struct {
		// GetObject holds details about calls to the GetObject method.
		GetObject []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Bkt is the bkt argument value.
			Bkt string
			// Key is the key argument value.
			Key string
			// Opts is the opts argument value.
			Opts minio.GetObjectOptions
		}
		// ListObjects holds details about calls to the ListObjects method.
		ListObjects []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Bkt is the bkt argument value.
			Bkt string
			// Opts is the opts argument value.
			Opts minio.ListObjectsOptions
		}
		// PresignedGetObject holds details about calls to the PresignedGetObject method.
		PresignedGetObject []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Bkt is the bkt argument value.
			Bkt string
			// Key is the key argument value.
			Key string
			// Expires is the expires argument value.
			Expires time.Duration
			// ReqParams is the reqParams argument value.
			ReqParams url.Values
		}
		// PutObject holds details about calls to the PutObject method.
		PutObject []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Bkt is the bkt argument value.
			Bkt string
			// Key is the key argument value.
			Key string
			// Rd is the rd argument value.
			Rd io.Reader
			// Sz is the sz argument value.
			Sz int64
			// Opts is the opts argument value.
			Opts minio.PutObjectOptions
		}
		// RemoveObject holds details about calls to the RemoveObject method.
		RemoveObject []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Bkt is the bkt argument value.
			Bkt string
			// Key is the key argument value.
			Key string
			// Opts is the opts argument value.
			Opts minio.RemoveObjectOptions
		}
	}
	lockGetObject          sync.RWMutex
	lockListObjects        sync.RWMutex
	lockPresignedGetObject sync.RWMutex
	lockPutObject          sync.RWMutex
	lockRemoveObject       sync.RWMutex
}

// GetObject calls GetObjectFunc.
func (mock *s3clientMock) GetObject(ctx context.Context, bkt string, key string, opts minio.GetObjectOptions) (*minio.Object, error) {
	if mock.GetObjectFunc == nil {
		panic("s3clientMock.GetObjectFunc: method is nil but s3client.GetObject was just called")
	}
	callInfo := struct {
		Ctx  context.Context
		Bkt  string
		Key  string
		Opts minio.GetObjectOptions
	}{
		Ctx:  ctx,
		Bkt:  bkt,
		Key:  key,
		Opts: opts,
	}
	mock.lockGetObject.Lock()
	mock.calls.GetObject = append(mock.calls.GetObject, callInfo)
	mock.lockGetObject.Unlock()
	return mock.GetObjectFunc(ctx, bkt, key, opts)
}

// GetObjectCalls gets all the calls that were made to GetObject.
// Check the length with:
//     len(mockeds3client.GetObjectCalls())
func (mock *s3clientMock) GetObjectCalls() []struct {
	Ctx  context.Context
	Bkt  string
	Key  string
	Opts minio.GetObjectOptions
} {
	var calls []struct {
		Ctx  context.Context
		Bkt  string
		Key  string
		Opts minio.GetObjectOptions
	}
	mock.lockGetObject.RLock()
	calls = mock.calls.GetObject
	mock.lockGetObject.RUnlock()
	return calls
}

// ListObjects calls ListObjectsFunc.
func (mock *s3clientMock) ListObjects(ctx context.Context, bkt string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	if mock.ListObjectsFunc == nil {
		panic("s3clientMock.ListObjectsFunc: method is nil but s3client.ListObjects was just called")
	}
	callInfo := struct {
		Ctx  context.Context
		Bkt  string
		Opts minio.ListObjectsOptions
	}{
		Ctx:  ctx,
		Bkt:  bkt,
		Opts: opts,
	}
	mock.lockListObjects.Lock()
	mock.calls.ListObjects = append(mock.calls.ListObjects, callInfo)
	mock.lockListObjects.Unlock()
	return mock.ListObjectsFunc(ctx, bkt, opts)
}

// ListObjectsCalls gets all the calls that were made to ListObjects.
// Check the length with:
//     len(mockeds3client.ListObjectsCalls())
func (mock *s3clientMock) ListObjectsCalls() []struct {
	Ctx  context.Context
	Bkt  string
	Opts minio.ListObjectsOptions
} {
	var calls []struct {
		Ctx  context.Context
		Bkt  string
		Opts minio.ListObjectsOptions
	}
	mock.lockListObjects.RLock()
	calls = mock.calls.ListObjects
	mock.lockListObjects.RUnlock()
	return calls
}

// PresignedGetObject calls PresignedGetObjectFunc.
func (mock *s3clientMock) PresignedGetObject(ctx context.Context, bkt string, key string, expires time.Duration, reqParams url.Values) (*url.URL, error) {
	if mock.PresignedGetObjectFunc == nil {
		panic("s3clientMock.PresignedGetObjectFunc: method is nil but s3client.PresignedGetObject was just called")
	}
	callInfo := struct {
		Ctx       context.Context
		Bkt       string
		Key       string
		Expires   time.Duration
		ReqParams url.Values
	}{
		Ctx:       ctx,
		Bkt:       bkt,
		Key:       key,
		Expires:   expires,
		ReqParams: reqParams,
	}
	mock.lockPresignedGetObject.Lock()
	mock.calls.PresignedGetObject = append(mock.calls.PresignedGetObject, callInfo)
	mock.lockPresignedGetObject.Unlock()
	return mock.PresignedGetObjectFunc(ctx, bkt, key, expires, reqParams)
}

// PresignedGetObjectCalls gets all the calls that were made to PresignedGetObject.
// Check the length with:
//     len(mockeds3client.PresignedGetObjectCalls())
func (mock *s3clientMock) PresignedGetObjectCalls() []struct {
	Ctx       context.Context
	Bkt       string
	Key       string
	Expires   time.Duration
	ReqParams url.Values
} {
	var calls []struct {
		Ctx       context.Context
		Bkt       string
		Key       string
		Expires   time.Duration
		ReqParams url.Values
	}
	mock.lockPresignedGetObject.RLock()
	calls = mock.calls.PresignedGetObject
	mock.lockPresignedGetObject.RUnlock()
	return calls
}

// PutObject calls PutObjectFunc.
func (mock *s3clientMock) PutObject(ctx context.Context, bkt string, key string, rd io.Reader, sz int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	if mock.PutObjectFunc == nil {
		panic("s3clientMock.PutObjectFunc: method is nil but s3client.PutObject was just called")
	}
	callInfo := struct {
		Ctx  context.Context
		Bkt  string
		Key  string
		Rd   io.Reader
		Sz   int64
		Opts minio.PutObjectOptions
	}{
		Ctx:  ctx,
		Bkt:  bkt,
		Key:  key,
		Rd:   rd,
		Sz:   sz,
		Opts: opts,
	}
	mock.lockPutObject.Lock()
	mock.calls.PutObject = append(mock.calls.PutObject, callInfo)
	mock.lockPutObject.Unlock()
	return mock.PutObjectFunc(ctx, bkt, key, rd, sz, opts)
}

// PutObjectCalls gets all the calls that were made to PutObject.
// Check the length with:
//     len(mockeds3client.PutObjectCalls())
func (mock *s3clientMock) PutObjectCalls() []struct {
	Ctx  context.Context
	Bkt  string
	Key  string
	Rd   io.Reader
	Sz   int64
	Opts minio.PutObjectOptions
} {
	var calls []struct {
		Ctx  context.Context
		Bkt  string
		Key  string
		Rd   io.Reader
		Sz   int64
		Opts minio.PutObjectOptions
	}
	mock.lockPutObject.RLock()
	calls = mock.calls.PutObject
	mock.lockPutObject.RUnlock()
	return calls
}

// RemoveObject calls RemoveObjectFunc.
func (mock *s3clientMock) RemoveObject(ctx context.Context, bkt string, key string, opts minio.RemoveObjectOptions) error {
	if mock.RemoveObjectFunc == nil {
		panic("s3clientMock.RemoveObjectFunc: method is nil but s3client.RemoveObject was just called")
	}
	callInfo := struct {
		Ctx  context.Context
		Bkt  string
		Key  string
		Opts minio.RemoveObjectOptions
	}{
		Ctx:  ctx,
		Bkt:  bkt,
		Key:  key,
		Opts: opts,
	}
	mock.lockRemoveObject.Lock()
	mock.calls.RemoveObject = append(mock.calls.RemoveObject, callInfo)
	mock.lockRemoveObject.Unlock()
	return mock.RemoveObjectFunc(ctx, bkt, key, opts)
}

// RemoveObjectCalls gets all the calls that were made to RemoveObject.
// Check the length with:
//     len(mockeds3client.RemoveObjectCalls())
func (mock *s3clientMock) RemoveObjectCalls() []struct {
	Ctx  context.Context
	Bkt  string
	Key  string
	Opts minio.RemoveObjectOptions
} {
	var calls []struct {
		Ctx  context.Context
		Bkt  string
		Key  string
		Opts minio.RemoveObjectOptions
	}
	mock.lockRemoveObject.RLock()
	calls = mock.calls.RemoveObject
	mock.lockRemoveObject.RUnlock()
	return calls
}
