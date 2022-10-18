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
	rdkutils "go.viam.com/rdk/utils"
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
	fmt.Println(filter.OrgIds)

	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	// TODO: Should we use more limited perms?
	// TODO: Probably shouldn't re-download files we already have? Maybe we actually should have an exlclude_ids field
	if err := os.MkdirAll(filepath.Join(dst, dataDir), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dst, metadataDir), os.ModePerm); err != nil {
		return err
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
		mds := resp.GetMetadata()
		if len(mds) != 1 {
			return errors.Errorf("expected a single response, received %d", len(mds))
		}
		data := resp.GetData()
		if len(data) != 1 {
			return errors.Errorf("expected a single response, received %d", len(data))
		}

		md := mds[0]
		datum := data[0]
		mdJsonBytes, err := protojson.Marshal(md)
		if err != nil {
			return err
		}
		jsonFile, err := os.Create(filepath.Join(dst, "metadata", datum.GetId()+".json"))
		if err != nil {
			return err
		}
		if _, err := jsonFile.Write(mdJsonBytes); err != nil {
			return err
		}

		// TODO: Include file name in metadata.

		gzippedBytes := datum.GetBinary()
		r, err := gzip.NewReader(bytes.NewBuffer(gzippedBytes))
		if err != nil {
			return err
		}

		// TODO: We need to store file extension too. In sync we map from ext -> mime type, so this is already available.
		//       Or maybe we can just get this from file name.
		ext := mimeTypeToFileExt(md.GetMimeType())

		dataFile, err := os.Create(filepath.Join(dst, "data", datum.GetId()+ext))
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

	// Make dst/data and dst/metadata directory.
	if err := os.MkdirAll(filepath.Join(dst, dataDir), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dst, metadataDir), os.ModePerm); err != nil {
		return err
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
			// TODO: This should never happen. Should this notify user? Error? Or just skip?
			fmt.Println("Received empty tabular data")
			continue
		}
		m := d.AsMap()
		m["TimeRequested"] = datum.GetTimeRequested()
		m["TimeReceived"] = datum.GetTimeReceived()
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

func mimeTypeToFileExt(mime string) string {
	switch mime {
	case rdkutils.MimeTypePCD:
		return ".pcd"
	case rdkutils.MimeTypeJPEG:
		return ".jpg"
	case rdkutils.MimeTypePNG:
		return ".png"
	default:
		return ".dat"
	}
}
