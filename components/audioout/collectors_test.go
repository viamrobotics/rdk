package audioout_test

import (
	"testing"
	"time"

	audioout "go.viam.com/rdk/components/audioout"
	datatu "go.viam.com/rdk/data/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	componentName   = "audioout"
	captureInterval = time.Millisecond
)

func TestGetWorldPoseCollector(t *testing.T) {
	datatu.TestGetWorldPoseCollector(t, datatu.GetWorldPoseTestConfig{
		ComponentName:   componentName,
		CaptureInterval: captureInterval,
		Collector:       audioout.NewGetWorldPoseCollector,
		ResourceFactory: func() interface{} { return &inject.AudioOut{} },
	})
}
