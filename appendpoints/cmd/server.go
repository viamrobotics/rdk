package main

import (
	"fmt"
	"net"
	"net/http"

	"go.viam.com/rdk/appendpoints"
)

type Server struct {
}

func (server *Server) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	appendpoints.IKHandler(resp, req)
}

func main() {
	server := &http.Server{
		Addr:    ":9090",
		Handler: &Server{},
	}

	nl, err := net.Listen("tcp", ":9090")
	if err != nil {
		panic(err)
	}

	fmt.Println("Listening on :9090")
	server.Serve(nl)
}
