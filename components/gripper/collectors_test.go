package gripper_test

import (
	"context"
	"testing"
	"time"

	gripper "go.viam.com/rdk/components/gripper"
	tu "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	componentName   = "gripper"
	captureInterval = time.Millisecond
)

var doCommandMap = map[string]any{"readings": "random-test"}

func TestDoCommandCollector(t *testing.T) {
	tu.TestDoCommandCollector(t, tu.DoCommandTestConfig{
		ComponentName:   componentName,
		CaptureInterval: captureInterval,
		DoCommandMap:    doCommandMap,
		Collector:       gripper.NewDoCommandCollector,
		ResourceFactory: func() interface{} { return newGripper() },
	})
}

func newGripper() gripper.Gripper {
	g := &inject.Gripper{}
	g.DoFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
		return doCommandMap, nil
	}
	return g
}
