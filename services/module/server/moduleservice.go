package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/config"
	pb "go.viam.com/rdk/proto/api/module/v1"
	"go.viam.com/rdk/protoutils"
)

type server struct {
	pb.UnimplementedModuleServiceServer
}

func (s *server) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	log.Printf("Received: %v", req.GetResourceName())

	c := &config.Component{
		Name:  req.GetResourceName(),
		Model: "Foobar",
		Attributes: config.AttributeMap{
			"Speed": 30,
			"Port":  "localhost:3030",
		},
	}

	conf, err := protoutils.StructToStructPb(c)
	if err != nil {
		return nil, err
	}

	return &pb.RegisterResponse{Config: []*structpb.Struct{conf}}, nil
}

func main() {
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt)
	signal.Notify(shutdown, syscall.SIGTERM)

	oldMask := syscall.Umask(0o077)
	lis, err := net.Listen("unix", "/tmp/viam-server.sock")
	syscall.Umask(oldMask)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterModuleServiceServer(s, &server{})
	log.Printf("server listening at %v", lis.Addr())
	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()
	<-shutdown
	log.Println("Sutting down gracefully.")
	s.GracefulStop()
}
