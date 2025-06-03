package artifact

import (
	"context"
	"io"
	"net/http"
	"os"
	"sync"

	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	gcphttp "google.golang.org/api/transport/http"
)

func init() {
	if path, ok := os.LookupEnv("ARTIFACT_GOOGLE_APPLICATION_CREDENTIALS"); ok && path != "" {
		setGoogleCredsPath(path)
	}
}

var (
	_googleCredsMu   sync.Mutex
	_googleCredsPath string
)

func getGoogleCredsPath() string {
	_googleCredsMu.Lock()
	defer _googleCredsMu.Unlock()
	return _googleCredsPath
}

func setGoogleCredsPath(path string) func() {
	_googleCredsMu.Lock()
	prevGoogleCredsPath := _googleCredsPath
	_googleCredsPath = path
	_googleCredsMu.Unlock()
	return func() {
		setGoogleCredsPath(prevGoogleCredsPath)
	}
}

// newGoogleStorageStore returns a new googleStorageStore based on the given config.
func newGoogleStorageStore(config *GoogleStorageStoreConfig) (*googleStorageStore, error) {
	if config.Bucket == "" {
		return nil, errors.New("bucket required")
	}

	var opts []option.ClientOption
	credsPath := getGoogleCredsPath()
	if credsPath == "" {
		opts = append(opts, option.WithoutAuthentication())
	} else {
		opts = append(opts, option.WithCredentialsFile(credsPath), option.WithScopes(storage.ScopeFullControl))
	}
	var httpTransport http.Transport
	var err error
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{Transport: &httpTransport})
	gcpTransport, err := gcphttp.NewTransport(ctx, &httpTransport, opts...)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{Transport: gcpTransport}
	opts = append(opts, option.WithHTTPClient(httpClient))

	client, err := storage.NewClient(context.Background(), opts...)
	if err != nil {
		httpTransport.CloseIdleConnections()
		return nil, errors.WithStack(err)
	}

	return &googleStorageStore{
		client:        client,
		bucket:        client.Bucket(config.Bucket),
		httpTransport: &httpTransport,
	}, nil
}

// A googleStorageStore is able to load and store artifacts by their hashes and content.
type googleStorageStore struct {
	client        *storage.Client
	bucket        *storage.BucketHandle
	httpTransport *http.Transport
}

func (s *googleStorageStore) Contains(hash string) error {
	_, err := s.bucket.Object(hash).Attrs(context.Background())
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return NewArtifactNotFoundHashError(hash)
		}
		return err
	}
	return nil
}

func (s *googleStorageStore) Load(hash string) (io.ReadCloser, error) {
	rc, err := s.bucket.Object(hash).NewReader(context.Background())
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return nil, NewArtifactNotFoundHashError(hash)
		}
		return nil, err
	}
	return rc, nil
}

func (s *googleStorageStore) Store(hash string, r io.Reader) (err error) {
	if rc, err := s.Load(hash); err == nil {
		return rc.Close()
	}
	wc := s.bucket.Object(hash).NewWriter(context.Background())
	defer func() {
		err = multierr.Combine(err, wc.Close())
	}()
	if _, err := io.Copy(wc, r); err != nil {
		return err
	}
	return nil
}

func (s *googleStorageStore) Close() error {
	defer s.httpTransport.CloseIdleConnections()
	return s.client.Close()
}
