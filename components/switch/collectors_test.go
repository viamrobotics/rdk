package toggleswitch_test

import (
	"context"
	"testing"
	"time"

	toggleswitch "go.viam.com/rdk/components/switch"
	datatu "go.viam.com/rdk/data/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	componentName   = "switch"
	captureInterval = time.Millisecond
)

var doCommandMap = map[string]any{"readings": "random-test"}

func TestDoCommandCollector(t *testing.T) {
	datatu.TestDoCommandCollector(t, datatu.DoCommandTestConfig{
		ComponentName:   componentName,
		CaptureInterval: captureInterval,
		DoCommandMap:    doCommandMap,
		Collector:       toggleswitch.NewDoCommandCollector,
		ResourceFactory: func() interface{} { return newSwitch() },
	})
}

func newSwitch() toggleswitch.Switch {
	s := &inject.Switch{}
	s.DoFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
		return doCommandMap, nil
	}
	return s
}
