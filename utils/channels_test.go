package utils

import (
	"sync"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"
)

func TestWaitWithNoTasks(t *testing.T) {
	var wg sync.WaitGroup

	err := WaitWithError(&wg, make(chan error))
	test.That(t, err, test.ShouldBeNil)
}

func TestWaitWithNoErrors(t *testing.T) {
	var wg sync.WaitGroup
	var val bool

	wg.Add(1)
	go func() {
		defer wg.Done()
		val = true
	}()

	err := WaitWithError(&wg, make(chan error))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, val, test.ShouldBeTrue)
}

func TestWaitWithError(t *testing.T) {
	var wg sync.WaitGroup
	var val bool

	errs := make(chan error)
	wg.Add(1)
	go func() {
		defer wg.Done()
		errs <- errors.New("failed")
	}()

	err := WaitWithError(&wg, errs)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, val, test.ShouldBeFalse)
}
