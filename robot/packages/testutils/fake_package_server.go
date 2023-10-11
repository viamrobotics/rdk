// Package testutils is test helpers for packages.
package testutils

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	pb "go.viam.com/api/app/packages/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
)

var errPackageMissng = errors.New("package missing")

// FakePackagesClientAndGCSServer to act as a stubbed client for the PackageServiceClient and host a fake server to serve HTTP get
// requests for the actual package tar.
type FakePackagesClientAndGCSServer struct {
	pb.UnimplementedPackageServiceServer

	packages map[string]*pb.Package

	exitWg       sync.WaitGroup
	httpserver   *http.Server
	rpcServer    rpc.Server
	httplistener net.Listener
	listener     net.Listener

	testPackagePath     string
	testPackageChecksum string

	invalidHTTPRes  bool
	invalidChecksum bool
	invalidTar      bool

	getRequestCount      int
	downloadRequestCount int

	mu     sync.Mutex
	logger logging.Logger
}

// NewFakePackageServer creates a new fake package server.
func NewFakePackageServer(ctx context.Context, logger logging.Logger) (*FakePackagesClientAndGCSServer, error) {
	httplistener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, err
	}

	rpclistener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, err
	}

	testPackagePath := artifact.MustPath("robot/packages/example.tar.gz")
	checksumForTestPackage, err := checksumFile(testPackagePath)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	httpServer := &http.Server{
		Addr:              httplistener.Addr().String(),
		Handler:           mux,
		ReadHeaderTimeout: time.Minute * 5,
	}

	server := &FakePackagesClientAndGCSServer{
		httpserver:          httpServer,
		httplistener:        httplistener,
		listener:            rpclistener,
		packages:            make(map[string]*pb.Package, 0),
		logger:              logger,
		testPackagePath:     testPackagePath,
		testPackageChecksum: checksumForTestPackage,
	}

	mux.Handle("/download-file", http.HandlerFunc(server.servePackage))

	server.exitWg.Add(1)
	utils.PanicCapturingGo(func() {
		defer server.exitWg.Done()

		err := server.httpserver.Serve(httplistener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Warnf("Error shutting down test server", "error", err)
		}
	})

	server.rpcServer, err = rpc.NewServer(logger,
		rpc.WithDisableMulticastDNS(),
		rpc.WithUnauthenticated(),
		rpc.WithWebRTCServerOptions(rpc.WebRTCServerOptions{Enable: false}))
	if err != nil {
		return nil, err
	}

	err = server.rpcServer.RegisterServiceServer(
		ctx,
		&pb.PackageService_ServiceDesc,
		server,
		pb.RegisterPackageServiceHandlerFromEndpoint,
	)
	if err != nil {
		return nil, err
	}

	server.exitWg.Add(1)
	utils.PanicCapturingGo(func() {
		defer server.exitWg.Done()

		err := server.rpcServer.Serve(rpclistener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Warnf("Error shutting down grpc server", "error", err)
		}
	})

	return server, nil
}

// Addr returns the listeners address.
func (c *FakePackagesClientAndGCSServer) Addr() net.Addr {
	return c.listener.Addr()
}

// Client returns a connect client to the server and connection.
func (c *FakePackagesClientAndGCSServer) Client(ctx context.Context) (pb.PackageServiceClient, rpc.ClientConn, error) {
	conn, err := rpc.DialDirectGRPC(ctx, c.listener.Addr().String(), c.logger, rpc.WithInsecure())
	if err != nil {
		return nil, nil, err
	}

	return pb.NewPackageServiceClient(conn), conn, nil
}

// SetInvalidChecksum sets failure state.
func (c *FakePackagesClientAndGCSServer) SetInvalidChecksum(flag bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.invalidChecksum = flag
}

// SetInvalidTar sets failure state.
func (c *FakePackagesClientAndGCSServer) SetInvalidTar(flag bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.invalidTar = flag
}

// SetInvalidHTTPRes sets failure state.
func (c *FakePackagesClientAndGCSServer) SetInvalidHTTPRes(flag bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.invalidHTTPRes = flag
}

// RequestCounts returns the request counters.
func (c *FakePackagesClientAndGCSServer) RequestCounts() (req, download int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.getRequestCount, c.downloadRequestCount
}

