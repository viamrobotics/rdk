package gpsutils

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
)

func TestConnectInvalidURL(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cancelCtx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	// Note: if maxConnectAttempts is 0, we don't bother trying to connect.
	ntripInfo := &NtripInfo{maxConnectAttempts: 1}
	err := ntripInfo.Connect(cancelCtx, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `address must start with http://`)
}

func TestConnectSucceeds(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cancelCtx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	config := NtripConfig{
		NtripURL:             "http://fakeurl",
		NtripConnectAttempts: 10,
		NtripMountpoint:      "",
		NtripUser:            "user",
		NtripPass:            "pwd",
	}

	ntripInfo, err := NewNtripInfo(&config, logger)
	test.That(t, err, test.ShouldBeNil)

	// Create the connection but don't wait for it to become live.
	err = ntripInfo.createConnection(cancelCtx, logger)
	test.That(t, err, test.ShouldBeNil)
}
