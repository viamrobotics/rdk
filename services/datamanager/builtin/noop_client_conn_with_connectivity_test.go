package builtin

import (
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/connectivity"
)

func newReadyNoOpClientConnWithConnectivity() rpc.ClientConn {
	return newNoOpClientConnWithConnectivity(func() connectivity.State { return connectivity.Ready })
}

func newConnectingNoOpClientConnWithConnectivity() rpc.ClientConn {
	return newNoOpClientConnWithConnectivity(func() connectivity.State { return connectivity.Connecting })
}

func newNoOpClientConnWithConnectivity(f func() connectivity.State) rpc.ClientConn {
	return &noOpClientConnWithConnectivity{getStateFunc: f}
}

type noOpClientConnWithConnectivity struct {
	getStateFunc func() connectivity.State
	noOpClientConn
}

func (n *noOpClientConnWithConnectivity) GetState() connectivity.State {
	return n.getStateFunc()
}
