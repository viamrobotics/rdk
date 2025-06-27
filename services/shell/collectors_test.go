package shell_test

import (
	"context"
	"testing"
	"time"

	shell "go.viam.com/rdk/services/shell"
	tu "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	componentName   = "shell"
	captureInterval = time.Millisecond
)

var doCommandMap = map[string]any{"readings": "random-test"}

func TestDoCommandCollector(t *testing.T) {
	tu.TestDoCommandCollector(t, tu.DoCommandTestConfig{
		ComponentName:   componentName,
		CaptureInterval: captureInterval,
		DoCommandMap:    doCommandMap,
		Collector:       shell.NewDoCommandCollector,
		ResourceFactory: func() interface{} { return newShell() },
	})
}

func newShell() shell.Service {
	s := &inject.ShellService{}
	s.DoCommandFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
		return doCommandMap, nil
	}
	return s
}
