package generic_test

import (
	"context"
	"testing"
	"time"

	datatu "go.viam.com/rdk/data/testutils"
	generic "go.viam.com/rdk/services/generic"
	"go.viam.com/rdk/testutils/inject"
)

const (
	componentName   = "generic"
	captureInterval = time.Millisecond
)

var doCommandMap = map[string]any{"readings": "random-test"}

func TestDoCommandCollector(t *testing.T) {
	datatu.TestDoCommandCollector(t, datatu.DoCommandTestConfig{
		ComponentName:   componentName,
		CaptureInterval: captureInterval,
		DoCommandMap:    doCommandMap,
		Collector:       generic.NewDoCommandCollector,
		ResourceFactory: func() interface{} { return newGeneric() },
	})
}

func newGeneric() generic.Service {
	g := &inject.GenericService{}
	g.DoFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
		return doCommandMap, nil
	}
	return g
}
