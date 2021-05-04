package utils

import (
	"errors"
	"testing"

	"go.viam.com/test"
)

func TestTryClose(t *testing.T) {
	// not a closer
	test.That(t, TryClose(5), test.ShouldBeNil)

	stc := &somethingToClose{}
	test.That(t, TryClose(stc), test.ShouldBeNil)
	test.That(t, stc.called, test.ShouldEqual, 1)

	stc.err = true
	err := TryClose(stc)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
	test.That(t, stc.called, test.ShouldEqual, 2)
}

type somethingToClose struct {
	called int
	err    bool
}

func (stc *somethingToClose) Close() error {
	stc.called++
	if stc.err {
		return errors.New("whoops")
	}
	return nil
}
