// Package shell contains a shell service, along with a gRPC server and client
package shell

import (
	"context"

	servicepb "go.viam.com/api/service/shell/v1"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Service]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           servicepb.RegisterShellServiceHandlerFromEndpoint,
		RPCServiceDesc:              &servicepb.ShellService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: doCommand.String(),
	}, newDoCommandCollector)
}

// A Service handles shells for a local robot.
type Service interface {
	resource.Resource
	Shell(ctx context.Context, extra map[string]interface{}) (
		input chan<- string, oobInput chan<- map[string]interface{}, output <-chan Output, retErr error)

	// CopyFilesToMachines copies a stream of files from a client to the connected-to machine.
	// Initially, metadata is sent to describe the destination in the filesystem in addition
	// to what kind of file(s) are being sent. A FileCopier is returned that can be used
	// to copy files one by one-by-one. When files are done being copied, the returned FileCopier
	// MUST be closed. Returning a FileCopier over passing in the files was chosen so as to promote
	// the streaming of files, less copies of memory, and less open files.
	CopyFilesToMachine(
		ctx context.Context,
		sourceType CopyFilesSourceType,
		destination string,
		preserve bool,
		extra map[string]interface{},
	) (FileCopier, error)

	// CopyFilesFromMachine copies a stream of files from a connected-to machine to the calling client.
	// Essentially, it is the inverse of CopyFilesToMachine. The FileCopyFactory passed in will be
	// called once the service knows what kinds of files are being transmitted and that FileCopier
	// will be passed all the files to be copied one-by-one. The method will return once all files
	// are copied or an error happens in between.
	CopyFilesFromMachine(
		ctx context.Context,
		paths []string,
		allowRecursion bool,
		preserve bool,
		copyFactory FileCopyFactory,
		extra map[string]interface{},
	) error
}

// Output reflects an instance of shell output on either stdout or stderr.
type Output struct {
	Output string // reflects stdout
	Error  string // reflects stderr
	EOF    bool
}

// SubtypeName is the name of the type of service.
const SubtypeName = "shell"

// API is a variable that identifies the shell service resource API.
var API = resource.APINamespaceRDK.WithServiceType(SubtypeName)

// Named is a helper for getting the named service's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}
