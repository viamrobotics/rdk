package toggleswitch_test

import (
	"context"
	"errors"
	"testing"

	pb "go.viam.com/api/component/switch/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"

	toggleswitch "go.viam.com/rdk/components/switch"
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
	switches := map[resource.Name]toggleswitch.Switch{
		toggleswitch.Named(testSwitchName):  injectSwitch,
		toggleswitch.Named(testSwitchName2): injectSwitch2,
	}
	switchSvc, err := resource.NewAPIResourceCollection(toggleswitch.API, switches)
	if err != nil {
		return nil, nil, nil, err
	}
	return toggleswitch.NewRPCServiceServer(switchSvc).(pb.SwitchServiceServer), injectSwitch, injectSwitch2, nil
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
	injectSwitch.GetNumberOfPositionsFunc = func(ctx context.Context, extra map[string]interface{}) (uint32, error) {
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
	injectSwitch2.GetNumberOfPositionsFunc = func(ctx context.Context, extra map[string]interface{}) (uint32, error) {
		return 0, errCantGetNumberOfPositions
	}

	tests := []struct {
		name       string
		operation  string
		switchName string
		position   uint32
		extras     map[string]interface{}
		wantErr    error
		wantSwitch string
		wantPos    uint32
		wantNum    uint32
		wantExtras map[string]interface{}
	}{
		{
			name:       "set position - missing switch",
			operation:  "set",
			switchName: missingSwitchName,
			wantErr:    errSwitchNotFound,
		},
		{
			name:       "set position - successful with extras",
			operation:  "set",
			switchName: testSwitchName,
			position:   0,
			extras:     map[string]interface{}{"foo": "SetPosition"},
			wantSwitch: testSwitchName,
			wantExtras: map[string]interface{}{"foo": "SetPosition"},
		},
		{
			name:       "set position - error",
			operation:  "set",
			switchName: testSwitchName2,
			wantErr:    errCantSetPosition,
			wantSwitch: testSwitchName2,
		},
		{
			name:       "get position - missing switch",
			operation:  "get",
			switchName: missingSwitchName,
			wantErr:    errSwitchNotFound,
		},
		{
			name:       "get position - successful with extras",
			operation:  "get",
			switchName: testSwitchName,
			extras:     map[string]interface{}{"foo": "GetPosition"},
			wantPos:    0,
			wantExtras: map[string]interface{}{"foo": "GetPosition"},
		},
		{
			name:       "get position - error",
			operation:  "get",
			switchName: testSwitchName2,
			wantErr:    errCantGetPosition,
		},
		{
			name:       "get number of positions - missing switch",
			operation:  "num",
			switchName: missingSwitchName,
			wantErr:    errSwitchNotFound,
		},
		{
			name:       "get number of positions - successful with extras",
			operation:  "num",
			switchName: testSwitchName,
			extras:     map[string]interface{}{"foo": "GetNumberOfPositions"},
			wantNum:    2,
			wantExtras: map[string]interface{}{"foo": "GetNumberOfPositions"},
		},
		{
			name:       "get number of positions - error",
			operation:  "num",
			switchName: testSwitchName2,
			wantErr:    errCantGetNumberOfPositions,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			var resp interface{}
			ext, err := protoutils.StructToStructPb(tt.extras)
			test.That(t, err, test.ShouldBeNil)

			switch tt.operation {
			case "set":
				resp, err = switchServer.SetPosition(context.Background(), &pb.SetPositionRequest{
					Name:     tt.switchName,
					Position: tt.position,
					Extra:    ext,
				})
			case "get":
				resp, err = switchServer.GetPosition(context.Background(), &pb.GetPositionRequest{
					Name:  tt.switchName,
					Extra: ext,
				})
			case "num":
				resp, err = switchServer.GetNumberOfPositions(context.Background(), &pb.GetNumberOfPositionsRequest{
					Name:  tt.switchName,
					Extra: ext,
				})
			}

			if tt.wantErr != nil {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tt.wantErr.Error())
			} else {
				test.That(t, err, test.ShouldBeNil)
				switch tt.operation {
				case "get":
					test.That(t, resp.(*pb.GetPositionResponse).Position, test.ShouldEqual, tt.wantPos)
				case "num":
					test.That(t, resp.(*pb.GetNumberOfPositionsResponse).NumberOfPositions, test.ShouldEqual, tt.wantNum)
				}
			}
			if tt.wantSwitch != "" {
				test.That(t, switchName, test.ShouldEqual, tt.wantSwitch)
			}
			if tt.wantExtras != nil {
				test.That(t, extraOptions, test.ShouldResemble, tt.wantExtras)
			}
		})
	}
}
