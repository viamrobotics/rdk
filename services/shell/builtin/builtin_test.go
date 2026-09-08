package builtin

import (
	"context"
	"testing"

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
