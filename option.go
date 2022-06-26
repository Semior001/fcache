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
