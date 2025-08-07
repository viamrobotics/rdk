package motion_test

import (
	"context"
	"testing"
	"time"

	datatu "go.viam.com/rdk/data/testutils"
	motion "go.viam.com/rdk/services/motion"
	inject "go.viam.com/rdk/testutils/inject/motion"
)

const (
	componentName   = "motion"
	captureInterval = time.Millisecond
)

var doCommandMap = map[string]any{"readings": "random-test"}

func TestDoCommandCollector(t *testing.T) {
	datatu.TestDoCommandCollector(t, datatu.DoCommandTestConfig{
		ComponentName:   componentName,
		CaptureInterval: captureInterval,
		DoCommandMap:    doCommandMap,
		Collector:       motion.NewDoCommandCollector,
		ResourceFactory: func() interface{} { return newMotion() },
	})
}

func newMotion() motion.Service {
	m := &inject.MotionService{}
	m.DoCommandFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
		return doCommandMap, nil
	}
	return m
}
