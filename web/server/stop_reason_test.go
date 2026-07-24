package server

import (
	"testing"

	"go.viam.com/test"
)

func TestRecordStopReasonFirstWins(t *testing.T) {
	var s robotServer
	test.That(t, s.stopReason.Load(), test.ShouldBeNil)

	s.recordStopReason("app_restart")
	s.recordStopReason("shutdown_request")

	reason := s.stopReason.Load()
	test.That(t, reason, test.ShouldNotBeNil)
	test.That(t, *reason, test.ShouldEqual, "app_restart")
}
