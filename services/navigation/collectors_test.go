package navigation_test

import (
	"context"
	"testing"
	"time"

	datatu "go.viam.com/rdk/data/testutils"
	navigation "go.viam.com/rdk/services/navigation"
	"go.viam.com/rdk/testutils/inject"
)

const (
	componentName   = "navigation"
	captureInterval = time.Millisecond
)

var doCommandMap = map[string]any{"readings": "random-test"}

func TestDoCommandCollector(t *testing.T) {
	datatu.TestDoCommandCollector(t, datatu.DoCommandTestConfig{
		ComponentName:   componentName,
		CaptureInterval: captureInterval,
		DoCommandMap:    doCommandMap,
		Collector:       navigation.NewDoCommandCollector,
		ResourceFactory: func() interface{} { return newNavigation() },
	})
}

func newNavigation() navigation.Service {
	n := &inject.NavigationService{}
	n.DoCommandFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
		return doCommandMap, nil
	}
	return n
}
