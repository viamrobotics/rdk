package switch_component_test

import (
	"context"
	"errors"
	"testing"

	pb "go.viam.com/api/component/switch/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"

	switch_component "go.viam.com/rdk/components/switch"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

var (
	errCantSetPosition          = errors.New("can't set position")
	errCantGetPosition          = errors.New("can't get position")
	errCantGetNumberOfPositions = errors.New("can't get number of positions")
	errSwitchNotFound           = errors.New("not found")
)

const testSwitchName2 = "switch3"

func newServer() (pb.SwitchServiceServer, *inject.Switch, *inject.Switch, error) {
	injectSwitch := &inject.Switch{}
	injectSwitch2 := &inject.Switch{}
	switches := map[resource.Name]switch_component.Switch{
		switch_component.Named(testSwitchName):  injectSwitch,
		switch_component.Named(testSwitchName2): injectSwitch2,
	}
	switchSvc, err := resource.NewAPIResourceCollection(switch_component.API, switches)
	if err != nil {
		return nil, nil, nil, err
	}
	return switch_component.NewRPCServiceServer(switchSvc).(pb.SwitchServiceServer), injectSwitch, injectSwitch2, nil
}

func TestServer(t *testing.T) {
	switchServer, injectSwitch, injectSwitch2, err := newServer()
	test.That(t, err, test.ShouldBeNil)

	var switchName string
	var extraOptions map[string]interface{}

	injectSwitch.SetPositionFunc = func(ctx context.Context, position uint32, extra map[string]interface{}) error {
		extraOptions = extra
		switchName = testSwitchName
		return nil
	}
	injectSwitch.GetPositionFunc = func(ctx context.Context, extra map[string]interface{}) (uint32, error) {
		extraOptions = extra
		return 0, nil
	}
	injectSwitch.GetNumberOfPositionsFunc = func(ctx context.Context, extra map[string]interface{}) (int, error) {
		extraOptions = extra
		return 2, nil
	}

	injectSwitch2.SetPositionFunc = func(ctx context.Context, position uint32, extra map[string]interface{}) error {
		switchName = testSwitchName2
		return errCantSetPosition
	}
	injectSwitch2.GetPositionFunc = func(ctx context.Context, extra map[string]interface{}) (uint32, error) {
		return 0, errCantGetPosition
	}
	injectSwitch2.GetNumberOfPositionsFunc = func(ctx context.Context, extra map[string]interface{}) (int, error) {
		return 0, errCantGetNumberOfPositions
	}

	t.Run("set position", func(t *testing.T) {
		_, err := switchServer.SetPosition(context.Background(), &pb.SetPositionRequest{Name: missingSwitchName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errSwitchNotFound.Error())

		extra := map[string]interface{}{"foo": "SetPosition"}
		ext, err := protoutils.StructToStructPb(extra)
		test.That(t, err, test.ShouldBeNil)
		_, err = switchServer.SetPosition(context.Background(), &pb.SetPositionRequest{Name: testSwitchName, Position: 0, Extra: ext})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, switchName, test.ShouldEqual, testSwitchName)
		test.That(t, extraOptions, test.ShouldResemble, extra)

		_, err = switchServer.SetPosition(context.Background(), &pb.SetPositionRequest{Name: testSwitchName2})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errCantSetPosition.Error())
		test.That(t, switchName, test.ShouldEqual, testSwitchName2)
	})

	t.Run("get position", func(t *testing.T) {
		_, err := switchServer.GetPosition(context.Background(), &pb.GetPositionRequest{Name: missingSwitchName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errSwitchNotFound.Error())

		extra := map[string]interface{}{"foo": "GetPosition"}
		ext, err := protoutils.StructToStructPb(extra)
		test.That(t, err, test.ShouldBeNil)
		resp, err := switchServer.GetPosition(context.Background(), &pb.GetPositionRequest{Name: testSwitchName, Extra: ext})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Position, test.ShouldEqual, 0)
		test.That(t, extraOptions, test.ShouldResemble, extra)

		_, err = switchServer.GetPosition(context.Background(), &pb.GetPositionRequest{Name: testSwitchName2})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errCantGetPosition.Error())
	})

	t.Run("get number of positions", func(t *testing.T) {
		_, err := switchServer.GetNumberOfPositions(context.Background(), &pb.GetNumberOfPositionsRequest{Name: missingSwitchName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errSwitchNotFound.Error())

		extra := map[string]interface{}{"foo": "GetNumberOfPositions"}
		ext, err := protoutils.StructToStructPb(extra)
		test.That(t, err, test.ShouldBeNil)
		resp, err := switchServer.GetNumberOfPositions(context.Background(), &pb.GetNumberOfPositionsRequest{Name: testSwitchName, Extra: ext})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.NumberOfPositions, test.ShouldEqual, 2)
		test.That(t, extraOptions, test.ShouldResemble, extra)

		_, err = switchServer.GetNumberOfPositions(context.Background(), &pb.GetNumberOfPositionsRequest{Name: testSwitchName2})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errCantGetNumberOfPositions.Error())
	})
}
