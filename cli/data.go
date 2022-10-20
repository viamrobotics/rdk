package cli

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
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

	skip := int64(0)
	for {
		resp, err := c.dataClient.BinaryDataByFilter(context.Background(), &datapb.BinaryDataByFilterRequest{
			DataRequest: &datapb.DataRequest{
				Filter: filter,
				Skip:   skip,
				Limit:  1,
			},
			IncludeBinary: true,
			CountOnly:     false,
		})
		// TODO: Make sure EOF is properly interpreted. Iirc rpc errors aren't properly parsed by errors.Is.
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		// TODO: change api to make metadata and data 1:1 so don't need to merge datum level metadata and capturemetadata
		mds := resp.GetMetadata()
		if len(mds) != 1 {
			return errors.Errorf("expected a single metadata response, received %d", len(mds))
		}
		data := resp.GetData()
		if len(data) != 1 {
			return errors.Errorf("expected a single data response, received %d", len(data))
		}

		md := mds[0]
		datum := data[0]

		mdJsonBytes, err := protojson.Marshal(md)
		if err != nil {
			return err
		}

		jsonFile, err := os.Create(filepath.Join(dst, "metadata", md.GetId()+".json"))
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

		dataFile, err := os.Create(filepath.Join(dst, "data", datum.GetId()+md.GetFileExt()))
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
	// TODO: Use textpb insted of ndjson, and save multiple files.
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
	if err := os.MkdirAll(filepath.Join(dst, dataDir), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dst, metadataDir), os.ModePerm); err != nil {
		return err
	}
	return nil
}
