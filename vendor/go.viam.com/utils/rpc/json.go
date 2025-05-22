package rpc

import (
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/protobuf/encoding/protojson"
)

// JSONPB are the JSON protobuf options we use globally.
var JSONPB = &runtime.JSONPb{
	MarshalOptions: protojson.MarshalOptions{
		UseProtoNames:   true,
		EmitUnpopulated: true,
	},
}
