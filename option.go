package fcache

import "time"

// Options defines cache options.
type Options struct {
	TTL time.Duration
	Log Logger
	// InvalidatePeriod sets the time for checking cache for expired items.
	// Zero means "no invalidation", i.e. backend invalidates items by its own.
	InvalidatePeriod time.Duration
}

// Option is a function to apply options.
type Option func(*Options)

// WithTTL sets the TTL duration for cache items.
func WithTTL(ttl time.Duration) Option {
	return func(o *Options) { o.TTL = ttl }
}

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
