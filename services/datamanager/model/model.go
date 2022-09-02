package model

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

var (
	viamModelDotDir = filepath.Join(os.Getenv("HOME"), "models", ".viam")
	Client          HTTPClient // Client is implementation of HTTPClient interface.
)

// HTTPClient allows us to mock a connection.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// MockClient is the mock client.
type MockClient struct{}

// Manager is responsible for deploying model files.
type Manager interface {
	Deploy(ctx context.Context, req *v1.DeployRequest) (*v1.DeployResponse, error)
	Close()
}

// modelManager is responsible for uploading files in captureDir to the cloud.
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
type ManagerConstructor func(logger golog.Logger, cfg *config.Config) (Manager, error)

// NewDefaultManager returns the default Manager that syncs data to app.viam.com.
func NewDefaultManager(logger golog.Logger, cfg *config.Config) (Manager, error) {
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
) (Manager, error) {
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

// GetModelsToDownload fetches the models that need to be downloaded according to the
// provided config.
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
		files, err := ioutil.ReadDir(model.Destination)
		if err != nil {
			return nil, err
		}
		if len(files) == 0 {
			return nil, nil
		}
		modelsToDownload = append(modelsToDownload, model)
	}
	return modelsToDownload, nil
}

// DownloadFile will download a url to a local file. It writes as it
// downloads and doesn't load the whole file into memory.
func DownloadFile(cancelCtx context.Context, client HTTPClient, filepath, url string, logger golog.Logger) error {
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

	// QUESTION: should I extract out to be a constant as well?
	s := filepath + ".gz"
	// //nolint:gosec
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

// UnzipSource unzips all files inside a zip file.
func UnzipSource(destination, fileName string, logger golog.Logger) error {
	zipReader, err := zip.OpenReader(filepath.Join(destination, fileName))
	if err != nil {
		return err
	}
	for _, f := range zipReader.File {
		if err := unzipFile(f, destination, logger); err != nil {
			return err
		}
	}
	if err = zipReader.Close(); err != nil {
		return err
	}
	return nil
}

func unzipFile(f *zip.File, destination string, logger golog.Logger) error {
	// TODO: DATA-307, We should be passing in the context to any operations that can take several seconds,
	// which includes unzipFile. As written, this can block .Close for an unbounded amount of time.
	//nolint:gosec
	filePath := filepath.Join(destination, f.Name)
	// Ensure file paths aren't vulnerable to zip slip
	if !strings.HasPrefix(filePath, filepath.Clean(destination)+string(os.PathSeparator)) {
		return fmt.Errorf("invalid file path: %s", filePath)
	}

	if f.FileInfo().IsDir() {
		if err := os.MkdirAll(filePath, os.ModePerm); err != nil {
			return err
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
		return err
	}

	// Create a destination file for unzipped content. We clean the
	// file above, so gosec doesn't need to complain.
	//nolint:gosec
	destinationFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		// os.Remove returns a path error which likely means that we don't have write permissions
		// or the file we're removing doesn't exist. In either case,
		// the file was never written to so don't need to remove it.
		//nolint:errcheck,gosec
		os.Remove(destinationFile.Name())
		return err
	}

	//nolint:gosec,errcheck
	defer destinationFile.Close()

	// Unzip the content of a file and copy it to the destination file
	zippedFile, err := f.Open()
	if err != nil {
		return err
	}
	//nolint:errcheck
	defer zippedFile.Close()

	// Gosec is worried about a decompression bomb; we restrict the size of the
	// files we upload to our data store, so should be OK.
	//nolint:gosec
	if _, err := io.Copy(destinationFile, zippedFile); err != nil {
		// See above comment regarding os.Remove.
		//nolint:errcheck
		os.Remove(destinationFile.Name())
		return err
	}

	// Double up trying to close zippedFile/destinationFile (defer above and explicitly below)
	// to ensure the buffer is flushed and closed.
	if err = zippedFile.Close(); err != nil {
		return err
	}

	if err = destinationFile.Close(); err != nil {
		logger.Error(err)
	}

	return nil
}

// Do is mockClient's `Do` func.
func (m *MockClient) Do(req *http.Request) (*http.Response, error) {
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
	err = zipWriter.Close()
	if err != nil {
		panic(err)
	}

	response := &http.Response{
		StatusCode: http.StatusOK, // Can I get rid of this?
		Body:       io.NopCloser(buf),
	}
	return response, nil
}
