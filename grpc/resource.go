package grpc

import (
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/resource"
)

// UnimplementedError is returned any time a gRPC method is unimplemented.
var UnimplementedError = status.Error(codes.Unimplemented, codes.Unimplemented.String())

// An ForeignResource is used to dynamically invoke RPC calls to resources that have their
// RPC information dervied on demand.
type ForeignResource struct {
	name resource.Name
	conn rpc.ClientConn
}

// NewForeignResource returns an ForeignResource for the given resource name and
// connection serving it.
func NewForeignResource(name resource.Name, conn rpc.ClientConn) *ForeignResource {
	return &ForeignResource{name, conn}
}

// Name returns the name of the resource.
func (res *ForeignResource) Name() resource.Name {
	return res.name
}

// NewStub returns a new gRPC client stub used to communicate with the resource.
func (res *ForeignResource) NewStub() grpcdynamic.Stub {
	return grpcdynamic.NewStub(res.conn)
}
