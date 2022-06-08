package datamanager_test

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/testutils/inject"
)

func TestNewDataManager(t *testing.T) {
	cfg := &datamanager.Config{
		CaptureDir: "/path/to/capture",
	}
	cfgService := config.Service{
		Type:                "data_manager",
		ConvertedAttributes: cfg,
	}

	logger := golog.NewTestLogger(t)
	r := &inject.Robot{}

	_, err := datamanager.New(context.Background(), r, cfgService, logger)

	test.That(t, err, test.ShouldBeNil)
}
