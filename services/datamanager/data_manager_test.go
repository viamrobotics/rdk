package datamanager

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/testutils/inject"
)

func TestNewDataManager(t *testing.T) {
	cfg := &Config{
		CaptureDir: "/path/to/capture",
		ComponentAttributes: map[string]componentAttributes{
			"imu1": {
				Type:               "imu",
				Method:             "ReadAngularVelocity",
				CaptureFrequencyHz: 10,
				AdditionalParams: map[string]string{
					"param1": "thing",
					"param2": "thing2",
				},
			},
		},
	}
	cfgService := config.Service{
		Type:                "data_manager",
		ConvertedAttributes: cfg,
	}

	logger := golog.NewTestLogger(t)
	r := &inject.Robot{}

	_, err := New(context.Background(), r, cfgService, logger)

	test.That(t, err, test.ShouldBeNil)
}

func TestCheckStorage(t *testing.T) {
	cfg := &Config{
		CaptureDir:        "/path/to/capture",
		MaxStoragePercent: 80,
		EnableAutoDelete:  false,
		ComponentAttributes: map[string]componentAttributes{
			"imu1": {
				Type:               "imu",
				Method:             "ReadAngularVelocity",
				CaptureFrequencyHz: 10,
				AdditionalParams: map[string]string{
					"param1": "thing",
					"param2": "thing2",
				},
			},
		},
	}
	cfgService := config.Service{
		Type:                "data_manager",
		ConvertedAttributes: cfg,
	}

	logger := golog.NewTestLogger(t)
	r := &inject.Robot{}

	_, err := New(context.Background(), r, cfgService, logger)

	test.That(t, err, test.ShouldBeNil)
}
