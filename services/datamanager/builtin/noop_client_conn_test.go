package builtin

import (
	"context"

	"github.com/viamrobotics/webrtc/v3"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"
)

func newNoOpClientConn() rpc.ClientConn {
	return &noOpClientConn{}
}

type noOpClientConn struct{}

func (*noOpClientConn) NewStream(
	ctx context.Context,
	desc *grpc.StreamDesc,
	method string,
	opts ...grpc.CallOption,
) (grpc.ClientStream, error) {
	return &noOpClientStream{}, nil
}

func (*noOpClientConn) Invoke(
	ctx context.Context,
	method string,
	args any,
	reply any,
	opts ...grpc.CallOption,
) error {
	return nil
}

func (*noOpClientConn) PeerConn() *webrtc.PeerConnection {
	return nil
}
func (*noOpClientConn) Close() error {
	return nil
}
