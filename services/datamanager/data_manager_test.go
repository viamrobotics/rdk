package datamanager

import (
	"context"
	"syscall"
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

type MockSysCallImplementation struct{}

func (MockSysCallImplementation) Statfs(path string, stat *syscall.Statfs_t) error {
	return syscall.Statfs(path, stat)
}

func TestDiskUsage(t *testing.T) {

}
