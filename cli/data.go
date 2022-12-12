package cli

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/pkg/errors"
	datapb "go.viam.com/api/app/data/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	dataDir                    = "data"
	metadataDir                = "metadata"
	defaultConcurrentDownloads = 10
	maxRetryCount              = 5
	logEveryN                  = 100
)

// BinaryData downloads binary data matching filter to dst.
func (c *AppClient) BinaryData(dst string, filter *datapb.Filter, concurrentDownloads int) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	if err := makeDestinationDirs(dst); err != nil {
		return errors.Wrapf(err, "error creating destination directories")
	}

	if concurrentDownloads == 0 {
		concurrentDownloads = defaultConcurrentDownloads
	}

	ids := make(chan string, concurrentDownloads)
	// Give channel buffer of 1+concurrentDownloads because that is the number of goroutines that may be passing an
	// error into this channel (1 get ids routine + concurrentRequest download routines).
	errs := make(chan error, 1+concurrentDownloads)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wg := sync.WaitGroup{}

	// In one routine, get all IDs matching the filter and pass them into ids.
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := getMatchingBinaryIDs(ctx, c.dataClient, filter, ids, concurrentDownloads); err != nil {
			errs <- err
			cancel()
		}
	}()

	// In parallel, read from ids and download the binary for each id in batches of defaultConcurrentDownloads.
	wg.Add(1)
	go func() {
		defer wg.Done()
		var nextID string
		var done bool
		numFilesDownloaded := &atomic.Int32{}
		downloadWG := sync.WaitGroup{}
		for {
			for i := 0; i < concurrentDownloads; i++ {
				if err := ctx.Err(); err != nil {
					errs <- err
					cancel()
					done = true
					break
				}

				nextID = <-ids

				// If nextID is zero value, the channel has been closed and there are no more IDs to be read.
				if nextID == "" {
					done = true
					break
				}

				downloadWG.Add(1)
				go func(id string) {
					defer downloadWG.Done()
					err := downloadBinary(ctx, c.dataClient, dst, id)
					if err != nil {
						errs <- err
						cancel()
						done = true
					}
					numFilesDownloaded.Add(1)
				}(nextID)
			}
			downloadWG.Wait()
			if numFilesDownloaded.Load()%logEveryN == 0 {
				fmt.Fprintf(c.c.App.Writer, "downloaded %d files\n", numFilesDownloaded.Load())
			}
			if done {
				break
			}
		}
		if numFilesDownloaded.Load()%logEveryN != 0 {
			fmt.Fprintf(c.c.App.Writer, "downloaded %d files to %s\n", numFilesDownloaded.Load(), dst)
		}
	}()
	wg.Wait()
	close(errs)

	if err := <-errs; err != nil {
		return err
	}

	return nil
}

// getMatchingIDs queries client for all BinaryData matching filter, and passes each of their ids into ids.
func getMatchingBinaryIDs(ctx context.Context, client datapb.DataServiceClient, filter *datapb.Filter,
	ids chan string, concurrent int,
) error {
	var last string
	defer close(ids)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		resp, err := client.BinaryDataByFilter(ctx, &datapb.BinaryDataByFilterRequest{
			DataRequest: &datapb.DataRequest{
				Filter: filter,
				Limit:  uint64(concurrent),
				Last:   last,
			},
			CountOnly: false,
		})
		if err != nil {
			return err
		}
		// If no data is returned, there is no more data.
		if len(resp.GetData()) == 0 {
			return nil
		}
		last = resp.GetLast()

		for _, bd := range resp.GetData() {
			ids <- bd.GetMetadata().GetId()
		}
	}
}

