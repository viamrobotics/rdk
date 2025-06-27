package input_test

import (
	"context"
	"testing"
	"time"

	input "go.viam.com/rdk/components/input"
	tu "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	componentName   = "input"
	captureInterval = time.Millisecond
)

var doCommandMap = map[string]any{"readings": "random-test"}

func TestDoCommandCollector(t *testing.T) {
	tu.TestDoCommandCollector(t, tu.DoCommandTestConfig{
		ComponentName:   componentName,
		CaptureInterval: captureInterval,
		DoCommandMap:    doCommandMap,
		Collector:       input.NewDoCommandCollector,
		ResourceFactory: func() interface{} { return newInput() },
	})
}

func newInput() input.Controller {
	i := &inject.InputController{}
	i.DoFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
		return doCommandMap, nil
	}
	return i
}
