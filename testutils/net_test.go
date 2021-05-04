package testutils

import (
	"net"
	"testing"
	"time"

	"go.viam.com/test"
)

func TestWaitSuccessfulDial(t *testing.T) {
	listener, err := net.Listen("tcp", ":0")
	test.That(t, err, test.ShouldBeNil)
	stop := make(chan struct{})
	defer func() {
		close(stop)
	}()
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			listener.Accept()
		}
	}()

	prevWaitDur := waitDur
	defer func() {
		waitDur = prevWaitDur
	}()
	waitDur = 50 * time.Millisecond
	test.That(t, WaitSuccessfulDial(listener.Addr().String()), test.ShouldBeNil)
	err = WaitSuccessfulDial("localhost:222")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "dial")
}
