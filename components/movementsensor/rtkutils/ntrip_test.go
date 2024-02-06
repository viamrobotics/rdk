package rtkutils

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
)

func TestConnect(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cancelCtx, _ := context.WithCancel(context.Background())

	ntripInfo := &NtripInfo{MaxConnectAttempts: 1}
	err := ntripInfo.Connect(cancelCtx, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `address must start with http://`)

	config := NtripConfig{
		NtripURL: "http://fakeurl",
		NtripConnectAttempts: 10,
		NtripMountpoint: "",
		NtripPass: "pwd",
		NtripUser: "user",
	}

	ntripInfo, err = NewNtripInfo(&config, logger)
	test.That(t, err, test.ShouldBeNil)

	err = ntripInfo.Connect(cancelCtx, logger)
	test.That(t, err, test.ShouldBeNil)
}
