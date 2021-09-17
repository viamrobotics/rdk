// Package main contains a gRPC based metadata service client.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	rpcclient "go.viam.com/utils/rpc/client"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc/dialer"

	pb "go.viam.com/core/proto/api/service/v1"
)

var (
	serverAddr = "localhost:10000"
)

// MetadataServiceClient satisfies the robot.Robot interface through a gRPC based
// client conforming to the robot.proto contract.
type MetadataServiceClient struct {
	address string
	conn    dialer.ClientConn
	client  pb.MetadataServiceClient

	logger golog.Logger
}

func NewClient(ctx context.Context, address string, logger golog.Logger) (*MetadataServiceClient, error) {
	conn, err := rpcclient.Dial(ctx, address, rpcclient.DialOptions{Insecure: true}, logger)
	if err != nil {
		return nil, err
	}

	client := pb.NewMetadataServiceClient(conn)
	mc := &MetadataServiceClient{
		address: address,
		conn:    conn,
		client:  client,
		logger:  logger,
	}
	return mc, nil
}

// Close cleanly closes the underlying connections
func (mc *MetadataServiceClient) Close() error {
	return mc.conn.Close()
}

// Resources either gets a cached or latest version of the status of the remote
// robot.
func (mc *MetadataServiceClient) Resources(ctx context.Context) ([]*pb.ResourceName, error) {
	resp, err := mc.client.Resources(ctx, &pb.ResourcesRequest{})
	if err != nil {
		return nil, err
	}
	return resp.Resources, nil
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	logger := golog.NewDevelopmentLogger("metadata_service")
	client, err := NewClient(context.Background(), serverAddr, logger)

	// Looking for a valid feature
	fmt.Println("Sending message")
	resources, err := client.Resources(ctx)
	if err != nil {
		log.Fatalf("failed to dial: %v", err)
	}
	for {
		fmt.Printf("Response received %v\n", resources)
		time.Sleep(3 * time.Second)
	}

}
