package testutils

import (
	"net"
	"testing"

	"github.com/edaniels/test"
)

func TestWaitSuccessfulDial(t *testing.T) {
	listener, err := net.Listen("tcp", ":0")
	test.That(t, err, test.ShouldBeNil)
	go func() {
		for {
			listener.Accept()
		}
	}()

	test.That(t, WaitSuccessfulDial(listener.Addr().String()), test.ShouldBeNil)
	err = WaitSuccessfulDial("localhost:222")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "dial")
}
