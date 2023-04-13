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
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	datapb "go.viam.com/api/app/data/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	dataDir                  = "data"
	metadataDir              = "metadata"
	defaultParallelDownloads = 100
	maxRetryCount            = 5
	logEveryN                = 100
	maxLimit                 = 100
)

// BinaryData downloads binary data matching filter to dst.
func (c *AppClient) BinaryData(dst string, filter *datapb.Filter, parallelDownloads uint) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	if err := makeDestinationDirs(dst); err != nil {
		return errors.Wrapf(err, "error creating destination directories")
	}

	if parallelDownloads == 0 {
		parallelDownloads = defaultParallelDownloads
	}

	ids := make(chan string, parallelDownloads)
	// Give channel buffer of 1+parallelDownloads because that is the number of goroutines that may be passing an
	// error into this channel (1 get ids routine + parallelDownloads download routines).
	errs := make(chan error, 1+parallelDownloads)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wg := sync.WaitGroup{}

	// In one routine, get all IDs matching the filter and pass them into ids.
	wg.Add(1)
	go func() {
		defer wg.Done()
		// If limit is too high the request can time out, so limit each call to a maximum value of 100.
		var limit uint
		if parallelDownloads > maxLimit {
			limit = maxLimit
		} else {
			limit = parallelDownloads
		}
		if err := getMatchingBinaryIDs(ctx, c.dataClient, filter, ids, limit); err != nil {
			errs <- err
			cancel()
		}
	}()

	// In parallel, read from ids and download the binary for each id in batches of defaultParallelDownloads.
	wg.Add(1)
	go func() {
		defer wg.Done()
		var nextID string
		var done bool
		numFilesDownloaded := &atomic.Int32{}
		downloadWG := sync.WaitGroup{}
		for {
			for i := uint(0); i < parallelDownloads; i++ {
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
					if numFilesDownloaded.Load()%logEveryN == 0 {
						fmt.Fprintf(c.c.App.Writer, "downloaded %d files\n", numFilesDownloaded.Load())
					}
				}(nextID)
			}
			downloadWG.Wait()
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
	ids chan string, limit uint,
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
				Limit:  uint64(limit),
				Last:   last,
			},
			CountOnly:     false,
			IncludeBinary: false,
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

	if len(data) != 1 {
		return errors.Errorf("expected a single response, received %d", len(data))
	}

	datum := data[0]
	mdJSONBytes, err := protojson.Marshal(datum.GetMetadata())
	if err != nil {
		return err
	}

	timeRequested := datum.GetMetadata().GetTimeRequested().AsTime().Format(time.RFC3339Nano)
	var fileName string
	if datum.GetMetadata().GetFileName() != "" {
		// Can use file ext directly from metadata.
		fileName = timeRequested + "_" + strings.TrimSuffix(datum.GetMetadata().GetFileName(), datum.GetMetadata().GetFileExt())
	} else {
		fileName = timeRequested + "_" + datum.GetMetadata().GetId()
	}

	//nolint:gosec
	jsonFile, err := os.Create(filepath.Join(dst, metadataDir, fileName+".json"))
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
	dataFile, err := os.Create(filepath.Join(dst, dataDir, fileName+datum.GetMetadata().GetFileExt()))
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

	var err error
	var resp *datapb.TabularDataByFilterResponse
	// TODO: [DATA-640] Support export in additional formats.
	//nolint:gosec
	dataFile, err := os.Create(filepath.Join(dst, dataDir, "data"+".ndjson"))
	if err != nil {
		return errors.Wrapf(err, "error creating data file")
	}
	w := bufio.NewWriter(dataFile)

	fmt.Fprintf(c.c.App.Writer, "Downloading..")
	var last string
	var metadataIdx int
	for {
		for count := 0; count < maxRetryCount; count++ {
			resp, err = c.dataClient.TabularDataByFilter(context.Background(), &datapb.TabularDataByFilterRequest{
				DataRequest: &datapb.DataRequest{
					Filter: filter,
					Limit:  maxLimit,
					Last:   last,
				},
				CountOnly: false,
			})
			fmt.Fprintf(c.c.App.Writer, ".")
			if err == nil {
				break
			}
		}
		if err != nil {
			return err
		}

		last = resp.GetLast()
		mds := resp.GetMetadata()
		if len(mds) == 0 {
			break
		}
		for _, md := range mds {
			mdJSONBytes, err := protojson.Marshal(md)
			if err != nil {
				return errors.Wrap(err, "error marshaling metadata")
			}
			//nolint:gosec
			mdFile, err := os.Create(filepath.Join(dst, metadataDir, strconv.Itoa(metadataIdx)+".json"))
			if err != nil {
				return errors.Wrapf(err, fmt.Sprintf("error creating metadata file for metadata index %d", metadataIdx))
			}
			if _, err := mdFile.Write(mdJSONBytes); err != nil {
				return errors.Wrapf(err, "error writing metadata file %s", mdFile.Name())
			}
			if err := mdFile.Close(); err != nil {
				return errors.Wrapf(err, "error closing metadata file %s", mdFile.Name())
			}

			metadataIdx++
		}

		data := resp.GetData()
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
	}

	fmt.Fprintf(c.c.App.Writer, "\n")
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
