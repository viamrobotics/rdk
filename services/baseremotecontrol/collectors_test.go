package baseremotecontrol_test

import (
	"context"
	"testing"
	"time"

	baseremotecontrol "go.viam.com/rdk/services/baseremotecontrol"
	tu "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	componentName   = "base_remote_control"
	captureInterval = time.Millisecond
)

var doCommandMap = map[string]any{"readings": "random-test"}

func TestDoCommandCollector(t *testing.T) {
	tu.TestDoCommandCollector(t, tu.DoCommandTestConfig{
		ComponentName:   componentName,
		CaptureInterval: captureInterval,
		DoCommandMap:    doCommandMap,
		Collector:       baseremotecontrol.NewDoCommandCollector,
		ResourceFactory: func() interface{} { return newBaseRemoteControl() },
	})
}

func newBaseRemoteControl() baseremotecontrol.Service {
	b := &inject.BaseRemoteControlService{}
	b.DoCommandFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
		return doCommandMap, nil
	}
	return b
}
