package model

import (
	"archive/zip"
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
	v1 "go.viam.com/api/app/model/v1"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	rdkutils "go.viam.com/rdk/utils"
)

const appAddress = "app.viam.com:443"

const zipExtension = ".zip"

// ViamModelDotDir is the default model directory a model will deployed into,
// unless otherwise specified.
var ViamModelDotDir = filepath.Join(os.Getenv("HOME"), "models", ".viam")

// Model describes a model we want to download to the robot.
type Model struct {
	Name        string `json:"source_model_name"`
	Destination string `json:"destination"`
}

// HTTPClient allows us to mock a connection.
type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Manager is responsible for deploying model files.
type Manager interface {
	DownloadModels(cfg *config.Config, modelsToDeploy []*Model) error
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
	httpClient        httpClient
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
	return NewManager(logger, cfg.Cloud.ID, client, conn, &http.Client{})
}

// NewManager returns a new modelr.
func NewManager(logger golog.Logger, partID string, client v1.ModelServiceClient,
	conn rpc.ClientConn, httpClient httpClient,
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
		httpClient:        httpClient,
	}
	return &ret, nil
}

func (m *modelManager) deploy(ctx context.Context, req *v1.DeployRequest) (*v1.DeployResponse, error) {
	resp, err := m.client.Deploy(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// Close closes all resources (goroutines) associated with modelManger.
func (m *modelManager) Close() {
	m.cancelFunc()
	m.backgroundWorkers.Wait()
	if m.conn != nil {
		if err := m.conn.Close(); err != nil {
			m.logger.Errorw("error closing model deploy server connection", "error", err)
		}
	}
}

func (m *modelManager) DownloadModels(cfg *config.Config, modelsToDeploy []*Model) error {
	modelsToDownload, err := getModelsToDownload(modelsToDeploy)
	if err != nil {
		return err
	}
	// TODO: DATA-295, delete models in file system that are no longer in the config. If we have no models to download, exit.
	if len(modelsToDownload) == 0 {
		return nil
	}
	// Stop download of previous models if we're trying to download new ones.
	if m.cancelFunc != nil {
		m.cancelFunc()
		m.backgroundWorkers.Wait()
	}

	cancelCtx, cancelFn := context.WithCancel(context.Background())
	m.cancelFunc = cancelFn
	// out := make(chan error)
	checkMe := make(chan error, len(modelsToDownload))
	m.backgroundWorkers.Add(len(modelsToDownload))
	for _, model := range modelsToDownload {
		// defer close(checkMe) // hmm is this correct?
		inner := func(model *Model) error {
			defer m.backgroundWorkers.Done()
			deployRequest := &v1.DeployRequest{
				Metadata: &v1.DeployMetadata{
					ModelName: model.Name,
				},
			}
			deployResp, err := m.deploy(cancelCtx, deployRequest)
			if err != nil {
				m.logger.Error(err)
				// out <- err
				return err
			}
			url := deployResp.Message
			filePath := filepath.Join(model.Destination, model.Name)
			err = downloadFile(cancelCtx, m.httpClient, filePath, url, m.logger)
			if err != nil {
				m.logger.Error(err)
				// out <- err
				return err
			}
			// A download from a GCS signed URL only returns one file.
			modelFileToUnzip := model.Name + zipExtension
			if err = unzipSource(modelFileToUnzip, model.Destination); err != nil {
				m.logger.Error(err)
				// out <- err
				return err
			}

			return nil
		}
		checkMe <- inner(model)
	}
	m.backgroundWorkers.Wait()
	close(checkMe)
	// err = <-checkMe

	return <-checkMe
}

// GetModelsToDownload fetches the models that need to be downloaded according to the
// provided config.
func getModelsToDownload(models []*Model) ([]*Model, error) {
	// TODO: DATA-405, if the user specifies one destination to deploy their models into
	// this function will not work as expected.
	modelsToDownload := make([]*Model, 0)
	for _, model := range models {
		if model.Destination == "" {
			// Set the model destination to default if it's not specified in the config.
			model.Destination = filepath.Join(ViamModelDotDir, model.Name)
		}
		_, err := os.Stat(model.Destination)
		switch {
		case errors.Is(err, os.ErrNotExist):
			// we know we have never attempted to deploy the model
			modelsToDownload = append(modelsToDownload, model)
			// create model.Destination directory
			err := os.MkdirAll(model.Destination, os.ModePerm)
			if err != nil {
				return nil, err
			}
		case err != nil:
			return nil, err
		default:
			files, err := ioutil.ReadDir(model.Destination)
			if err != nil {
				return nil, err
			}
			if len(files) == 0 {
				// know there was a partial download
				modelsToDownload = append(modelsToDownload, model)
			}
		}
	}
	return modelsToDownload, nil
}

// DownloadFile will download a url to a local file. It writes as it
// downloads and doesn't load the whole file into memory.
func downloadFile(cancelCtx context.Context, client httpClient, filepath, url string, logger golog.Logger) error {
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

	s := filepath + zipExtension
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

// UnzipSource unzips all files inside a zip file.
func unzipSource(fileName, destination string) error {
	zipReader, err := zip.OpenReader(filepath.Join(destination, fileName))
	if err != nil {
		return err
	}
	for _, f := range zipReader.File {
		if err := unzipFile(f, destination); err != nil {
			return err
		}
	}
	if err = zipReader.Close(); err != nil {
		return err
	}

	if err = os.Remove(filepath.Join(destination, fileName)); err != nil {
		return err
	}
	return nil
}

func unzipFile(f *zip.File, destination string) error {
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
		return err
	}

	return nil
}
