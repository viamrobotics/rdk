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
	dataDir     = "data"
	metadataDir = "metadata"
	// TODO: possibly make this param with default value.
	numConcurrentRequests = 10
)

// BinaryData downloads binary data matching filter to dst.
func (c *AppClient) BinaryData(dst string, filter *datapb.Filter) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	if err := makeDestinationDirs(dst); err != nil {
		return errors.Wrapf(err, "error creating destination directories")
	}

	var last string
	idsChannel := make(chan string, 10)

	// In one loop: Send series of IncludeBinary=false requests with Limit 50. Through IDs into channel
	for {
		resp, err := c.dataClient.BinaryDataByFilter(context.Background(), &datapb.BinaryDataByFilterRequest{
			DataRequest: &datapb.DataRequest{
				Filter: filter,
				Limit:  50,
				Last:   last,
			},
			CountOnly: false,
		})
		if err != nil {
			return errors.Wrapf(err, "received error from server")
		}
		// If no data is returned, there is no more data.
		if len(resp.GetData()) == 0 {
			close(idsChannel)
			break
		}

		for _, bd := range resp.GetData() {
			idsChannel <- bd.GetMetadata().GetId()
		}
	}

	// In other loop: read from channel, and 10 IDs concurrently at a time, do IncludeBinary
	var nextID string
	var done bool
	wg := sync.WaitGroup{}
	for {
		for i := 0; i < numConcurrentRequests; i++ {
			nextID = <-idsChannel

			// If nextID is zero value, the channel has been closed and there are no more IDs.
			if nextID == "" {
				done = true
				break
			}
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				err := downloadBinary(c.dataClient, dst, nextID)
				// TODO: Cancel other requests and return from parent function if we receive an error. Use an error
				//       channel and cancelCTx for this.
				if err != nil {
					fmt.Println(err)
				}
			}(nextID)
		}
		wg.Wait()
		if done {
			break
		}
	}

	return nil
}

func downloadBinary(client datapb.DataServiceClient, dst string, id string) error {
	resp, err := client.BinaryDataByIDs(context.Background(), &datapb.BinaryDataByIDsRequest{
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
		return errors.Wrapf(err, fmt.Sprintf("error creating file for file %s", datum.GetMetadata().GetId()))
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
