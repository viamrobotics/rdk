package gpsrtkserial

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	rtk "go.viam.com/rdk/components/movementsensor/rtkutils"
	"go.viam.com/test"
	"go.viam.com/utils"
)

// mock ntripinfo client.
func makeMockNtripClient() *rtk.NtripInfo {
	return &rtk.NtripInfo{}
}

func TestValidateRTK(t *testing.T) {
	path := "path"
	fakecfg := &Config{
		NtripURL:             "",
		NtripConnectAttempts: 10,
		NtripPass:            "somepass",
		NtripUser:            "someuser",
		NtripMountpoint:      "NYC",
		SerialPath:           path,
		SerialBaudRate:       3600,
	}
	_, err := fakecfg.Validate(path)
	test.That(
		t,
		err,
		test.ShouldBeError,
		utils.NewConfigValidationFieldRequiredError(path, "ntrip_url"))

	fakecfg.NtripURL = "asdfg"
	_, err = fakecfg.Validate(path)
	test.That(
		t,
		err,
		test.ShouldBeNil)
	_, err = fakecfg.Validate("path")
	test.That(t, err, test.ShouldBeNil)
}

func TestConnect(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := rtkSerial{
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		logger:     logger,
	}

	url := "http://fakeurl"
	username := "user"
	password := "pwd"

	// create new ntrip client and connect
	err := g.connect("invalidurl", username, password, 10)
	g.ntripClient = makeMockNtripClient()

	test.That(t, err, test.ShouldNotBeNil)

	err = g.connect(url, username, password, 10)
	test.That(t, err, test.ShouldBeNil)

	err = g.getStream("", 10)
	test.That(t, err, test.ShouldNotBeNil)
}
