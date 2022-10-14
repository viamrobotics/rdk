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
	fmt.Println(filter.String())
	fmt.Println("Calling BinaryData")
	if err := c.ensureLoggedIn(); err != nil {
		fmt.Println("not logged in :(")
		return err
	}

	// Make dst/data and dst/metadata directory.
	// TODO: perms?
	if err := os.MkdirAll(filepath.Join(dst, "data"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dst, "metadata"), os.ModePerm); err != nil {
		return err
	}

	// TODO: parallelize
	skip := int64(0)
	for {
		fmt.Println(skip)
		// Make BinartDataByFilter request with binary=true
		resp, err := c.dataClient.BinaryDataByFilter(context.Background(), &datapb.BinaryDataByFilterRequest{
			DataRequest: &datapb.DataRequest{
				Filter: filter,
				Skip:   skip,
				Limit:  1,
			},
			IncludeBinary: true,
			CountOnly:     false,
		})
		// TODO: what error indicates none left?
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			fmt.Println(fmt.Sprintf("encountered error %v", err))
			return err
		}
		mds := resp.GetMetadata()
		if len(mds) != 1 {
			return errors.Errorf("expected a single response, received %d", len(mds))
		}
		md := mds[0]
		mdJsonBytes, err := protojson.Marshal(md)
		if err != nil {
			return err
		}

		// TODO: I think we might want to add file name to response for arbitrary files, or at least include it in
		//       the metadata. Until then, just do file id for those too.
		data := resp.GetData()
		if len(data) != 1 {
			return errors.Errorf("expected a single response, received %d", len(data))
		}
		datum := data[0]
		jsonFile, err := os.Create(filepath.Join(dst, "metadata", datum.GetId()+".json"))
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

		// TODO: map mime type to file extension
		// TODO: We need to store file extension too. In sync we map from ext -> mime type, so this is already available.
		fmt.Println(md.GetFileExt())
		fmt.Println(md.GetFileName())
		fmt.Println(md.GetMimeType())
		ext, err := mimeTypeToFileExt(md.GetMimeType())
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(ext)
		dataFile, err := os.Create(filepath.Join(dst, "data", datum.GetId()+ext))
		if _, err := io.Copy(dataFile, r); err != nil {
			return err
		}
		r.Close()
		skip++
	}

	return nil
}

func (c *AppClient) TabularData(dst string, filter *datapb.Filter) error {
	fmt.Println(filter.String())
	fmt.Println("Calling BinaryData")
	if err := c.ensureLoggedIn(); err != nil {
		fmt.Println("not logged in :(")
		return err
	}

	// Make dst/data and dst/metadata directory.
	if err := os.MkdirAll(filepath.Join(dst, dataDir), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dst, metadataDir), os.ModePerm); err != nil {
		return err
	}

	// TODO: parallelize
	skip := int64(0)
	fmt.Println(skip)
	// Make BinartDataByFilter request with binary=true
	resp, err := c.dataClient.TabularDataByFilter(context.Background(), &datapb.TabularDataByFilterRequest{
		DataRequest: &datapb.DataRequest{
			Filter: filter,
			Skip:   skip,
			// TODO: for now don't worry about limit. Just do everything in one request.
			//Limit:  1000,
		},
		CountOnly: false,
	})
	// TODO: what error indicates none left?
	//if errors.Is(err, io.EOF) {
	//	return nil
	//}
	if err != nil {
		fmt.Println(fmt.Sprintf("encountered error %v", err))
		return err
	}
	mds := resp.GetMetadata()
	for i, md := range mds {
		mdJsonBytes, err := protojson.Marshal(md)
		if err != nil {
			return err
		}
		jsonFile, err := os.Create(filepath.Join(dst, "metadata", strconv.Itoa(i)+".json"))
		if err != nil {
			return err
		}
		if _, err := jsonFile.Write(mdJsonBytes); err != nil {
			return err
		}
	}

	// TODO: I think we might want to add file name to response for arbitrary files, or at least include it in
	//       the metadata. Until then, just do file id for those too.
	data := resp.GetData()
	dataFile, err := os.Create(filepath.Join(dst, "data", "data"+".ndjson"))
	w := bufio.NewWriter(dataFile)
	defer w.Flush()
	for _, datum := range data {
		// Write everything as json for now.
		d := datum.GetData()
		if d == nil {
			fmt.Println("received empty tabular data")
			continue
		}
		m := d.AsMap()
		m["TimeRequested"] = datum.GetTimeRequested()
		m["TimeReceived"] = datum.GetTimeReceived()
		j, err := json.Marshal(m)
		if err != nil {
			fmt.Println(fmt.Sprintf("error marshaling json response %v", err))
			return err
		}
		_, err = w.Write(j)
		if err != nil {
			fmt.Println(fmt.Sprintf("error writing json to file %v", err))
			return err
		}
		_, err = w.Write([]byte("\n"))
		if err != nil {
			fmt.Println(fmt.Sprintf("error writing json to file %v", err))
			return err
		}
	}

	return nil
}

func mimeTypeToFileExt(mime string) (string, error) {
	switch mime {
	case rdkutils.MimeTypePCD:
		return ".pcd", nil
	case rdkutils.MimeTypeJPEG:
		return ".jpg", nil
	case rdkutils.MimeTypePNG:
		return ".png", nil
	default:
		return "", errors.Errorf("could not determine file extension for mime type %s", mime)
	}
}
