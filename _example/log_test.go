package _example

import "testing"

type tLogAdapter testing.T

func (t *tLogAdapter) Printf(format string, args ...interface{}) { t.Logf(format, args...) }
