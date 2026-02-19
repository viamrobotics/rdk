// Package modutil provides lightweight module utility types and functions
// that can be imported without pulling in the heavy blanket API registrations
// from the module package (components/register_apis, services/register_apis).
package modutil

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/fullstorydev/grpcurl"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/grpcreflect"
	"go.uber.org/multierr"
	pb "go.viam.com/api/module/v1"
	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/utils/rpc"
	reflectpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/discovery"
)

const (
	socketSuffix = ".sock"
	// socketHashSuffixLength determines how many characters from the module's name's hash should be used when truncating the module socket.
	socketHashSuffixLength int = 5
	// socketMaxAddressLength is the length (-1 for null terminator) of the .sun_path field as used in kernel bind()/connect() syscalls.
	// Linux allows for a max length of 107 but to simplify this code, we truncate to the macOS limit of 103.
	socketMaxAddressLength int = 103

	// NoModuleParentEnvVar indicates whether there is a parent for a module being started.
	NoModuleParentEnvVar = "VIAM_NO_MODULE_PARENT"
)

// CreateSocketAddress returns a socket address of the form parentDir/desiredName.sock
// if it is shorter than the socketMaxAddressLength. If this path would be too long, this function
// truncates desiredName and returns parentDir/truncatedName-hashOfDesiredName.sock.
//
// Importantly, this function will return the same socket address as long as the desiredName doesn't change.
func CreateSocketAddress(parentDir, desiredName string) (string, error) {
	baseAddr := filepath.ToSlash(parentDir)
	numRemainingChars := socketMaxAddressLength -
		len(baseAddr) -
		len(socketSuffix) -
		1 // `/` between baseAddr and name
	if numRemainingChars < len(desiredName) && numRemainingChars < socketHashSuffixLength+1 {
		return "", fmt.Errorf("module socket base path would result in a path greater than the OS limit of %d characters: %s",
			socketMaxAddressLength, baseAddr)
	}
	// If possible, early-exit with a non-truncated socket path
	if numRemainingChars >= len(desiredName) {
		return filepath.Join(baseAddr, desiredName+socketSuffix), nil
	}
	// Hash the desiredName so that every invocation returns the same truncated address
	desiredNameHashCreator := sha256.New()
	_, err := desiredNameHashCreator.Write([]byte(desiredName))
	if err != nil {
		return "", fmt.Errorf("failed to calculate a hash for %q while creating a truncated socket address", desiredName)
	}
	desiredNameHash := base32.StdEncoding.EncodeToString(desiredNameHashCreator.Sum(nil))
	if len(desiredNameHash) < socketHashSuffixLength {
		// sha256.Sum() should return 32 bytes so this shouldn't occur, but good to check instead of panicing
		return "", fmt.Errorf("the encoded hash %q for %q is shorter than the minimum socket suffix length %v",
			desiredNameHash, desiredName, socketHashSuffixLength)
	}
	// Assemble the truncated socket address
	socketHashSuffix := desiredNameHash[:socketHashSuffixLength]
	truncatedName := desiredName[:(numRemainingChars - socketHashSuffixLength - 1)]
	return filepath.Join(baseAddr, fmt.Sprintf("%s-%s%s", truncatedName, socketHashSuffix, socketSuffix)), nil
}

// HandlerMap is the format for api->model pairs that the module will service.
// Ex: mymap["rdk:component:motor"] = ["acme:marine:thruster", "acme:marine:outboard"].
type HandlerMap map[resource.RPCAPI][]resource.Model

// ToProto converts the HandlerMap to a protobuf representation.
func (h HandlerMap) ToProto() *pb.HandlerMap {
	pMap := &pb.HandlerMap{}
	for s, models := range h {
		subtype := &robotpb.ResourceRPCSubtype{
			Subtype: protoutils.ResourceNameToProto(resource.Name{
				API:  s.API,
				Name: "",
			}),
			ProtoService: s.ProtoSvcName,
		}

		handler := &pb.HandlerDefinition{Subtype: subtype}
		for _, m := range models {
			handler.Models = append(handler.Models, m.String())
		}
		pMap.Handlers = append(pMap.Handlers, handler)
	}
	return pMap
}

// NewHandlerMapFromProto converts protobuf to HandlerMap.
func NewHandlerMapFromProto(ctx context.Context, pMap *pb.HandlerMap, conn rpc.ClientConn) (HandlerMap, error) {
	hMap := make(HandlerMap)
	refClient := grpcreflect.NewClientV1Alpha(ctx, reflectpb.NewServerReflectionClient(conn))
	defer refClient.Reset()
	reflSource := grpcurl.DescriptorSourceFromServer(ctx, refClient)

	var errs error
	for _, h := range pMap.GetHandlers() {
		api := protoutils.ResourceNameFromProto(h.Subtype.Subtype).API
		rpcAPI := &resource.RPCAPI{
			API: api,
		}
		// due to how tagger is setup in the api we cannot use reflection on the discovery service currently
		// for now we will skip the reflection step for discovery until the issue is resolved.
		// TODO(RSDK-9718) - remove the skip.
		if api != discovery.API {
			symDesc, err := reflSource.FindSymbol(h.Subtype.ProtoService)
			if err != nil {
				errs = multierr.Combine(errs, err)
				if errors.Is(err, grpcurl.ErrReflectionNotSupported) {
					return nil, errs
				}
				continue
			}
			svcDesc, ok := symDesc.(*desc.ServiceDescriptor)
			if !ok {
				return nil, fmt.Errorf("expected descriptor to be service descriptor but got %T", symDesc)
			}
			rpcAPI.Desc = svcDesc
		}
		for _, m := range h.Models {
			model, err := resource.NewModelFromString(m)
			if err != nil {
				return nil, err
			}
			hMap[*rpcAPI] = append(hMap[*rpcAPI], model)
		}
	}
	return hMap, errs
}
