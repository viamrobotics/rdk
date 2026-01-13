package button_test

import (
	"context"
	"errors"
	"testing"

	pbcommon "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/button/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/components/button"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

var (
	errCantPush       = errors.New("can't push")
	errButtonNotFound = errors.New("not found")
)

func newServer(logger logging.Logger) (pb.ButtonServiceServer, *inject.Button, *inject.Button, error) {
	injectButton := &inject.Button{}
	injectButton2 := &inject.Button{}
	buttons := map[resource.Name]button.Button{
		button.Named(testButtonName):  injectButton,
		button.Named(testButtonName2): injectButton2,
	}
	buttonSvc, err := resource.NewAPIResourceCollection(button.API, buttons)
	if err != nil {
		return nil, nil, nil, err
	}
	return button.NewRPCServiceServer(buttonSvc, logger).(pb.ButtonServiceServer), injectButton, injectButton2, nil
}

func TestServer(t *testing.T) {
	buttonServer, injectButton, injectButton2, err := newServer(logging.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)

	var buttonPushed string
	var extraOptions map[string]interface{}

	injectButton.PushFunc = func(ctx context.Context, extra map[string]interface{}) error {
		extraOptions = extra
		buttonPushed = testButtonName
		return nil
	}

	injectButton2.PushFunc = func(ctx context.Context, extra map[string]interface{}) error {
		buttonPushed = testButtonName2
		return errCantPush
	}

	t.Run("push", func(t *testing.T) {
		_, err := buttonServer.Push(context.Background(), &pb.PushRequest{Name: missingButtonName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errButtonNotFound.Error())

		extra := map[string]interface{}{"foo": "Push"}
		ext, err := protoutils.StructToStructPb(extra)
		test.That(t, err, test.ShouldBeNil)
		_, err = buttonServer.Push(context.Background(), &pb.PushRequest{Name: testButtonName, Extra: ext})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, buttonPushed, test.ShouldEqual, testButtonName)
		test.That(t, extraOptions, test.ShouldResemble, extra)

		_, err = buttonServer.Push(context.Background(), &pb.PushRequest{Name: testButtonName2})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, errCantPush)
		test.That(t, buttonPushed, test.ShouldEqual, testButtonName2)
	})

	t.Run("do command", func(t *testing.T) {
		_, err := buttonServer.DoCommand(context.Background(), &pbcommon.DoCommandRequest{Name: missingButtonName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errButtonNotFound.Error())

		injectButton.DoFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
			return cmd, nil
		}

		extra := map[string]interface{}{"foo": "DoCommand"}
		ext, err := protoutils.StructToStructPb(extra)
		test.That(t, err, test.ShouldBeNil)
		resp, err := buttonServer.DoCommand(context.Background(), &pbcommon.DoCommandRequest{
			Name:    testButtonName,
			Command: ext,
		})
		test.That(t, err, test.ShouldBeNil)
		respMap := resp.GetResult().AsMap()
		test.That(t, respMap, test.ShouldResemble, extra)
	})
}
