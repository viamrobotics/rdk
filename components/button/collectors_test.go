package button_test

import (
	"context"
	"testing"
	"time"

	button "go.viam.com/rdk/components/button"
	datatu "go.viam.com/rdk/data/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	componentName   = "button"
	captureInterval = time.Millisecond
)

var doCommandMap = map[string]any{"readings": "random-test"}

func TestDoCommandCollector(t *testing.T) {
	datatu.TestDoCommandCollector(t, datatu.DoCommandTestConfig{
		ComponentName:   componentName,
		CaptureInterval: captureInterval,
		DoCommandMap:    doCommandMap,
		Collector:       button.NewDoCommandCollector,
		ResourceFactory: func() interface{} { return newButton() },
	})
}

func newButton() button.Button {
	b := &inject.Button{}
	b.DoFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
		return doCommandMap, nil
	}
	return b
}
