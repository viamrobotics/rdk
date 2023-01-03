package builtin

import (
	"archive/zip"
	"bytes"
	"context"
	"github.com/edaniels/golog"
	m1 "go.viam.com/api/app/model/v1"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/services/datamanager/model"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TODO(DATA-341): Handle partial downloads in order to resume deployment.
// TODO(DATA-344): Compare checksum of downloaded model to blob to determine whether to redeploy.
// TODO(DATA-493): Test model deployment from config file.
// TODO(DATA-510): Make TestModelDeploy concurrency safe
// Validates that models can be deployed onto a robot.
func TestModelDeploy(t *testing.T) {
	t.Skip()
	deployModelWaitTime := time.Millisecond * 1000
	deployedZipFileName := "model.zip"
	originalFileName := "model.txt"
	otherOriginalFileName := "README.md"
	b0 := []byte("text representing model.txt internals.")
	b1 := []byte("text representing README.md internals.")

	// Create zip file.
	deployedZipFile, err := os.Create(deployedZipFileName)
	test.That(t, err, test.ShouldBeNil)
	zipWriter := zip.NewWriter(deployedZipFile)

	defer os.Remove(deployedZipFileName)
	defer deployedZipFile.Close()

	// Write zip file contents
	zipFile1, err := zipWriter.Create(originalFileName)
	test.That(t, err, test.ShouldBeNil)
	_, err = zipFile1.Write(b0)
	test.That(t, err, test.ShouldBeNil)

	zipFile2, err := zipWriter.Create(otherOriginalFileName)
	test.That(t, err, test.ShouldBeNil)
	_, err = zipFile2.Write(b1)
	test.That(t, err, test.ShouldBeNil)

	// Close zipWriter so we can unzip later
	zipWriter.Close()

	// Register mock model service with a mock server.
	modelServer, _ := buildAndStartLocalModelServer(t, deployedZipFileName)
	defer func() {
		err := modelServer.Stop()
		test.That(t, err, test.ShouldBeNil)
	}()

	// Generate models.
	var allModels []*model.Model
	m1 := &model.Model{Name: "m1", Destination: filepath.Join(os.Getenv("HOME"), "custom")} // with custom location
	m2 := &model.Model{Name: "m2", Destination: ""}                                         // with default location
	allModels = append(allModels, m1, m2)

	defer func() {
		for i := range allModels {
			os.RemoveAll(allModels[i].Destination)
		}
	}()

	testCfg := setupConfig(t, configPath)
	dmCfg, err := getDataManagerConfig(testCfg)
	test.That(t, err, test.ShouldBeNil)

	// Set SyncIntervalMins equal to zero so we do not enable syncing.
	dmCfg.SyncIntervalMins = 0
	dmCfg.ModelsToDeploy = allModels

	// Initialize the data manager and update it with our config.
	dmsvc := newTestDataManager(t)
	defer dmsvc.Close(context.Background())
	dmsvc.SetModelManagerConstructor(getTestModelManagerConstructor(t, modelServer, deployedZipFileName))

	err = dmsvc.Update(context.Background(), testCfg)
	test.That(t, err, test.ShouldBeNil)

	time.Sleep(deployModelWaitTime)

	// Close the data manager.
	_ = dmsvc.Close(context.Background())

	// Validate that the models were deployed.
	files, err := ioutil.ReadDir(allModels[0].Destination)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(files), test.ShouldEqual, 2)

	files, err = ioutil.ReadDir(allModels[1].Destination)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(files), test.ShouldEqual, 2)

	// Validate that the deployed model files equal the dummy files that were zipped.
	similar, err := fileCompareTestHelper(filepath.Join(allModels[0].Destination, originalFileName), b0)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, similar, test.ShouldBeTrue)

	similar, err = fileCompareTestHelper(filepath.Join(allModels[0].Destination, otherOriginalFileName), b1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, similar, test.ShouldBeTrue)

	similar, err = fileCompareTestHelper(filepath.Join(allModels[1].Destination, originalFileName), b0)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, similar, test.ShouldBeTrue)

	similar, err = fileCompareTestHelper(filepath.Join(allModels[1].Destination, otherOriginalFileName), b1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, similar, test.ShouldBeTrue)
}

func fileCompareTestHelper(path string, info []byte) (bool, error) {
	deployedUnzippedFile, err := ioutil.ReadFile(path)
	if err != nil {
		return false, err
	}
	return bytes.Equal(deployedUnzippedFile, info), nil
}

// TODO(DATA-487): Support zipping multiple files in
// buildAndStartLocalModelServer and getTestModelManagerConstructor
type mockModelServiceServer struct {
	zipFileName string
	lock        *sync.Mutex
	m1.UnimplementedModelServiceServer
}

type mockClient struct {
	zipFileName string
}

func (m *mockClient) Do(req *http.Request) (*http.Response, error) {
	buf, err := os.ReadFile(m.zipFileName)
	if err != nil {
		return nil, err
	}

	// convert bytes into bytes.Reader
	reader := bytes.NewReader(buf)

	response := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(reader),
	}

	return response, nil
}

func (m mockModelServiceServer) Deploy(ctx context.Context, req *m1.DeployRequest) (*m1.DeployResponse, error) {
	(*m.lock).Lock()
	defer (*m.lock).Unlock()
	depResp := &m1.DeployResponse{Message: m.zipFileName}
	return depResp, nil
}

//nolint:thelper
func buildAndStartLocalModelServer(t *testing.T, deployedZipFileName string) (rpc.Server, mockModelServiceServer) {
	logger, _ := golog.NewObservedTestLogger(t)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)
	mockService := mockModelServiceServer{
		zipFileName:                     deployedZipFileName,
		lock:                            &sync.Mutex{},
		UnimplementedModelServiceServer: m1.UnimplementedModelServiceServer{},
	}
	err = rpcServer.RegisterServiceServer(
		context.Background(),
		&m1.ModelService_ServiceDesc,
		mockService,
		m1.RegisterModelServiceHandlerFromEndpoint,
	)
	test.That(t, err, test.ShouldBeNil)

	// Stand up the server. Defer stopping the server.
	go func() {
		err := rpcServer.Start()
		test.That(t, err, test.ShouldBeNil)
	}()
	return rpcServer, mockService
}

//nolint:thelper
func getTestModelManagerConstructor(t *testing.T, server rpc.Server, zipFileName string) model.ManagerConstructor {
	return func(logger golog.Logger, cfg *config.Config) (model.Manager, error) {
		conn, err := getLocalServerConn(server, logger)
		test.That(t, err, test.ShouldBeNil)
		client := model.NewClient(conn)
		return model.NewManager(logger, cfg.Cloud.ID, client, conn, &mockClient{zipFileName: zipFileName})
	}
}
