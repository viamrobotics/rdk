package main

import (
	"testing"

	"go.uber.org/zap/zaptest/observer"
	"go.viam.com/robotcore/testutils"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
)

func TestMain(t *testing.T) {
	for _, tc := range []struct {
		Name   string
		Args   []string
		Err    string
		During func(exec *testutils.ContextualMainExecution)
		After  func(t *testing.T, logs *observer.ObservedLogs)
	}{
		// parsing
		{"no args", nil, "", nil, nil},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			var logs *observer.ObservedLogs
			logger, logs = golog.NewObservedTestLogger(t)
			exec := testutils.ContextualMain(mainWithArgs, tc.Args)
			<-exec.Ready

			if tc.During != nil {
				tc.During(&exec)
			}
			exec.Stop()
			err := <-exec.Done
			if tc.Err == "" {
				test.That(t, err, test.ShouldBeNil)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.Err)
			}
			if tc.After != nil {
				tc.After(t, logs)
			}
		})
	}
}
