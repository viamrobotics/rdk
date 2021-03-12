package main

import (
	"testing"

	"go.viam.com/robotcore/testutils"

	"github.com/edaniels/test"
)

func TestMain(t *testing.T) {
	for _, tc := range []struct {
		Name string
		Args []string
		Err  string
	}{
		{"no args", nil, ""},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			exec := testutils.ContextualMain(mainWithArgs, tc.Args)
			<-exec.Ready
			exec.Stop()
			err := <-exec.Done
			if tc.Err == "" {
				test.That(t, err, test.ShouldBeNil)
				return
			}
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, tc.Err)
		})
	}
}
