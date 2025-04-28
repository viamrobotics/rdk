package test

import "testing"

func That(tb testing.TB, actual interface{}, assert func(actual interface{}, expected ...interface{}) string, expected ...interface{}) {
	tb.Helper()
	if result := assert(actual, expected...); result != "" {
		tb.Fatal(result)
	}
}
