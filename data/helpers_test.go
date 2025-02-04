package data

import (
	"testing"
	"time"

	"go.viam.com/test"
)

func TestGetDurationFromHz(t *testing.T) {
	test.That(t, GetDurationFromHz(0.1), test.ShouldEqual, time.Second*10)
	test.That(t, GetDurationFromHz(0.5), test.ShouldEqual, time.Second*2)
	test.That(t, GetDurationFromHz(1), test.ShouldEqual, time.Second)
	test.That(t, GetDurationFromHz(1000), test.ShouldEqual, time.Millisecond)
	test.That(t, GetDurationFromHz(0), test.ShouldEqual, 0)
}