func (c *FakePackagesClientAndGCSServer) servePackage(w http.ResponseWriter, r *http.Request) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := r.URL.Query().Get("id")
	version := r.URL.Query().Get("version")

	c.downloadRequestCount++

	if c.invalidHTTPRes {
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte("some error"))
		if err != nil {
			c.logger.Error(err)
		}
		return
	}

	_, ok := c.packages[fmt.Sprintf("%s-%s", id, version)]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if c.invalidTar {
		w.Header().Set("Content-Type", "application/x-gzip")
		w.Header().Add("x-goog-hash", "crc32c=V9EBFQ==")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("not a tar"))
		if err != nil {
			c.logger.Error(err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/x-gzip")
	if !c.invalidChecksum {
		w.Header().Add("x-goog-hash", fmt.Sprintf("crc32c=%s", c.testPackageChecksum))
		w.Header().Add("x-goog-hash", "md5=Ojk9c3dhfxgoKVVHYwFbHQ==")
	} else {
		w.Header().Add("x-goog-hash", "crc32c=invalid==")
		w.Header().Add("x-goog-hash", "md5=invalid==")
	}

	f, err := os.Open(c.testPackagePath)
	if err != nil {
		c.logger.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, err = io.Copy(w, f)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// Shutdown will stop the server.
func (c *FakePackagesClientAndGCSServer) Shutdown() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	err := c.httpserver.Shutdown(ctx)
	if err != nil {
		return err
	}

	err = c.rpcServer.Stop()
	if err != nil {
		return err
	}

	c.exitWg.Wait()

	return nil
}

// Clear resets the fake servers state, does not restart the server.
func (c *FakePackagesClientAndGCSServer) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.packages = make(map[string]*pb.Package)
	c.invalidChecksum = false
	c.invalidTar = false
	c.invalidHTTPRes = false
	c.downloadRequestCount = 0
	c.getRequestCount = 0
}

// StorePackage store pacakges to known to fake server.
func (c *FakePackagesClientAndGCSServer) StorePackage(all ...config.PackageConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, p := range all {
		c.packages[fmt.Sprintf("%s-%s", p.Package, p.Version)] = &pb.Package{
			Id:        p.Package,
			CreatedOn: timestamppb.Now(),
			Info:      &pb.PackageInfo{Name: p.Package, Version: p.Version, OrganizationId: "org1", Type: pb.PackageType_PACKAGE_TYPE_ARCHIVE},
			Checksum:  "xyz",
		}
	}
}

// GetPackage returns the URL and metadata for a requested package version.
func (c *FakePackagesClientAndGCSServer) GetPackage(ctx context.Context, in *pb.GetPackageRequest) (*pb.GetPackageResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.getRequestCount++

	p, ok := c.packages[fmt.Sprintf("%s-%s", in.Id, in.Version)]
	if !ok {
		return nil, errPackageMissng
	}

	if in.IncludeUrl != nil && *in.IncludeUrl {
		p.Url = fmt.Sprintf("http://%s/download-file?id=%s&version=%s", c.httplistener.Addr().String(), p.Info.Name, p.Info.Version)
	}

	return &pb.GetPackageResponse{Package: p}, nil
}

// ValidateContentsOfPPackage validates the expected uncompressed / unzipped contents of the test package returned
// from the fake server.
//
// Contents:
// .
// ├── some-link.txt -> sub-dir/sub-file.txt
// ├── some-text.txt (crc32c:p_E54w==)
// ├── some-text2.txt (crc32c:p_E54w==)
// ├── sub-dir
// │   └── sub-file.txt (crc32c:p_E54w==)
// └── sub-dir-link -> sub-dir.
func ValidateContentsOfPPackage(t *testing.T, dir string) {
	t.Helper()

	type content struct {
		path       string
		checksum   string
		isLink     bool
		linkTarget string
		isDir      bool
		perms      os.FileMode
	}

	expected := []content{
		{path: "some-link.txt", isLink: true, linkTarget: "sub-dir/sub-file.txt", perms: 0o777},
		{path: "some-text.txt", checksum: "p/E54w==", perms: 0o644},
		{path: "some-text2.txt", checksum: "p/E54w==", perms: 0o644},
		{path: "sub-dir", isDir: true, perms: 0o755},
		{path: "sub-dir/sub-file.txt", checksum: "p/E54w==", perms: 0o644},
		{path: "sub-dir-link", isLink: true, linkTarget: "sub-dir", perms: 0o777},
	}

	out := make([]content, 0, len(expected))

	err := filepath.Walk(dir+"/", func(path string, info os.FileInfo, err error) error {
		test.That(t, err, test.ShouldBeNil)

		rel, err := filepath.Rel(dir, path)
		test.That(t, err, test.ShouldBeNil)

		if rel == "." {
			return nil
		}

		var symTarget string
		var checksum string
		isSymLink := info.Mode()&os.ModeSymlink != 0
		if isSymLink {
			symTarget, err = os.Readlink(path)
			test.That(t, err, test.ShouldBeNil)
		} else if info.Mode().IsRegular() {
			checksum, err = checksumFile(path)
			test.That(t, err, test.ShouldBeNil)
		}

		out = append(out, content{
			path:       rel,
			checksum:   checksum,
			isDir:      info.IsDir(),
			isLink:     isSymLink,
			linkTarget: symTarget,
			perms:      info.Mode().Perm(),
		})

		return nil
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out, test.ShouldResemble, expected)
}

func checksumFile(path string) (string, error) {
	hasher := crc32Hash()
	//nolint:gosec
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer utils.UncheckedErrorFunc(f.Close)

	_, err = io.Copy(hasher, f)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(hasher.Sum(nil)), nil
}

func crc32Hash() hash.Hash32 {
	return crc32.New(crc32.MakeTable(crc32.Castagnoli))
}
