package fcache

import "log"

// Logger defines a single method for logging in caches.
type Logger interface {
	Printf(format string, args ...interface{})
}

type stdLogger struct{}

func (stdLogger) Printf(format string, args ...interface{}) {
	log.Printf(format, args...)
}
