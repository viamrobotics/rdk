package testutils

import (
	"net"
	"testing"
	"time"

	"go.viam.com/test"
	goutils "go.viam.com/utils"
)

// HoldPortListener keeps a server's port bound across a restart so a follow-up
// server can reuse the exact same socket with no window for another process to
// claim the port in between. A server stops serving by calling Close on its
// listener (http.Server.Shutdown or grpc.Server.Stop both do this); HoldPort-
// Listener's Close arms a past deadline instead of closing the socket, so the
// port stays reserved. The server still shuts down cleanly because its stop
// method also sets an internal quit flag that ends the serve loop — the deadline
// just lets the in-progress Accept return so the loop can notice. Re-arm with
// Rearm before serving on it again.
type HoldPortListener struct {
	*net.TCPListener
}

// Close is invoked by the server's stop method. Arm a past deadline to unblock
// Accept instead of closing the socket, so the port stays bound.
func (h *HoldPortListener) Close() error {
	return h.SetDeadline(time.Unix(1, 0))
}

// Rearm clears the deadline so the next server can accept on the listener. Call
// it after the previous server has stopped and before serving again.
func (h *HoldPortListener) Rearm(tb testing.TB) {
	tb.Helper()
	test.That(tb, h.SetDeadline(time.Time{}), test.ShouldBeNil)
}

// HoldPort wraps lis (which must be a *net.TCPListener) so its port survives a
// server restart, and registers cleanup to truly close the socket at test end.
func HoldPort(tb testing.TB, lis net.Listener) *HoldPortListener {
	tb.Helper()
	tcp, ok := lis.(*net.TCPListener)
	test.That(tb, ok, test.ShouldBeTrue)
	h := &HoldPortListener{tcp}
	tb.Cleanup(func() { goutils.UncheckedError(h.TCPListener.Close()) })
	return h
}
