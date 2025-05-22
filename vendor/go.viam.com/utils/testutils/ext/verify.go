// Package testutilsext is purely for test utilities that may access other packages
// in the codebase that tend to use testutils.
package testutilsext

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/edaniels/golog"
	"go.uber.org/goleak"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"

	"go.viam.com/utils"
	"go.viam.com/utils/artifact"
	"go.viam.com/utils/testutils"
)

// VerifyTestMain preforms various runtime checks on code that tests run.
func VerifyTestMain(m goleak.TestingM) {
	func() {
		// workaround https://github.com/googleapis/google-cloud-go/issues/5430
		httpClient := &http.Client{Transport: http.DefaultTransport.(*http.Transport).Clone()}
		defer httpClient.CloseIdleConnections()

		//nolint:errcheck
		_, _ = transport.Creds(context.Background(), option.WithHTTPClient(httpClient))

		t := time.NewTimer(100 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				return
			default:
				runtime.Gosched()
			}
		}
	}()

	currentGoroutines := goleak.IgnoreCurrent()

	cache, err := artifact.GlobalCache()
	if err != nil {
		golog.Global().Fatalw("error opening artifact", "error", err)
	}
	//nolint:ifshort
	exitCode := m.Run()
	testutils.Teardown()
	if err := cache.Close(); err != nil {
		golog.Global().Errorw("error closing artifact", "error", err)
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
	if err := utils.FindGoroutineLeaks(currentGoroutines); err != nil {
		fmt.Fprintf(os.Stderr, "goleak: Errors on successful test run: %v\n", err)
		os.Exit(1)
	}
}
