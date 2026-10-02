package gripper_test

import (
	"context"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	datasyncpb "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/structpb"

	gripper "go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/data"
	datatu "go.viam.com/rdk/data/testutils"
	"go.viam.com/rdk/logging"
	tu "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	componentName   = "gripper"
	captureInterval = time.Millisecond
)

var doCommandMap = map[string]any{"readings": "random-test"}

func TestIsHoldingSomethingCollector(t *testing.T) {
	start := time.Now()
	buf := tu.NewMockBuffer(t)
	params := data.CollectorParams{
		DataType:      data.CaptureTypeTabular,
		ComponentName: componentName,
		Interval:      captureInterval,
		Logger:        logging.NewTestLogger(t),
		Clock:         clock.New(),
		Target:        buf,
	}
	collector := data.CollectorLookup(data.MethodMetadata{
		API:        gripper.API,
		MethodName: "IsHoldingSomething",
	})
	test.That(t, collector, test.ShouldNotBeNil)
	col, err := collector(newGripper(), params)
	test.That(t, err, test.ShouldBeNil)

	defer col.Close()
	col.Collect()

	expectedData, err := structpb.NewStruct(map[string]any{
		"is_holding_something": true,
		"meta":                 map[string]any{"pressure": 0.8},
	})
	test.That(t, err, test.ShouldBeNil)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	tu.CheckMockBufferWrites(t, ctx, start, buf.Writes, []*datasyncpb.SensorData{{
		Metadata: &datasyncpb.SensorMetadata{},
		Data:     &datasyncpb.SensorData_Struct{Struct: expectedData},
	}})
	buf.Close()
}

func TestDoCommandCollector(t *testing.T) {
	datatu.TestDoCommandCollector(t, datatu.DoCommandTestConfig{
		ComponentName:   componentName,
		CaptureInterval: captureInterval,
		DoCommandMap:    doCommandMap,
		Collector:       gripper.NewDoCommandCollector,
		ResourceFactory: func() interface{} { return newGripper() },
	})
}

func newGripper() gripper.Gripper {
	g := &inject.Gripper{}
	g.IsHoldingSomethingFunc = func(ctx context.Context, extra map[string]interface{}) (gripper.HoldingStatus, error) {
		return gripper.HoldingStatus{
			IsHoldingSomething: true,
			Meta:               map[string]interface{}{"pressure": 0.8},
		}, nil
	}
	g.DoFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
		return doCommandMap, nil
	}
	return g
}

func TestGetWorldPoseCollector(t *testing.T) {
	datatu.TestGetWorldPoseCollector(t, datatu.GetWorldPoseTestConfig{
		ComponentName:   componentName,
		CaptureInterval: captureInterval,
		Collector:       gripper.NewGetWorldPoseCollector,
		ResourceFactory: func() interface{} { return newGripper() },
	})
}
