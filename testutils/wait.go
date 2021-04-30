package testutils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testingTNoFail struct {
	Failed bool
}

func (t *testingTNoFail) Errorf(format string, args ...interface{}) {
	t.Failed = true
}

// WaitForAssertion waits for a testify.Assertion to succeed or ultimately fail.
// Note: This should only be used if there's absolutely no way to have the
// test code be able to be signaled that the assertion is ready to be checked.
// That is, if waiting with respect time is critical, it's okay to use this.
func WaitForAssertion(t *testing.T, assertion func(assert.TestingT)) {
	var attempts int
	var checkOk bool
	maxAttempts := 100
	for !checkOk && attempts < maxAttempts {
		noFailT := &testingTNoFail{}
		assertion(noFailT)
		checkOk = !noFailT.Failed
		if checkOk {
			break
		}
		time.Sleep(50 * time.Millisecond)
		attempts++
	}
	assertion(t)
}
