package generic_test

import (
	"context"
	"testing"
	"time"

	generic "go.viam.com/rdk/components/generic"
	tu "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	componentName   = "generic"
	captureInterval = time.Millisecond
)

var doCommandMap = map[string]any{"readings": "random-test"}

func TestDoCommandCollector(t *testing.T) {
	tu.TestDoCommandCollector(t, tu.DoCommandTestConfig{
		ComponentName:   componentName,
		CaptureInterval: captureInterval,
		DoCommandMap:    doCommandMap,
		Collector:       generic.NewDoCommandCollector,
		ResourceFactory: func() interface{} { return newResource() },
	})
}

func newResource() generic.Resource {
	g := &inject.GenericComponent{}
	g.DoFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
		return doCommandMap, nil
	}
	return g
}
