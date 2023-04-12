package config

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	pb "go.viam.com/api/service/slam/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// This increases the message size from 4MB to 32MB to match the RDK.
// This is necessary for transmitting large pointclouds.
var grpcMaxMessageSize = 32 * 1024 * 1024

// SetupDirectories creates the data directory at the specified path along with
// its data, map, and config subdirectories.
func SetupDirectories(dataDirectory string, logger golog.Logger) error {
	for _, directoryName := range [4]string{"", "data", "map", "config"} {
		directoryPath := filepath.Join(dataDirectory, directoryName)
		if _, err := os.Stat(directoryPath); err != nil {
			// This error includes the directoryPath
			logger.Warnf("setup directories: %v", err)
			if err := os.Mkdir(directoryPath, os.ModePerm); err != nil {
				return errors.Errorf("issue creating directory at %v: %v", directoryPath, err)
			}
		}
	}
	return nil
}

// SetupGRPCConnection uses the defined port to create a GRPC client for communicating with the SLAM algorithms.
func SetupGRPCConnection(
	ctx context.Context,
	port string,
	dialMaxTimeoutSec int,
	logger golog.Logger,
) (pb.SLAMServiceClient, func() error, error) {
	ctx, span := trace.StartSpan(ctx, "slam::SetupGRPCConnection")
	defer span.End()
	ctx, timeoutCancel := context.WithTimeout(ctx, time.Duration(dialMaxTimeoutSec)*time.Second)
	defer timeoutCancel()
	maxMsgSizeOption := grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(grpcMaxMessageSize))
	// TODO: If we support running SLAM in the cloud, we need to pass credentials to this function
	connLib, err := grpc.DialContext(ctx, port, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock(), maxMsgSizeOption)
	if err != nil {
		logger.Errorw("error connecting to slam process", "error", err)
		return nil, nil, err
	}
	return pb.NewSLAMServiceClient(connLib), connLib.Close, err
}
