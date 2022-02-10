package spatialmath

import (
	"encoding/json"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func TestVolumeSerialization(t *testing.T) {
	testCases := []struct {
		name    string
		config  VolumeConfig
		success bool
	}{
		{"box", VolumeConfig{Type: "box", X: 1, Y: 1, Z: 1}, true},
		{"box bad dims", VolumeConfig{Type: "box", X: 1, Y: 0, Z: 1}, false},
		{"sphere", VolumeConfig{Type: "sphere", R: 1}, true},
		{"sphere bad dims", VolumeConfig{Type: "sphere", R: -1}, false},
		{"bad infer", VolumeConfig{}, false},
		{"bad type", VolumeConfig{Type: "bad"}, false},
	}

	pose := NewPoseFromPoint(r3.Vector{X: 1, Y: 1, Z: 1})
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			vc, err := testCase.config.ParseConfig()
			if testCase.success == false {
				test.That(t, err, test.ShouldNotBeNil)
				return
			}
			test.That(t, err, test.ShouldBeNil)
			data, err := vc.MarshalJSON()
			test.That(t, err, test.ShouldBeNil)
			config := VolumeConfig{}
			err = json.Unmarshal(data, &config)
			test.That(t, err, test.ShouldBeNil)
			newVc, err := config.ParseConfig()
			test.That(t, err, test.ShouldBeNil)
			test.That(t, vc.NewVolume(pose).AlmostEqual(newVc.NewVolume(pose)), test.ShouldBeTrue)
		})
	}
}
