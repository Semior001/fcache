package fcache

import "time"

// GetRequest defines parameters to get file.
type GetRequest struct {
	Key string
	TTL time.Duration
	Loader
}

// Options defines cache options.
type Options struct {
	Log Logger
	// InvalidatePeriod sets the time for checking cache for expired items.
	// Zero means "no invalidation", i.e. backend invalidates items by its own.
	InvalidatePeriod time.Duration
	// ExtendTTL sets whether cache should extend TTL of cached items on hit.
	ExtendTTL bool
}

// Option is a function to apply options.
type Option func(*Options)

// WithLogger sets logger for cache.
// `log` package is used by default.
func WithLogger(log Logger) Option {
	return func(o *Options) { o.Log = log }
}

// WithInvalidationPeriod sets the period between cache's checks for expired
// items. Useful for caches, like S3.
// No manual invalidation by default.
func WithInvalidationPeriod(period time.Duration) Option {
	return func(o *Options) { o.InvalidatePeriod = period }
}
