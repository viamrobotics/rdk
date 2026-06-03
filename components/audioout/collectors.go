package audioout

import (
	"go.viam.com/rdk/data"
)

type method int64

const (
	getWorldPose method = iota
)

func (m method) String() string {
	if m == getWorldPose {
		return "GetWorldPose"
	}
	return "Unknown"
}

// newGetWorldPoseCollector returns a collector to capture the audio output's world-space pose via the frame system.
// If one is already registered with the same MethodMetadata it will panic.
func newGetWorldPoseCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	if _, err := assertAudioOut(resource); err != nil {
		return nil, err
	}
	cFunc, err := data.NewGetWorldPoseCaptureFunc(params)
	if err != nil {
		return nil, err
	}
	return data.NewCollector(cFunc, params)
}

func assertAudioOut(resource interface{}) (AudioOut, error) {
	audioOut, ok := resource.(AudioOut)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}
	return audioOut, nil
}
