package discovery_test

import (
	"context"
	"testing"
	"time"

	datatu "go.viam.com/rdk/data/testutils"
	discovery "go.viam.com/rdk/services/discovery"
	"go.viam.com/rdk/testutils/inject"
)

const (
	componentName   = "discovery"
	captureInterval = time.Millisecond
)

var doCommandMap = map[string]any{"readings": "random-test"}

func TestDoCommandCollector(t *testing.T) {
	datatu.TestDoCommandCollector(t, datatu.DoCommandTestConfig{
		ComponentName:   componentName,
		CaptureInterval: captureInterval,
		DoCommandMap:    doCommandMap,
		Collector:       discovery.NewDoCommandCollector,
		ResourceFactory: func() interface{} { return newDiscovery() },
	})
}

func newDiscovery() discovery.Service {
	d := &inject.DiscoveryService{}
	d.DoFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
		return doCommandMap, nil
	}
	return d
}
