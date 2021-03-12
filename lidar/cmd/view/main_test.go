package main

import (
	"fmt"
	"net/http"
	"testing"

	"go.viam.com/robotcore/testutils"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
)

func TestMain(t *testing.T) {
	randomPort, err := utils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	defaultPort = randomPort

	for _, tc := range []struct {
		Name         string
		Args         []string
		ExpectedPort int
		Err          string
	}{
		{"no args", nil, defaultPort, ""},
		{"bad port", []string{"ten"}, 0, "invalid syntax"},
		{"unknown named arg", []string{"--unknown"}, 0, "not defined"},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			logger = golog.NewTestLogger(t)
			exec := testutils.ContextualMain(mainWithArgs, tc.Args)
			<-exec.Ready

			if tc.Err == "" {
				req, err := http.NewRequest("GET", fmt.Sprintf("http://localhost:%d", tc.ExpectedPort), nil)
				test.That(t, err, test.ShouldBeNil)
				resp, err := http.DefaultClient.Do(req)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, resp.StatusCode, test.ShouldEqual, http.StatusOK)
			}
			exec.Stop()
			err = <-exec.Done
			if tc.Err == "" {
				test.That(t, err, test.ShouldBeNil)
				return
			}
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, tc.Err)
		})
	}
}
