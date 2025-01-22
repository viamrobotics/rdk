package sync

import (
	"net"
	"runtime"
	"time"

	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/connectivity"
)

// ConnToConnectivityState takes an rpc.ClientConn and returns an object
// that implents GetStatus so that the state of the connection (whether it is healthy or not)
// can be monitored.
// NOTE: Currently, as a temporary measure, this is done by trying to make a new tcp connection
// with app.viam.com, but once https://viam.atlassian.net/browse/DATA-3009 is
// unblocked & implemented, we will either remove this function or have it call the appropriate
// methods on conn.
func ConnToConnectivityState(conn rpc.ClientConn) ConnectivityState {
	return offlineChecker{}
}

// ConnectivityState allows callers to check the connectivity state of
// the connection
// see https://github.com/grpc/grpc-go/blob/master/clientconn.go#L648
type ConnectivityState interface {
	GetState() connectivity.State
}

type offlineChecker struct{}

func (oc offlineChecker) GetState() connectivity.State {
	if isOffline() {
		return connectivity.TransientFailure
	}
	return connectivity.Ready
}

// returns true if the device is offline.
func isOffline() bool {
	timeout := 5 * time.Second
	attempts := 1
	if runtime.GOOS == "windows" {
		// TODO(RSDK-8344): this is temporary as we 1) debug connectivity issues on windows,
		// and 2) migrate to using the native checks on the underlying connection.
		timeout = 15 * time.Second
		attempts = 2
	}
	for i := range attempts {
		conn, err := net.DialTimeout("tcp", "app.viam.com:443", timeout)
		if err == nil {
			conn.Close() //nolint:gosec,errcheck
			return false
		}
		if i < attempts-1 {
			time.Sleep(time.Second)
		}
	}
	return true
}