func downloadBinary(ctx context.Context, client datapb.DataServiceClient, dst, id string) error {
	var resp *datapb.BinaryDataByIDsResponse
	var err error
	for count := 0; count < maxRetryCount; count++ {
		resp, err = client.BinaryDataByIDs(ctx, &datapb.BinaryDataByIDsRequest{
			FileIds:       []string{id},
			IncludeBinary: true,
		})
		if err == nil {
			break
		}
	}
	if err != nil {
		return errors.Wrapf(err, "received error from server")
	}
	data := resp.GetData()

	// We should always receive data.
	if len(data) == 0 {
		return errors.Errorf("received no binary data for id %s", id)
	}

	if len(data) != 1 {
		return errors.Errorf("expected a single response, received %d", len(data))
	}

	datum := data[0]
	mdJSONBytes, err := protojson.Marshal(datum.GetMetadata())
	if err != nil {
		return err
	}

	//nolint:gosec
	jsonFile, err := os.Create(filepath.Join(dst, "metadata", datum.GetMetadata().GetId()+".json"))
	if err != nil {
		return err
	}
	if _, err := jsonFile.Write(mdJSONBytes); err != nil {
		return err
	}

	gzippedBytes := datum.GetBinary()
	r, err := gzip.NewReader(bytes.NewBuffer(gzippedBytes))
	if err != nil {
		return err
	}

	//nolint:gosec
	dataFile, err := os.Create(filepath.Join(dst, "data", datum.GetMetadata().GetId()+datum.GetMetadata().GetFileExt()))
	if err != nil {
		return errors.Wrapf(err, fmt.Sprintf("error creating file for datum %s", datum.GetMetadata().GetId()))
	}
	//nolint:gosec
	if _, err := io.Copy(dataFile, r); err != nil {
		return err
	}
	if err := r.Close(); err != nil {
		return err
	}
	return nil
}

// TabularData downloads binary data matching filter to dst.
func (c *AppClient) TabularData(dst string, filter *datapb.Filter) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	if err := makeDestinationDirs(dst); err != nil {
		return errors.Wrapf(err, "error creating destination directories")
	}

	resp, err := c.dataClient.TabularDataByFilter(context.Background(), &datapb.TabularDataByFilterRequest{
		DataRequest: &datapb.DataRequest{
			Filter: filter,
			// TODO: For now don't worry about skip/limit. Just do everything in one request. Can implement batching when
			//       tabular is implemented.
		},
		CountOnly: false,
	})
	if err != nil {
		return err
	}
	mds := resp.GetMetadata()

	for i, md := range mds {
		mdJSONBytes, err := protojson.Marshal(md)
		if err != nil {
			return errors.Wrap(err, "error marshaling metadata")
		}
		//nolint:gosec
		mdFile, err := os.Create(filepath.Join(dst, "metadata", strconv.Itoa(i)+".json"))
		if err != nil {
			return errors.Wrapf(err, fmt.Sprintf("error creating metadata file for metadata index %d", i))
		}
		if _, err := mdFile.Write(mdJSONBytes); err != nil {
			return errors.Wrapf(err, "error writing metadata file %s", mdFile.Name())
		}
		if err := mdFile.Close(); err != nil {
			return errors.Wrapf(err, "error closing metadata file %s", mdFile.Name())
		}
	}

	data := resp.GetData()
	// TODO: [DATA-640] Support export in additional formats.
	//nolint:gosec
	dataFile, err := os.Create(filepath.Join(dst, "data", "data"+".ndjson"))
	if err != nil {
		return errors.Wrapf(err, "error creating data file")
	}
	w := bufio.NewWriter(dataFile)
	for _, datum := range data {
		// Write everything as json for now.
		d := datum.GetData()
		if d == nil {
			continue
		}
		m := d.AsMap()
		m["TimeRequested"] = datum.GetTimeRequested()
		m["TimeReceived"] = datum.GetTimeReceived()
		m["MetadataIndex"] = datum.GetMetadataIndex()
		j, err := json.Marshal(m)
		if err != nil {
			return errors.Wrap(err, "error marshaling json response")
		}
		_, err = w.Write(append(j, []byte("\n")...))
		if err != nil {
			return errors.Wrapf(err, "error writing reading to file %s", dataFile.Name())
		}
	}

	if err := w.Flush(); err != nil {
		return errors.Wrapf(err, "error flushing writer for %s", dataFile.Name())
	}

	return nil
}

func makeDestinationDirs(dst string) error {
	if err := os.MkdirAll(filepath.Join(dst, dataDir), 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dst, metadataDir), 0o700); err != nil {
		return err
	}
	return nil
}
