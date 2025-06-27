package posetracker_test

import (
	"context"
	"testing"
	"time"

	posetracker "go.viam.com/rdk/components/posetracker"
	datatu "go.viam.com/rdk/data/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	componentName   = "posetracker"
	captureInterval = time.Millisecond
)

var doCommandMap = map[string]any{"readings": "random-test"}

func TestDoCommandCollector(t *testing.T) {
	datatu.TestDoCommandCollector(t, datatu.DoCommandTestConfig{
		ComponentName:   componentName,
		CaptureInterval: captureInterval,
		DoCommandMap:    doCommandMap,
		Collector:       posetracker.NewDoCommandCollector,
		ResourceFactory: func() interface{} { return newPoseTracker() },
	})
}

func newPoseTracker() posetracker.PoseTracker {
	p := &inject.PoseTracker{}
	p.DoFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
		return doCommandMap, nil
	}
	return p
}
