package builtin

import (
	"context"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/shell"
	rutils "go.viam.com/rdk/utils"
)

func TestDoCommand(t *testing.T) {
	svc, err := NewBuiltIn(shell.Named("shell1"), logging.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)

	resp, err := svc.DoCommand(context.Background(), map[string]interface{}{shell.GetViamHomeCommand: true})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp[shell.ViamHomeKey], test.ShouldEqual, rutils.ViamDotDir)

	_, err = svc.DoCommand(context.Background(), map[string]interface{}{"something_else": true})
	test.That(t, err, test.ShouldBeError, resource.ErrDoUnimplemented)
}

func TestCloseWorksWithActiveShell(t *testing.T) {
	svc, err := NewBuiltIn(shell.Named("shell1"), logging.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)

	// Start a shell session and do not close it.
	//nolint:dogsled
	_, _, _, err = svc.Shell(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)

	done := make(chan error, 1)
	go func() { done <- svc.Close(context.Background()) }()

	select {
	case err := <-done:
		test.That(t, err, test.ShouldBeNil)
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not return within 5s while a Shell session was active")
	}
}
