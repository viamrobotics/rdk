// Package main contains a gRPC based metadata service client.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	pb "go.viam.com/core/proto/api/service/v1"
	"google.golang.org/grpc"
)

var (
	serverAddr = flag.String("server_addr", "localhost:10000", "The server address in the format of host:port")
)

// GetResources gets the list of resources from the server.
func GetResources(client pb.MetadataServiceClient) ([]*pb.ResourceName, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := client.Resources(ctx, &pb.ResourcesRequest{})
	if err != nil {
		return nil, err
	}

	return resp.Resources, nil
}

func main() {
	flag.Parse()
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())
	opts = append(opts, grpc.WithBlock())
	conn, err := grpc.Dial(*serverAddr, opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()
	client := pb.NewMetadataServiceClient(conn)

	// Looking for a valid feature
	fmt.Println("Sending message")
	resources, err := GetResources(client)
	if err != nil {
		log.Fatalf("failed to dial: %v", err)
	}
	for {
		fmt.Printf("Response received %v\n", resources)
		time.Sleep(3 * time.Second)
	}

}
