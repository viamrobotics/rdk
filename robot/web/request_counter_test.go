package web

import (
	"context"
	"testing"

	"github.com/viamrobotics/webrtc/v3"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
)

func TestConnectionActivityEvents(t *testing.T) {
	logger := logging.NewTestLogger(t)
	activityLogs := logging.NewObservedActivityLogger(t, logger)

	rc := RequestCounter{logger: logger}

	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, pc.GracefulClose(), test.ShouldBeNil)
	}()

	ctx := context.Background()
	rc.setClientMetadataForPC(ctx, pc)
	// Later requests over the same connection add no events.
	rc.setClientMetadataForPC(ctx, pc)

	all := activityLogs.All()
	test.That(t, len(all), test.ShouldEqual, 1)
	connect := all[0].ContextMap()
	test.That(t, connect["activity"], test.ShouldEqual, "connection")
	test.That(t, connect["event"], test.ShouldEqual, "connect")
	test.That(t, connect["client"], test.ShouldEqual, "maybe-typescript;unknown;unknown")
}
