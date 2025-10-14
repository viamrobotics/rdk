package builtin

import (
	"context"

	"github.com/viamrobotics/webrtc/v3"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"

	datasync "go.viam.com/rdk/services/datamanager/builtin/sync"
)

func NoOpClientConnReady(rpc.ClientConn) rpc.ClientConn {
	return &noOpClientConnWithConnectivity{getStateFunc: func() connectivity.State { return connectivity.Ready }}
}

func noOpClientConnError(rpc.ClientConn) rpc.ClientConn {
	return &noOpClientConnWithConnectivity{getStateFunc: func() connectivity.State { return connectivity.TransientFailure }}
}

type noOpClientConnWithConnectivity struct {
	getStateFunc func() connectivity.State
	NoOpClientConn
}

func (n *noOpClientConnWithConnectivity) GetState() connectivity.State {
	return n.getStateFunc()
}

func NewNoOpClientConn() rpc.ClientConn {
	return &NoOpClientConn{}
}

type NoOpClientConn struct{}

func (*NoOpClientConn) NewStream(
	ctx context.Context,
	desc *grpc.StreamDesc,
	method string,
	opts ...grpc.CallOption,
) (grpc.ClientStream, error) {
	return &datasync.NoOpClientStream{}, nil
}

func (*NoOpClientConn) Invoke(
	ctx context.Context,
	method string,
	args any,
	reply any,
	opts ...grpc.CallOption,
) error {
	return nil
}

func (*NoOpClientConn) PeerConn() *webrtc.PeerConnection {
	return nil
}

func (*NoOpClientConn) GetState() connectivity.State {
	return connectivity.Idle
}

func (*NoOpClientConn) Close() error {
	return nil
}
