package model

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/edaniels/golog"
	v1 "go.viam.com/api/proto/viam/model/v1"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	rdkutils "go.viam.com/rdk/utils"
)

const appAddress = "app.viam.com:443"

// Model describes a model we want to download to the robot.
type Model struct {
	Name        string `json:"source_model_name"`
	Destination string `json:"destination"`
}

var viamModelDotDir = filepath.Join(os.Getenv("HOME"), "models", ".viam")

// Manager is responsible for deploying model files.
type ModelManager interface {
	Deploy(ctx context.Context, req *v1.DeployRequest) (*v1.DeployResponse, error)
	Close()
	DownloadFile(cancelCtx context.Context, client HTTPClient, filepath, url string, logger golog.Logger) error
}

// HTTPClient allows us to mock a connection.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client is implementation of HTTPClient interface.
var Client HTTPClient

// type Client struct {
// 	client HTTPClient
// }

// modelr is responsible for uploading files in captureDir to the cloud.
type modelManager struct {
	partID            string
	conn              rpc.ClientConn
	client            v1.ModelServiceClient
	logger            golog.Logger
	backgroundWorkers sync.WaitGroup
	cancelCtx         context.Context
	cancelFunc        func()
}

// ManagerConstructor is a function for building a Manager.
type ManagerConstructor func(logger golog.Logger, cfg *config.Config) (ModelManager, error)

// NewDefaultManager returns the default Manager that syncs data to app.viam.com.
func NewDefaultManager(logger golog.Logger, cfg *config.Config) (ModelManager, error) {
	tlsConfig := config.NewTLSConfig(cfg).Config
	cloudConfig := cfg.Cloud
	rpcOpts := []rpc.DialOption{
		rpc.WithTLSConfig(tlsConfig),
		rpc.WithEntityCredentials(
			cloudConfig.ID,
			rpc.Credentials{
				Type:    rdkutils.CredentialsTypeRobotSecret,
				Payload: cloudConfig.Secret,
			}),
	}

	conn, err := NewConnection(logger, appAddress, rpcOpts)
	if err != nil {
		return nil, err
	}
	client := NewClient(conn)
	return NewManager(logger, cfg.Cloud.ID, client, conn)
}

// NewManager returns a new modelr.
func NewManager(logger golog.Logger, partID string, client v1.ModelServiceClient,
	conn rpc.ClientConn,
) (ModelManager, error) {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	ret := modelManager{
		conn:              conn,
		client:            client,
		logger:            logger,
		backgroundWorkers: sync.WaitGroup{},
		cancelCtx:         cancelCtx,
		cancelFunc:        cancelFunc,
		partID:            partID,
	}
	return &ret, nil
}

func (m *modelManager) Deploy(ctx context.Context, req *v1.DeployRequest) (*v1.DeployResponse, error) {
	resp, err := m.client.Deploy(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// Close closes all resources (goroutines) associated with s.
func (m *modelManager) Close() {
	m.cancelFunc()
	m.backgroundWorkers.Wait()
	if m.conn != nil {
		if err := m.conn.Close(); err != nil {
			m.logger.Errorw("error closing model deploy server connection", "error", err)
		}
	}
}

func GetModelsToDownload(models []*Model) ([]*Model, error) {
	// Right now, this may not act as expected. It currently checks
	// if the model folder is empty. If it is, then we proceed to download the model.
	// I can imagine a scenario where the user specifies a local folder to dump
	// all their models in. In that case, this wouldn't work as expected.
	// TODO: Fix.
	modelsToDownload := make([]*Model, 0)
	for _, model := range models {
		if model.Destination == "" {
			// Set the model destination to default if it's not specified in the config.
			model.Destination = filepath.Join(viamModelDotDir, model.Name)
		}
		_, err := os.Stat(model.Destination)
		if errors.Is(err, os.ErrNotExist) {
			modelsToDownload = append(modelsToDownload, model)
			// create model.Destination directory
			err := os.MkdirAll(model.Destination, os.ModePerm)
			if err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		}
	}
	return modelsToDownload, nil
}

// downloadFile will download a url to a local file. It writes as it
// downloads and doesn't load the whole file into memory.
func (m *modelManager) DownloadFile(cancelCtx context.Context, client HTTPClient, filepath, url string, logger golog.Logger) error {
	getReq, err := http.NewRequestWithContext(cancelCtx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(getReq)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Error(err)
		}
	}()

	s := filepath + ".zip"
	//nolint:gosec
	out, err := os.Create(s)
	if err != nil {
		return err
	}
	//nolint:gosec,errcheck
	defer out.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(out.Name(), bodyBytes, os.ModePerm)
}

// Do is mockClient's `Do` func.
func (m *MockClient) Do(req *http.Request) (*http.Response, error) {
	// why we are still using ioutil.NopCloser
	// https://stackoverflow.com/questions/28158990/golang-io-ioutil-nopcloser
	// using io.NopCloser and not ioutil.NopCloser bc the latter is depreciated

	r := bytes.NewReader([]byte("mocked response readme"))
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)
	w1, err := zipWriter.Create("README.md")
	if err != nil {
		panic(err)
	}
	if _, err := io.Copy(w1, r); err != nil {
		panic(err)
	}
	zipWriter.Close()

	response := &http.Response{
		StatusCode: http.StatusOK, // might not even need this
		Body:       io.NopCloser(buf),
	}
	return response, nil
}

// mockClient is the mock client.
type MockClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

// GetDoFunc fetches the mockClient's `Do` func.
var GetDoFunc func(req *http.Request) (*http.Response, error)
