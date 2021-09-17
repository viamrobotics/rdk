// Package main contains a gRPC based metadata service server.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"

	pb "go.viam.com/core/proto/api/service/v1"
	"google.golang.org/grpc"
)

var (
	port = flag.Int("port", 10000, "The server port")
)

// Server implements the contract from metadata.proto
type MetadataServiceServer struct {
	pb.UnimplementedMetadataServiceServer
	m
	resources []*pb.ResourceName
}

// newServer constructs a gRPC service servert.
func newServer() *MetadataServiceServer {
	s := &MetadataServiceServer{}
	s.loadResources("")
	return s
}

// Resources returns the list of resources.
func (s *MetadataServiceServer) Resources(ctx context.Context, _ *pb.ResourcesRequest) (*pb.ResourcesResponse, error) {
	fmt.Printf("Sending :%v\n", s.resources)
	return &pb.ResourcesResponse{Resources: s.resources}, nil
}

// loadResources can load resources from a JSON file.
func (s *MetadataServiceServer) loadResources(filePath string) {
	var data []byte
	if filePath != "" {
		// TODO: parse config file, either through using robot or own parser
		var err error
		data, err = ioutil.ReadFile(filePath)
		if err != nil {
			log.Fatalf("Failed to load config file: %v", err)
		}
	} else {
		data = exampleData
	}
	if err := json.Unmarshal(data, &s.resources); err != nil {
		log.Fatalf("Failed to load resource list: %v", err)
	}
}

func main() {
	flag.Parse()
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterMetadataServiceServer(grpcServer, newServer())
	fmt.Printf("Server listening on port %d\n", *port)
	grpcServer.Serve(lis)
}

var exampleData = []byte(`[{
    "uuid": "ad9cffc4-3cb6-446d-a4f8-f2776e6ed0e1",
	"namespace": "core",
	"type": "service",
	"subtype": "metadata",
    "name": ""
}, {
    "uuid": "84032c6c-a5fa-4462-88ac-69deb0b55243",
	"namespace": "acme",
	"type": "component",
	"subtype": "arm",
    "name": "arm1"
}]`)
