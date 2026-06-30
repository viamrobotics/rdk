package status

import (
	"errors"
	"testing"
	"time"

	pb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
)

func TestStatusToProto(t *testing.T) {
	lastUpdated := time.Unix(1700000000, 0).UTC()

	t.Run("all fields", func(t *testing.T) {
		s := Status{
			Name:                "my-module",
			State:               ModuleStateUnhealthy,
			LastUpdated:         lastUpdated,
			Error:               errors.New("boom"),
			ConsecutiveFailures: 3,
		}
		proto := s.ToProto()
		test.That(t, proto.ModuleName, test.ShouldEqual, "my-module")
		test.That(t, proto.State, test.ShouldEqual, pb.ModuleStatus_STATE_UNHEALTHY)
		test.That(t, proto.LastUpdated.AsTime().Equal(lastUpdated), test.ShouldBeTrue)
		test.That(t, proto.Error, test.ShouldEqual, "boom")
		test.That(t, proto.ConsecutiveFailures, test.ShouldEqual, uint32(3))
	})

	t.Run("nil error yields empty string", func(t *testing.T) {
		proto := Status{Name: "m", State: ModuleStateReady}.ToProto()
		test.That(t, proto.Error, test.ShouldEqual, "")
	})

	t.Run("every state maps", func(t *testing.T) {
		for state, expected := range map[State]pb.ModuleStatus_State{
			ModuleStateUnknown:   pb.ModuleStatus_STATE_UNSPECIFIED,
			ModuleStatePending:   pb.ModuleStatus_STATE_PENDING,
			ModuleStateStarting:  pb.ModuleStatus_STATE_STARTING,
			ModuleStateReady:     pb.ModuleStatus_STATE_READY,
			ModuleStateUnhealthy: pb.ModuleStatus_STATE_UNHEALTHY,
			ModuleStateClosing:   pb.ModuleStatus_STATE_CLOSING,
		} {
			test.That(t, Status{State: state}.ToProto().State, test.ShouldEqual, expected)
		}
	})
}
