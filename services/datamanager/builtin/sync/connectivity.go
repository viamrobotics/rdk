package sync

import (
	"context"
	"runtime"
	"time"

	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/connectivity"

	"go.viam.com/rdk/grpc"
)

// ConnToConnectivityState takes an rpc.ClientConn and returns an object
// that implents GetStatus so that the state of the connection (whether it is healthy or not)
// can be monitored.
// NOTE: Currently, as a temporary measure, this is done by trying to make a new tcp connection
// with app.viam.com, but once https://viam.atlassian.net/browse/DATA-3009 is
// unblocked & implemented, we will either remove this function or have it call the appropriate
// methods on conn.
func ConnToConnectivityState(conn rpc.ClientConn) grpc.ConnectivityState {
	return offlineChecker{}
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
		// Use DialDirectGRPC to make a connection to app.viam.com instead of a
		// basic net.Dial in order to ensure that the connection can be made
		// behind wifi or the BLE-SOCKS bridge (DialDirectGRPC can dial through
		// the BLE-SOCKS bridge.)
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		conn, err := rpc.DialDirectGRPC(ctx, "app.viam.com:443", nil)
		cancel()
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
