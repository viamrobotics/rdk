package nmea

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

func TestConnect(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := RTKGPS{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}
	url := "http://rtn.dot.ny.gov:8082"
	username := "evelyn"
	password := "checkmate"
	mountPoint := "NJI2"

	// create new ntrip client and connect
	err := g.Connect(url, username, password, 10)
	test.That(t, err, test.ShouldBeNil)

	err = g.GetStream("", 10)
	test.That(t, err, test.ShouldNotBeNil)

	err = g.GetStream(mountPoint, 10)
	test.That(t, err, test.ShouldBeNil)
}
