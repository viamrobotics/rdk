// Package grpchelpers provides some grpc helper utilities.
package grpchelpers

import "google.golang.org/grpc/connectivity"

// ConnectivityState allows callers to check the connectivity state of
// the connection
// see https://github.com/grpc/grpc-go/blob/master/clientconn.go#L648
type ConnectivityState interface {
	GetState() connectivity.State
}
