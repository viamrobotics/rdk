package cli

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	datapb "go.viam.com/api/app/data/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

const (
	dataDir     = "data"
	metadataDir = "metadata"
)

// BinaryData writes the requested data to the passed directory.
func (c *AppClient) BinaryData(dst string, filter *datapb.Filter) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	if err := makeDestinationDirs(dst); err != nil {
		return errors.Wrapf(err, "error creating destination directories")
	}

	fmt.Println(filter.String())
	skip := int64(0)
	for {
		fmt.Println("on image " + strconv.Itoa(int(skip)))
		resp, err := c.dataClient.BinaryDataByFilter(context.Background(), &datapb.BinaryDataByFilterRequest{
			DataRequest: &datapb.DataRequest{
				Filter: filter,
				Skip:   skip,
				Limit:  1,
			},
			IncludeBinary: true,
			CountOnly:     false,
		})

		if err != nil {
			return errors.Wrapf(err, "received error from server")
		}
		data := resp.GetData()

		// If no data is returned, there is no more data.
		if len(data) == 0 {
			break
		}

		if len(data) != 1 {
			return errors.Errorf("expected a single response, received %d", len(data))
		}

		datum := data[0]
		mdJsonBytes, err := protojson.Marshal(datum.GetMetadata())
		if err != nil {
			return err
		}

		jsonFile, err := os.Create(filepath.Join(dst, "metadata", datum.GetMetadata().GetId()+".json"))
		if err != nil {
			return err
		}
		if _, err := jsonFile.Write(mdJsonBytes); err != nil {
			return err
		}

		gzippedBytes := datum.GetBinary()
		r, err := gzip.NewReader(bytes.NewBuffer(gzippedBytes))
		if err != nil {
			return err
		}

		dataFile, err := os.Create(filepath.Join(dst, "data", datum.GetMetadata().GetId()+datum.GetMetadata().GetFileExt()))
		if _, err := io.Copy(dataFile, r); err != nil {
			return err
		}
		if err := r.Close(); err != nil {
			return err
		}
		skip++
	}

	return nil
}

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
		mdJsonBytes, err := protojson.Marshal(md)
		if err != nil {
			return errors.Wrap(err, "error marshaling metadata")
		}
		mdFile, err := os.Create(filepath.Join(dst, "metadata", strconv.Itoa(i)+".json"))
		if err != nil {
			return errors.Wrapf(err, "error creating metadata file %s", mdFile.Name())
		}
		if _, err := mdFile.Write(mdJsonBytes); err != nil {
			return errors.Wrapf(err, "error writing metadata file %s", mdFile.Name())
		}
		if err := mdFile.Close(); err != nil {
			return errors.Wrapf(err, "error closing metadata file %s", mdFile.Name())
		}
	}

	data := resp.GetData()
	// TODO: [DATA-640] Support export in additional formats.
	dataFile, err := os.Create(filepath.Join(dst, "data", "data"+".ndjson"))
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
