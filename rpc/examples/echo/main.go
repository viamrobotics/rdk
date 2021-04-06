package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	pb "go.viam.com/robotcore/proto/rpc/examples/echo/v1"
	"go.viam.com/robotcore/rlog"
	"go.viam.com/robotcore/rpc"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
	"goji.io"
	"goji.io/pat"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

var (
	defaultPort = 8080
	logger      = rlog.Logger.Named("server")
)

// Arguments for the command.
type Arguments struct {
	Port utils.NetPortFlag `flag:"0"`
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	var argsParsed Arguments
	if err := utils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}
	if argsParsed.Port == 0 {
		argsParsed.Port = utils.NetPortFlag(defaultPort)
	}

	return runServer(ctx, int(argsParsed.Port), logger)
}

func runServer(ctx context.Context, port int, logger golog.Logger) (err error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return err
	}

	rpcServer, err := rpc.NewServer()
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(err, rpcServer.Stop())
	}()

	if err := rpcServer.RegisterServiceServer(
		ctx,
		&pb.EchoService_ServiceDesc,
		&echoServer{},
		pb.RegisterEchoServiceHandlerFromEndpoint,
	); err != nil {
		return err
	}

	mux := goji.NewMux()
	mux.Handle(pat.Get("/"), http.FileServer(http.Dir(utils.ResolveFile("rpc/examples/echo/frontend/static"))))
	mux.Handle(pat.Get("/static/*"), http.StripPrefix("/static", http.FileServer(http.Dir(utils.ResolveFile("rpc/examples/echo/frontend/dist")))))
	mux.Handle(pat.New("/api/*"), http.StripPrefix("/api", rpcServer.GatewayHandler()))
	mux.Handle(pat.New("/*"), rpcServer.GRPCHandler())

	h2s := &http2.Server{}
	httpServer := &http.Server{
		Addr:           listener.Addr().String(),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
		Handler:        h2c.NewHandler(mux, h2s),
	}
	go func() {
		<-ctx.Done()
		defer func() {
			if err := rpcServer.Stop(); err != nil {
				panic(err)
			}
		}()
		if err := httpServer.Shutdown(context.Background()); err != nil {
			panic(err)
		}
	}()
	go func() {
		if err := rpcServer.Start(); err != nil {
			panic(err)
		}
	}()
	utils.ContextMainReadyFunc(ctx)()

	logger.Infow("serving", "url", fmt.Sprintf("http://%s", listener.Addr().String()))
	if err := httpServer.Serve(listener); err != http.ErrServerClosed {
		return err
	}
	return nil
}

type echoServer struct {
	pb.UnimplementedEchoServiceServer
}

func (es *echoServer) Echo(ctx context.Context, req *pb.EchoRequest) (*pb.EchoResponse, error) {
	return &pb.EchoResponse{Message: req.Message}, nil
}
