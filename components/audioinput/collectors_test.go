package audioinput_test

import (
	"context"
	"testing"
	"time"

	audioinput "go.viam.com/rdk/components/audioinput"
	datatu "go.viam.com/rdk/data/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	componentName   = "audioinput"
	captureInterval = time.Millisecond
)

var doCommandMap = map[string]any{"readings": "random-test"}

func TestDoCommandCollector(t *testing.T) {
	datatu.TestDoCommandCollector(t, datatu.DoCommandTestConfig{
		ComponentName:   componentName,
		CaptureInterval: captureInterval,
		DoCommandMap:    doCommandMap,
		Collector:       audioinput.NewDoCommandCollector,
		ResourceFactory: func() interface{} { return newAudioInput() },
	})
}

func newAudioInput() audioinput.AudioInput {
	ai := &inject.AudioInput{}
	ai.DoFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
		return doCommandMap, nil
	}
	return ai
}
