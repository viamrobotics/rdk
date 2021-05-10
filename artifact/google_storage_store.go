package artifact

import (
	"context"
	"io"
	"os"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

func newGoogleStorageStore(config *googleStorageStoreConfig) (*googleStorageStore, error) {
	var opts []option.ClientOption
	if path, ok := os.LookupEnv("GOOGLE_APPLICATION_CREDENTIALS"); !ok || path == "" {
		opts = append(opts, option.WithoutAuthentication())
	}
	client, err := storage.NewClient(context.Background(), opts...)
	if err != nil {
		return nil, err
	}

	return &googleStorageStore{client: client, bucket: client.Bucket(config.Bucket)}, nil
}

type googleStorageStore struct {
	client *storage.Client
	bucket *storage.BucketHandle
}

func (s *googleStorageStore) Contains(hash string) error {
	_, err := s.bucket.Object(hash).Attrs(context.Background())
	if err != nil {
		if err == storage.ErrObjectNotExist {
			return NewErrArtifactNotFoundHash(hash)
		}
		return err
	}
	return nil
}

func (s *googleStorageStore) Load(hash string) (io.ReadCloser, error) {
	rc, err := s.bucket.Object(hash).NewReader(context.Background())
	if err != nil {
		if err == storage.ErrObjectNotExist {
			return nil, NewErrArtifactNotFoundHash(hash)
		}
		return nil, err
	}
	return rc, nil
}

func (s *googleStorageStore) Store(hash string, r io.Reader) error {
	if _, err := s.Load(hash); err == nil {
		return nil
	}
	wc := s.bucket.Object(hash).NewWriter(context.Background())
	if _, err := io.Copy(wc, r); err != nil {
		return err
	}
	return wc.Close()
}

func (s *googleStorageStore) Close() error {
	return s.client.Close()
}
