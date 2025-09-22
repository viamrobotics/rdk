package sync

import (
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
func ConnToConnectivityState(conn rpc.ClientConn) rpc.ClientConn {
	return conn
}

// ConnectivityState allows callers to check the connectivity state of
// the connection
// see https://github.com/grpc/grpc-go/blob/master/clientconn.go#L648
type ConnectivityState interface {
	GetState() connectivity.State
}
