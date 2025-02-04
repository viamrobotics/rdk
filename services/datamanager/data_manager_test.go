package datamanager

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
)

func TestDataCaptureConfig(t *testing.T) {
	type testCase struct {
		name  string
		a     *DataCaptureConfig
		b     *DataCaptureConfig
		equal bool
	}

	empty := &DataCaptureConfig{}
	full := &DataCaptureConfig{
		Name:               arm.Named("arm1"),
		Method:             "Position",
		CaptureFrequencyHz: 1.0,
		CaptureQueueSize:   100,
		CaptureBufferSize:  100,
		AdditionalParams:   map[string]string{"a": "b", "c": "d"},
		Disabled:           true,
		Tags:               []string{"e", "f", "g"},
		CaptureDirectory:   "/tmp/some/capture",
	}
	tcs := []testCase{
		{
			name:  "the same empty config is equal to itself",
			a:     empty,
			b:     empty,
			equal: true,
		},
		{
			name:  "different empty configs are equal",
			a:     &DataCaptureConfig{},
			b:     &DataCaptureConfig{},
			equal: true,
		},
		{
			name:  "the same full config is equal to itself",
			a:     full,
			b:     full,
			equal: true,
		},
		{
			name: "full configs that are equal are equal",
			a: &DataCaptureConfig{
				Name:               arm.Named("arm1"),
				Method:             "Position",
				CaptureFrequencyHz: 1.0,
				CaptureQueueSize:   100,
				CaptureBufferSize:  100,
				AdditionalParams:   map[string]string{"a": "b", "c": "d"},
				Disabled:           true,
				Tags:               []string{"e", "f", "g"},
				CaptureDirectory:   "/tmp/some/capture",
			},
			b: &DataCaptureConfig{
				Name:               arm.Named("arm1"),
				Method:             "Position",
				CaptureFrequencyHz: 1.0,
				CaptureQueueSize:   100,
				CaptureBufferSize:  100,
				AdditionalParams:   map[string]string{"a": "b", "c": "d"},
				Disabled:           true,
				Tags:               []string{"e", "f", "g"},
				CaptureDirectory:   "/tmp/some/capture",
			},
			equal: true,
		},
		{
			name: "different names are not equal",
			a: &DataCaptureConfig{
				Name: arm.Named("arm1"),
			},
			b: &DataCaptureConfig{
				Name: arm.Named("arm2"),
			},
			equal: false,
		},
		{
			name: "different methods are not equal",
			a: &DataCaptureConfig{
				Method: "Position",
			},
			b: &DataCaptureConfig{
				Method: "NotPosition",
			},
			equal: false,
		},
		{
			name: "different CaptureFrequencyHz are not equal",
			a: &DataCaptureConfig{
				CaptureFrequencyHz: 1.0,
			},
			b: &DataCaptureConfig{
				CaptureFrequencyHz: 1.1,
			},
			equal: false,
		},
		{
			name: "different CaptureQueueSize are not equal",
			a: &DataCaptureConfig{
				CaptureQueueSize: 100,
			},
			b: &DataCaptureConfig{
				CaptureQueueSize: 101,
			},
			equal: false,
		},
		{
			name: "different CaptureBufferSize are not equal",
			a: &DataCaptureConfig{
				CaptureBufferSize: 100,
			},
			b: &DataCaptureConfig{
				CaptureBufferSize: 101,
			},
			equal: false,
		},
		{
			name: "different AdditionalParams are not equal",
			a: &DataCaptureConfig{
				AdditionalParams: map[string]string{"a": "b", "c": "d"},
			},
			b: &DataCaptureConfig{
				AdditionalParams: map[string]string{"a": "b"},
			},
			equal: false,
		},
		{
			name: "different Disabled are not equal",
			a: &DataCaptureConfig{
				Disabled:         true,
				Tags:             []string{"e", "f", "g"},
				CaptureDirectory: "/tmp/some/capture",
			},
			b: &DataCaptureConfig{
				Disabled: false,
			},
			equal: false,
		},
		{
			name: "different Tags are not equal",
			a: &DataCaptureConfig{
				Tags: []string{"e", "f", "g"},
			},
			b: &DataCaptureConfig{
				Tags: []string{"e", "f"},
			},
			equal: false,
		},
		{
			name: "different CaptureDirectory are not equal",
			a: &DataCaptureConfig{
				CaptureDirectory: "/tmp/some/capture",
			},
			b: &DataCaptureConfig{
				CaptureDirectory: "/tmp/some/other/capture",
			},
			equal: false,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			test.That(t, tc.a.Equals(tc.b), test.ShouldEqual, tc.equal)
			test.That(t, tc.b.Equals(tc.a), test.ShouldEqual, tc.equal)
		})
	}
}
