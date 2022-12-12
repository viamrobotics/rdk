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

	"github.com/pkg/errors"
	datapb "go.viam.com/api/app/data/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	dataDir                   = "data"
	metadataDir               = "metadata"
	defaultConcurrentRequests = 10
)

// BinaryData downloads binary data matching filter to dst.
func (c *AppClient) BinaryData(dst string, filter *datapb.Filter, concurrentRequests int) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	if err := makeDestinationDirs(dst); err != nil {
		return errors.Wrapf(err, "error creating destination directories")
	}

	if concurrentRequests == 0 {
		concurrentRequests = defaultConcurrentRequests
	}

	ids := make(chan string, 10)
	getIDsErrs := make(chan error)
	downloadErrs := make(chan error, concurrentRequests)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wg := sync.WaitGroup{}

	// In one routine, get all IDs matching the filter and pass them into ids.
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := getMatchingBinaryIDs(ctx, c.dataClient, filter, ids); err != nil {
			cancel()
			getIDsErrs <- err
		}
		close(getIDsErrs)
	}()

	// In parallel, read from ids and download the binary for each id in batches of defaultConcurrentRequests.
	wg.Add(1)
	go func() {
		defer wg.Done()
		var nextID string
		var done bool
		downloadWG := sync.WaitGroup{}
		for {
			for i := 0; i < concurrentRequests; i++ {
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
						downloadErrs <- err
						cancel()
						done = true
					}
				}(nextID)
			}
			downloadWG.Wait()
			if done {
				break
			}
		}
		close(downloadErrs)
	}()
	wg.Wait()

	// TODO: how do we filter out all the context cancelled errors? We don't really care about those generally -
	//       their a side effect of some first "real" error causing the rest to be cancelled.

	// TODO: I feel like we should ensure we read some "real" error first. I think if we receive an error in download
	//       we would first read the context cancellation error from getIDs first here and return that, when it's not
	//       our root cause. Should we filter out all context.Cancelled errors though? Then we will miss out on "real"
	//       context cancellation errors.
	if err := <-getIDsErrs; err != nil {
		return err
	}
	if err := <-downloadErrs; err != nil {
		return err
	}

	return nil
}

// getMatchingIDs queries client for all BinaryData matching filter, and passes each of their ids into ids.
func getMatchingBinaryIDs(ctx context.Context, client datapb.DataServiceClient, filter *datapb.Filter,
	ids chan string) error {

	var last string
	for {
		resp, err := client.BinaryDataByFilter(ctx, &datapb.BinaryDataByFilterRequest{
			DataRequest: &datapb.DataRequest{
				Filter: filter,
				Limit:  50,
				Last:   last,
			},
			CountOnly: false,
		})
		if err != nil {
			close(ids)
			return err
		}
		// If no data is returned, there is no more data.
		if len(resp.GetData()) == 0 {
			close(ids)
			return nil
		}
		last = resp.GetLast()

		for _, bd := range resp.GetData() {
			ids <- bd.GetMetadata().GetId()
		}
	}
}

func downloadBinary(ctx context.Context, client datapb.DataServiceClient, dst string, id string) error {
	resp, err := client.BinaryDataByIDs(ctx, &datapb.BinaryDataByIDsRequest{
		FileIds:       []string{id},
		IncludeBinary: true,
	})
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
