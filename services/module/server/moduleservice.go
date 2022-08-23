package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	//"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/utils"

	//"go.viam.com/rdk/config"
	pb "go.viam.com/rdk/proto/api/module/v1"
	//"go.viam.com/rdk/protoutils"
	//"go.viam.com/rdk/resource"
)

type server struct {
	pb.UnimplementedModuleServiceServer
}


func (s * server) AddResource(ctx context.Context, req *pb.AddResourceRequest) (*pb.AddResourceResponse, error) {
	log.Printf("SMURF100: %+v", req)
	return &pb.AddResourceResponse{}, nil
}

// Arguments for the command.
type Arguments struct {
	Socket         string `flag:"0,required,usage=socket path"`
}

func main() {

	f, err := os.OpenFile("/tmp/mod.log", os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()

	log.SetOutput(f)


	var argsParsed Arguments
	if err := utils.ParseFlags(os.Args, &argsParsed); err != nil {
		log.Fatal(err)
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt)
	signal.Notify(shutdown, syscall.SIGTERM)

	oldMask := syscall.Umask(0o077)
	lis, err := net.Listen("unix", argsParsed.Socket)
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
