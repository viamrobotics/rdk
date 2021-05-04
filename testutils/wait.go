package testutils

import (
	"testing"
	"time"
)

// WaitForAssertion waits for a testify.Assertion to succeed or ultimately fail.
// Note: This should only be used if there's absolutely no way to have the
// test code be able to be signaled that the assertion is ready to be checked.
// That is, if waiting with respect time is critical, it's okay to use this.
func WaitForAssertion(t *testing.T, assertion func(tb testing.TB)) {
	var attempts int
	var checkOk bool
	maxAttempts := 100
	for !checkOk && attempts < maxAttempts {
		noFailT := &testingTBNoFail{}
		assertion(noFailT)
		checkOk = !noFailT.Failed()
		if checkOk {
			return
		}
		time.Sleep(50 * time.Millisecond)
		attempts++
	}
	assertion(t)
}

type testingTBNoFail struct {
	testing.TB
	failed bool
}

func (t *testingTBNoFail) Cleanup(func()) {

}

func (t *testingTBNoFail) Error(args ...interface{}) {
	t.failed = true
}

func (t *testingTBNoFail) Errorf(format string, args ...interface{}) {
	t.failed = true
}

func (t *testingTBNoFail) Fail() {
	t.failed = true
}

func (t *testingTBNoFail) FailNow() {
	t.failed = true
}

func (t *testingTBNoFail) Failed() bool {
	return t.failed
}

func (t *testingTBNoFail) Fatal(args ...interface{}) {
	t.failed = true
}

func (t *testingTBNoFail) Fatalf(format string, args ...interface{}) {
	t.failed = true
}

func (t *testingTBNoFail) Helper() {

}

func (t *testingTBNoFail) Log(args ...interface{}) {

}

func (t *testingTBNoFail) Logf(format string, args ...interface{}) {

}

func (t *testingTBNoFail) Name() string {
	return ""
}

func (t *testingTBNoFail) Skip(args ...interface{}) {

}

func (t *testingTBNoFail) SkipNow() {

}

func (t *testingTBNoFail) Skipf(format string, args ...interface{}) {

}

func (t *testingTBNoFail) Skipped() bool {
	return false
}

func (t *testingTBNoFail) TempDir() string {
	return ""
}
