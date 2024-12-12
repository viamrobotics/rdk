package cli

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	datapb "go.viam.com/api/app/data/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/data"
)

const (
	dataDir       = "data"
	metadataDir   = "metadata"
	maxRetryCount = 5
	logEveryN     = 100
	maxLimit      = 100

	dataTypeBinary  = "binary"
	dataTypeTabular = "tabular"

	dataCommandAdd    = "add"
	dataCommandRemove = "remove"

	gzFileExt = ".gz"

	serverErrorMessage = "received error from server"

	viamCaptureDotSubdir = "/.viam/capture/"

	noExistingADFErrCode = "NotFound"
)

type commonFilterArgs struct {
	OrgIDs        []string
	LocationIDs   []string
	MachineID     string
	PartID        string
	MachineName   string
	PartName      string
	ComponentType string
	ComponentName string
	Method        string
	MimeTypes     []string
	Start         string
	End           string
	BBoxLabels    []string
	FilterTags    []string
	Tags          []string
}

type dataExportArgs struct {
	Destination string
	ChunkLimit  uint
	Parallel    uint
	DataType    string
	Timeout     uint
}

// DataExportAction is the corresponding action for 'data export'.
func DataExportAction(c *cli.Context, args dataExportArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	return client.dataExportAction(c, args)
}

func (c *viamClient) dataExportAction(cCtx *cli.Context, args dataExportArgs) error {
	filter, err := createDataFilter(cCtx)
	if err != nil {
		return err
	}

	switch args.DataType {
	case dataTypeBinary:
		if err := c.binaryData(args.Destination, filter, args.Parallel, args.Timeout); err != nil {
			return err
		}
	case dataTypeTabular:
		if err := c.tabularData(args.Destination, filter, args.ChunkLimit); err != nil {
			return err
		}
	default:
		return errors.Errorf("%s must be binary or tabular, got %q", dataFlagDataType, args.DataType)
	}
	return nil
}

type dataTagByFilterArgs struct {
	Tags []string
}

// DataTagActionByFilter is the corresponding action for 'data tag filter'.
func DataTagActionByFilter(c *cli.Context, args dataTagByFilterArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	filter, err := createDataFilter(c)
	if err != nil {
		return err
	}

	switch c.Command.Name {
	case dataCommandAdd:
		if err := client.dataAddTagsToBinaryByFilter(filter, args.Tags); err != nil {
			return err
		}
		return nil
	case dataCommandRemove:
		if err := client.dataRemoveTagsFromBinaryByFilter(filter, args.Tags); err != nil {
			return err
		}
		return nil
	default:
		return errors.Errorf("command must be add or remove, got %q", c.Command.Name)
	}
}

type dataTagByIDsArgs struct {
	Tags       []string
	OrgID      string
	LocationID string
	FileIDs    []string
}

// DataTagActionByIds is the corresponding action for 'data tag'.
func DataTagActionByIds(c *cli.Context, args dataTagByIDsArgs) error { //nolint:var-naming,revive
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	switch c.Command.Name {
	case dataCommandAdd:
		if err := client.dataAddTagsToBinaryByIDs(args.Tags, args.OrgID, args.LocationID, args.FileIDs); err != nil {
			return err
		}
		return nil
	case dataCommandRemove:
		if err := client.dataRemoveTagsFromBinaryByIDs(args.Tags, args.OrgID, args.LocationID, args.FileIDs); err != nil {
			return err
		}
		return nil
	default:
		return errors.Errorf("command must be add or remove, got %q", c.Command.Name)
	}
}

// DataDeleteBinaryAction is the corresponding action for 'data delete'.
func DataDeleteBinaryAction(c *cli.Context, args emptyArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	filter, err := createDataFilter(c)
	if err != nil {
		return err
	}
	if err := client.deleteBinaryData(filter); err != nil {
		return err
	}
	return nil
}

type dataDeleteTabularArgs struct {
	OrgID               string
	DeleteOlderThanDays int
}

// DataDeleteTabularAction is the corresponding action for 'data delete-tabular'.
func DataDeleteTabularAction(c *cli.Context, args dataDeleteTabularArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	if err := client.deleteTabularData(args.OrgID, args.DeleteOlderThanDays); err != nil {
		return err
	}
	return nil
}

func createDataFilter(c *cli.Context) (*datapb.Filter, error) {
	args := parseStructFromCtx[commonFilterArgs](c)
	filter := &datapb.Filter{}

	if args.OrgIDs != nil {
		filter.OrganizationIds = args.OrgIDs
	}
	if args.LocationIDs != nil {
		filter.LocationIds = args.LocationIDs
	}
	if args.MachineID != "" {
		filter.RobotId = args.MachineID
	}
	if args.PartID != "" {
		filter.PartId = args.PartID
	}
	if args.MachineName != "" {
		filter.RobotName = args.MachineName
	}
	if args.PartName != "" {
		filter.PartName = args.PartName
	}
	if args.ComponentType != "" {
		filter.ComponentType = args.ComponentType
	}
	if args.ComponentName != "" {
		filter.ComponentName = args.ComponentName
	}
	if args.Method != "" {
		filter.Method = args.Method
	}
	if len(args.MimeTypes) != 0 {
		filter.MimeType = args.MimeTypes
	}
	// We have some weirdness because the --tags flag can mean two completely different things.
	// It could be either tags to filter by, or, if running 'viam data tag' it will mean the
	// tags to add to the data. To account for this, we have to check if we're running the tag
	// command, and if so, to add the filter tags if we pass in --filter-tags.
	if strings.Contains(c.Command.UsageText, "tag") && len(args.FilterTags) != 0 {
		filter.TagsFilter = &datapb.TagsFilter{
			Type: datapb.TagsFilterType_TAGS_FILTER_TYPE_MATCH_BY_OR,
			Tags: args.FilterTags,
		}
	}
	// Similar to the above comment, we only want to add filter tags with --tags if we're NOT
	// running the tag command.
	if !strings.Contains(c.Command.UsageText, "tag") && args.Tags != nil {
		if len(args.FilterTags) == 0 {
			switch {
			case len(args.Tags) == 1 && args.Tags[0] == "tagged":
				filter.TagsFilter = &datapb.TagsFilter{
					Type: datapb.TagsFilterType_TAGS_FILTER_TYPE_TAGGED,
				}
			case len(args.Tags) == 1 && args.Tags[0] == "untagged":
				filter.TagsFilter = &datapb.TagsFilter{
					Type: datapb.TagsFilterType_TAGS_FILTER_TYPE_UNTAGGED,
				}
			default:
				filter.TagsFilter = &datapb.TagsFilter{
					Type: datapb.TagsFilterType_TAGS_FILTER_TYPE_MATCH_BY_OR,
					Tags: args.Tags,
				}
			}
		}
	}
	if len(args.BBoxLabels) != 0 {
		filter.BboxLabels = args.BBoxLabels
	}
	var start *timestamppb.Timestamp
	var end *timestamppb.Timestamp
	timeLayout := time.RFC3339
	if args.Start != "" {
		t, err := time.Parse(timeLayout, args.Start)
		if err != nil {
			return nil, errors.Wrap(err, "could not parse start flag")
		}
		start = timestamppb.New(t)
	}
	if args.End != "" {
		t, err := time.Parse(timeLayout, args.End)
		if err != nil {
			return nil, errors.Wrap(err, "could not parse end flag")
		}
		end = timestamppb.New(t)
	}
	if start != nil || end != nil {
		filter.Interval = &datapb.CaptureInterval{
			Start: start,
			End:   end,
		}
	}
	return filter, nil
}

// BinaryData downloads binary data matching filter to dst.
func (c *viamClient) binaryData(dst string, filter *datapb.Filter, parallelDownloads, timeout uint) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	return c.performActionOnBinaryDataFromFilter(
		func(id *datapb.BinaryID) error {
			return c.downloadBinary(dst, id, timeout)
		},
		filter, parallelDownloads,
		func(i int32) {
			printf(c.c.App.Writer, "Downloaded %d files", i)
		},
	)
}

// performActionOnBinaryDataFromFilter is a helper action that retrieves all BinaryIDs associated with
// a filter in batches and then performs actionOnBinaryData on each binary data in parallel.
// Each time `logEveryN` actions have been performed, the printStatement logs a statement that takes in as
// input how much binary data has been processed thus far.
func (c *viamClient) performActionOnBinaryDataFromFilter(actionOnBinaryData func(*datapb.BinaryID) error,
	filter *datapb.Filter, parallelActions uint, printStatement func(int32),
) error {
	ids := make(chan *datapb.BinaryID, parallelActions)
	// Give channel buffer of 1+parallelActions because that is the number of goroutines that may be passing an
	// error into this channel (1 get ids routine + parallelActions download routines).
	errs := make(chan error, 1+parallelActions)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup

	// In one routine, get all IDs matching the filter and pass them into the ids channel.
	wg.Add(1)
	go func() {
		defer wg.Done()
		// If limit is too high the request can time out, so limit each call to a maximum value of 100.
		limit := min(parallelActions, maxLimit)
		if err := getMatchingBinaryIDs(ctx, c.dataClient, filter, ids, limit); err != nil {
			errs <- err
			cancel()
		}
	}()

	// In parallel, read from ids and perform the action on the binary data for each id in batches of parallelActions.
	wg.Add(1)
	go func() {
		defer wg.Done()
		var nextID *datapb.BinaryID
		var done bool
		var numFilesProcessed atomic.Int32
		var downloadWG sync.WaitGroup
		for {
			for i := uint(0); i < parallelActions; i++ {
				if err := ctx.Err(); err != nil {
					errs <- err
					cancel()
					done = true
					break
				}

				nextID = <-ids

				// If nextID is nil, the channel has been closed and there are no more IDs to be read.
				if nextID == nil {
					done = true
					break
				}

				downloadWG.Add(1)
				go func(id *datapb.BinaryID) {
					defer downloadWG.Done()
					// Perform the desired action on the binary data
					err := actionOnBinaryData(id)
					if err != nil {
						errs <- err
						cancel()
						done = true
					}
					numFilesProcessed.Add(1)
					if numFilesProcessed.Load()%logEveryN == 0 {
						printStatement(numFilesProcessed.Load())
					}
				}(nextID)
			}
			downloadWG.Wait()
			if done {
				break
			}
		}
		if numFilesProcessed.Load()%logEveryN != 0 {
			printStatement(numFilesProcessed.Load())
		}
	}()
	wg.Wait()
	close(errs)

	if err := <-errs; err != nil {
		return err
	}

	return nil
}

// getMatchingBinaryIDs queries client for all BinaryData matching filter, and passes each of their ids into ids.
func getMatchingBinaryIDs(ctx context.Context, client datapb.DataServiceClient, filter *datapb.Filter,
	ids chan *datapb.BinaryID, limit uint,
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
			md := bd.GetMetadata()
			ids <- &datapb.BinaryID{
				FileId:         md.GetId(),
				OrganizationId: md.GetCaptureMetadata().GetOrganizationId(),
				LocationId:     md.GetCaptureMetadata().GetLocationId(),
			}
		}
	}
}

func (c *viamClient) downloadBinary(dst string, id *datapb.BinaryID, timeout uint) error {
	args := parseStructFromCtx[globalArgs](c.c)
	debugf(c.c.App.Writer, args.Debug, "Attempting to download binary file %s", id.FileId)

	var resp *datapb.BinaryDataByIDsResponse
	var err error
	largeFile := false
	// To begin, we assume the file is small and downloadable, so we try getting the binary directly
	for count := 0; count < maxRetryCount; count++ {
		resp, err = c.dataClient.BinaryDataByIDs(c.c.Context, &datapb.BinaryDataByIDsRequest{
			BinaryIds:     []*datapb.BinaryID{id},
			IncludeBinary: !largeFile,
		})
		// If the file is too large, we break and try a different pathway for downloading
		if err == nil || status.Code(err) == codes.ResourceExhausted {
			debugf(c.c.App.Writer, args.Debug, "Small file download file %s: attempt %d/%d succeeded", id.FileId, count+1, maxRetryCount)
			break
		}
		debugf(c.c.App.Writer, args.Debug, "Small file download for file %s: attempt %d/%d failed", id.FileId, count+1, maxRetryCount)
	}
	// For large files, we get the metadata but not the binary itself
	// Resource exhausted is returned when the message we're receiving exceeds the GRPC maximum limit
	if err != nil && status.Code(err) == codes.ResourceExhausted {
		largeFile = true
		for count := 0; count < maxRetryCount; count++ {
			resp, err = c.dataClient.BinaryDataByIDs(c.c.Context, &datapb.BinaryDataByIDsRequest{
				BinaryIds:     []*datapb.BinaryID{id},
				IncludeBinary: !largeFile,
			})
			if err == nil {
				debugf(c.c.App.Writer, args.Debug, "Metadata fetch for file %s: attempt %d/%d succeeded", id.FileId, count+1, maxRetryCount)
				break
			}
			debugf(c.c.App.Writer, args.Debug, "Metadata fetch for file %s: attempt %d/%d failed", id.FileId, count+1, maxRetryCount)
		}
	}
	if err != nil {
		return errors.Wrapf(err, serverErrorMessage)
	}

	data := resp.GetData()
	if len(data) != 1 {
		return errors.Errorf("expected a single response, received %d", len(data))
	}
	datum := data[0]

	fileName := filenameForDownload(datum.GetMetadata())
	// Modify the file name in the metadata to reflect what it will be saved as.
	metadata := datum.GetMetadata()
	metadata.FileName = fileName

	jsonPath := filepath.Join(dst, metadataDir, fileName+".json")
	if err := os.MkdirAll(filepath.Dir(jsonPath), 0o700); err != nil {
		return errors.Wrapf(err, "could not create metadata directory %s", filepath.Dir(jsonPath))
	}
	//nolint:gosec
	jsonFile, err := os.Create(jsonPath)
	if err != nil {
		return err
	}
	mdJSONBytes, err := protojson.Marshal(metadata)
	if err != nil {
		return err
	}
	if _, err := jsonFile.Write(mdJSONBytes); err != nil {
		return err
	}

	var r io.ReadCloser
	if largeFile {
		debugf(c.c.App.Writer, args.Debug, "Attempting file %s as a large file download", id.FileId)
		// Make request to the URI for large files since we exceed the message limit for gRPC
		req, err := http.NewRequestWithContext(c.c.Context, http.MethodGet, datum.GetMetadata().GetUri(), nil)
		if err != nil {
			return errors.Wrapf(err, serverErrorMessage)
		}

		// Set the headers so HTTP requests that are not gRPC calls can still be authenticated in app
		// We can authenticate via token or API key, so we try both.
		token, ok := c.conf.Auth.(*token)
		if ok {
			req.Header.Add(rpc.MetadataFieldAuthorization, rpc.AuthorizationValuePrefixBearer+token.AccessToken)
		}
		apiKey, ok := c.conf.Auth.(*apiKey)
		if ok {
			req.Header.Add("key_id", apiKey.KeyID)
			req.Header.Add("key", apiKey.KeyCrypto)
		}

		httpClient := &http.Client{Timeout: time.Duration(timeout) * time.Second}

		var res *http.Response
		for count := 0; count < maxRetryCount; count++ {
			res, err = httpClient.Do(req)

			if err == nil && res.StatusCode == http.StatusOK {
				debugf(c.c.App.Writer, args.Debug,
					"Large file download for file %s: attempt %d/%d succeeded", id.FileId, count+1, maxRetryCount)
				break
			}
			debugf(c.c.App.Writer, args.Debug, "Large file download for file %s: attempt %d/%d failed", id.FileId, count+1, maxRetryCount)
		}

		if err != nil {
			debugf(c.c.App.Writer, args.Debug, "Failed downloading large file %s: %s", id.FileId, err)
			return errors.Wrapf(err, serverErrorMessage)
		}
		if res.StatusCode != http.StatusOK {
			debugf(c.c.App.Writer, args.Debug, "Failed downloading large file %s: Server returned %d response", id.FileId, res.StatusCode)
			return errors.New(serverErrorMessage)
		}
		defer func() {
			utils.UncheckedError(res.Body.Close())
		}()

		r = res.Body
	} else {
		// If the binary has not already been populated as large file download,
		// get the binary data from the response.
		r = io.NopCloser(bytes.NewReader(datum.GetBinary()))
	}

	dataPath := filepath.Join(dst, dataDir, fileName)
	ext := datum.GetMetadata().GetFileExt()

	// If the file is gzipped, unzip.
	if ext == gzFileExt {
		r, err = gzip.NewReader(r)
		if err != nil {
			debugf(c.c.App.Writer, args.Debug, "Failed unzipping file %s: %s", id.FileId, err)
			return err
		}
	} else if filepath.Ext(dataPath) != ext {
		// If the file name did not already include the extension (e.g. for data capture files), add it.
		// Don't do this for files that we're unzipping.
		dataPath += ext
	}

	if err := os.MkdirAll(filepath.Dir(dataPath), 0o700); err != nil {
		debugf(c.c.App.Writer, args.Debug, "Failed creating data directory %s: %s", dataPath, err)
		return errors.Wrapf(err, "could not create data directory %s", filepath.Dir(dataPath))
	}
	//nolint:gosec
	dataFile, err := os.Create(dataPath)
	if err != nil {
		debugf(c.c.App.Writer, args.Debug, "Failed creating file %s: %s", id.FileId, err)
		return errors.Wrapf(err, fmt.Sprintf("could not create file for datum %s", datum.GetMetadata().GetId())) //nolint:govet
	}
	//nolint:gosec
	if _, err := io.Copy(dataFile, r); err != nil {
		debugf(c.c.App.Writer, args.Debug, "Failed writing data to file %s: %s", id.FileId, err)
		return err
	}
	if err := r.Close(); err != nil {
		debugf(c.c.App.Writer, args.Debug, "Failed closing file %s: %s", id.FileId, err)
		return err
	}
	return nil
}

// transform datum's filename to a destination path on this computer.
func filenameForDownload(meta *datapb.BinaryMetadata) string {
	timeRequested := meta.GetTimeRequested().AsTime().Format(time.RFC3339Nano)
	fileName := meta.GetFileName()

	// If there is no file name, this is a data capture file.
	if fileName == "" {
		fileName = timeRequested + "_" + meta.GetId() + meta.GetFileExt()
	} else if filepath.Dir(fileName) == "." {
		// If the file name does not contain a directory, prepend if with a requested time so that it is sorted.
		// Otherwise, keep the file name as-is to maintain the directory structure that the user uploaded the file with.
		fileName = timeRequested + "_" + fileName
	}

	// If the file name is not a data capture file but was manually saved in the default viam capture directory, remove
	// that directory. Otherwise, the file will be hidden due to the .viam directory.
	// Use ReplaceAll rather than TrimPrefix since it will be stored under os.Getenv("HOME"), which differs between upload
	// to export time.
	fileName = strings.ReplaceAll(fileName, viamCaptureDotSubdir, "")

	// The file name will end with .gz if the user uploaded a gzipped file. We will unzip it below, so remove the last
	// .gz from the file name. If the user has gzipped the file multiple times, we will only unzip once.
	if filepath.Ext(fileName) == gzFileExt {
		fileName = strings.TrimSuffix(fileName, gzFileExt)
	}

	// Replace reserved characters.
	fileName = data.CaptureFilePathWithReplacedReservedChars(fileName)

	return fileName
}

// tabularData downloads binary data matching filter to dst.
func (c *viamClient) tabularData(dst string, filter *datapb.Filter, limit uint) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	if err := makeDestinationDirs(dst); err != nil {
		return errors.Wrapf(err, "could not create destination directories")
	}

	var err error
	//nolint:deprecated,staticcheck
	var resp *datapb.TabularDataByFilterResponse
	// TODO(DATA-640): Support export in additional formats.
	//nolint:gosec
	dataFile, err := os.Create(filepath.Join(dst, dataDir, "data.ndjson"))
	if err != nil {
		return errors.Wrapf(err, "could not create data file")
	}
	w := bufio.NewWriter(dataFile)

	fmt.Fprintf(c.c.App.Writer, "Downloading..") //nolint:errcheck // no newline
	var last string
	mdIndexes := make(map[string]int)
	mdIndex := 0
	for {
		for count := 0; count < maxRetryCount; count++ {
			//nolint:deprecated,staticcheck
			resp, err = c.dataClient.TabularDataByFilter(context.Background(), &datapb.TabularDataByFilterRequest{
				DataRequest: &datapb.DataRequest{
					Filter: filter,
					Limit:  uint64(limit),
					Last:   last,
				},
				CountOnly: false,
			})
			fmt.Fprintf(c.c.App.Writer, ".") //nolint:errcheck // no newline
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
		// Map the current response's metadata indexes to those combined across all responses.
		localToGlobalMDIndex := make(map[int]int)
		for i, md := range mds {
			currMDIndex, ok := mdIndexes[md.String()]
			if ok {
				localToGlobalMDIndex[i] = currMDIndex
				continue // Already have this metadata file, so skip creating it again.
			}
			mdIndexes[md.String()] = mdIndex
			localToGlobalMDIndex[i] = mdIndex

			mdJSONBytes, err := protojson.Marshal(md)
			if err != nil {
				return errors.Wrap(err, "could not marshal metadata")
			}
			//nolint:gosec
			mdFile, err := os.Create(filepath.Join(dst, metadataDir, strconv.Itoa(mdIndex)+".json"))
			if err != nil {
				return errors.Wrapf(err, fmt.Sprintf("could not create metadata file for metadata index %d", mdIndex)) //nolint:govet
			}
			if _, err := mdFile.Write(mdJSONBytes); err != nil {
				return errors.Wrapf(err, "could not write to metadata file %s", mdFile.Name())
			}
			if err := mdFile.Close(); err != nil {
				return errors.Wrapf(err, "could not close metadata file %s", mdFile.Name())
			}
			mdIndex++
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
			m["MetadataIndex"] = localToGlobalMDIndex[int(datum.GetMetadataIndex())]
			j, err := json.Marshal(m)
			if err != nil {
				return errors.Wrap(err, "could not marshal JSON response")
			}
			_, err = w.Write(append(j, []byte("\n")...))
			if err != nil {
				return errors.Wrapf(err, "could not write to file %s", dataFile.Name())
			}
		}
	}

	printf(c.c.App.Writer, "") // newline
	if err := w.Flush(); err != nil {
		return errors.Wrapf(err, "could not flush writer for %s", dataFile.Name())
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

func (c *viamClient) deleteBinaryData(filter *datapb.Filter) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
	resp, err := c.dataClient.DeleteBinaryDataByFilter(context.Background(),
		&datapb.DeleteBinaryDataByFilterRequest{Filter: filter})
	if err != nil {
		return errors.Wrapf(err, serverErrorMessage)
	}
	printf(c.c.App.Writer, "Deleted %d files", resp.GetDeletedCount())
	return nil
}

func (c *viamClient) dataAddTagsToBinaryByFilter(filter *datapb.Filter, tags []string) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
	_, err := c.dataClient.AddTagsToBinaryDataByFilter(context.Background(),
		&datapb.AddTagsToBinaryDataByFilterRequest{Filter: filter, Tags: tags})
	if err != nil {
		return errors.Wrapf(err, serverErrorMessage)
	}
	printf(c.c.App.Writer, "Successfully tagged data")
	return nil
}

func (c *viamClient) dataRemoveTagsFromBinaryByFilter(filter *datapb.Filter, tags []string) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
	_, err := c.dataClient.RemoveTagsFromBinaryDataByFilter(context.Background(),
		&datapb.RemoveTagsFromBinaryDataByFilterRequest{Filter: filter, Tags: tags})
	if err != nil {
		return errors.Wrapf(err, serverErrorMessage)
	}
	printf(c.c.App.Writer, "Successfully removed tags from data")
	return nil
}

func (c *viamClient) dataAddTagsToBinaryByIDs(tags []string, orgID, locationID string, fileIDs []string) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
	binaryData := make([]*datapb.BinaryID, 0, len(fileIDs))
	for _, fileID := range fileIDs {
		binaryData = append(binaryData, &datapb.BinaryID{
			OrganizationId: orgID,
			LocationId:     locationID,
			FileId:         fileID,
		})
	}
	_, err := c.dataClient.AddTagsToBinaryDataByIDs(context.Background(),
		&datapb.AddTagsToBinaryDataByIDsRequest{Tags: tags, BinaryIds: binaryData})
	if err != nil {
		return errors.Wrapf(err, serverErrorMessage)
	}
	printf(c.c.App.Writer, "Added tags %v to data", tags)
	return nil
}

// dataRemoveTagsFromData removes tags from data, with the specified org ID, location ID,
// and file IDs.
func (c *viamClient) dataRemoveTagsFromBinaryByIDs(tags []string, orgID, locationID string, fileIDs []string) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
	binaryData := make([]*datapb.BinaryID, 0, len(fileIDs))
	for _, fileID := range fileIDs {
		binaryData = append(binaryData, &datapb.BinaryID{
			OrganizationId: orgID,
			LocationId:     locationID,
			FileId:         fileID,
		})
	}
	_, err := c.dataClient.RemoveTagsFromBinaryDataByIDs(context.Background(),
		&datapb.RemoveTagsFromBinaryDataByIDsRequest{Tags: tags, BinaryIds: binaryData})
	if err != nil {
		return errors.Wrapf(err, serverErrorMessage)
	}
	printf(c.c.App.Writer, "Removed tags %v from data", tags)
	return nil
}

// deleteTabularData delete tabular data matching filter.
func (c *viamClient) deleteTabularData(orgID string, deleteOlderThanDays int) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
	resp, err := c.dataClient.DeleteTabularData(context.Background(),
		&datapb.DeleteTabularDataRequest{OrganizationId: orgID, DeleteOlderThanDays: uint32(deleteOlderThanDays)})
	if err != nil {
		return errors.Wrapf(err, serverErrorMessage)
	}
	printf(c.c.App.Writer, "Deleted %d datapoints", resp.GetDeletedCount())
	return nil
}

type dataAddToDatasetByIDsArgs struct {
	DatasetID  string
	OrgID      string
	LocationID string
	FileIDs    []string
}

// DataAddToDatasetByIDs is the corresponding action for 'data dataset add ids'.
func DataAddToDatasetByIDs(c *cli.Context, args dataAddToDatasetByIDsArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	if err := client.dataAddToDatasetByIDs(args.DatasetID, args.OrgID,
		args.LocationID, args.FileIDs); err != nil {
		return err
	}
	return nil
}

// dataAddToDatasetByIDs adds data, with the specified org ID, location ID, and file IDs to the dataset corresponding to the dataset ID.
func (c *viamClient) dataAddToDatasetByIDs(datasetID, orgID, locationID string, fileIDs []string) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
	binaryData := make([]*datapb.BinaryID, 0, len(fileIDs))
	for _, fileID := range fileIDs {
		binaryData = append(binaryData, &datapb.BinaryID{
			OrganizationId: orgID,
			LocationId:     locationID,
			FileId:         fileID,
		})
	}
	_, err := c.dataClient.AddBinaryDataToDatasetByIDs(context.Background(),
		&datapb.AddBinaryDataToDatasetByIDsRequest{DatasetId: datasetID, BinaryIds: binaryData})
	if err != nil {
		return errors.Wrapf(err, serverErrorMessage)
	}
	printf(c.c.App.Writer, "Added data to dataset ID %s", datasetID)
	return nil
}

type dataAddToDatasetByFilterArgs struct {
	DatasetID string
}

// DataAddToDatasetByFilter is the corresponding action for 'data dataset add filter'.
func DataAddToDatasetByFilter(c *cli.Context, args dataAddToDatasetByFilterArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	filter, err := createDataFilter(c)
	if err != nil {
		return err
	}
	if err := client.dataAddToDatasetByFilter(filter, args.DatasetID); err != nil {
		return err
	}
	return nil
}

// dataAddToDatasetByFilter adds data, with the specified filter to the dataset corresponding to the dataset ID.
func (c *viamClient) dataAddToDatasetByFilter(filter *datapb.Filter, datasetID string) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
	parallelActions := uint(100)

	return c.performActionOnBinaryDataFromFilter(
		func(id *datapb.BinaryID) error {
			_, err := c.dataClient.AddBinaryDataToDatasetByIDs(c.c.Context,
				&datapb.AddBinaryDataToDatasetByIDsRequest{DatasetId: datasetID, BinaryIds: []*datapb.BinaryID{id}})
			return err
		},
		filter, parallelActions,
		func(i int32) {
			printf(c.c.App.Writer, "Added %d files to dataset ID %s", i, datasetID)
		})
}

type dataRemoveFromDatasetArgs struct {
	DatasetID  string
	OrgID      string
	LocationID string
	FileIDs    []string
}

// DataRemoveFromDataset is the corresponding action for 'data dataset remove'.
func DataRemoveFromDataset(c *cli.Context, args dataRemoveFromDatasetArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	if err := client.dataRemoveFromDataset(args.DatasetID, args.OrgID,
		args.LocationID, args.FileIDs); err != nil {
		return err
	}
	return nil
}

// dataRemoveFromDataset removes data, with the specified org ID, location ID,
// and file IDs from the dataset corresponding to the dataset ID.
func (c *viamClient) dataRemoveFromDataset(datasetID, orgID, locationID string, fileIDs []string) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
	binaryData := make([]*datapb.BinaryID, 0, len(fileIDs))
	for _, fileID := range fileIDs {
		binaryData = append(binaryData, &datapb.BinaryID{
			OrganizationId: orgID,
			LocationId:     locationID,
			FileId:         fileID,
		})
	}
	_, err := c.dataClient.RemoveBinaryDataFromDatasetByIDs(context.Background(),
		&datapb.RemoveBinaryDataFromDatasetByIDsRequest{DatasetId: datasetID, BinaryIds: binaryData})
	if err != nil {
		return errors.Wrapf(err, serverErrorMessage)
	}
	printf(c.c.App.Writer, "Removed data from dataset ID %s", datasetID)
	return nil
}

type dataConfigureDatabaseUserArgs struct {
	OrgID    string
	Password string
}

// DataConfigureDatabaseUserConfirmation is the Before action for 'data database configure'.
// it asks for the user to confirm that they are aware that they are changing the authentication
// credentials of their database.
func DataConfigureDatabaseUserConfirmation(c *cli.Context, args dataConfigureDatabaseUserArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	res, err := client.dataGetDatabaseConnection(args.OrgID)
	// if the error is adf doesn't exist for org yet, continue and skip HasDatabaseUser check
	if err != nil && !strings.Contains(err.Error(), noExistingADFErrCode) {
		return err
	}

	// skip this check if we don't have an existing ADF instance
	if err == nil && res.HasDatabaseUser {
		yellow := "\033[1;33m%s\033[0m"
		printf(c.App.Writer, yellow, "WARNING!!\n")
		printf(c.App.Writer, yellow, "You or someone else in your organization have already created a user.\n")
		printf(c.App.Writer, yellow, "The following steps update the password for that user.\n")
		printf(c.App.Writer, yellow, "Once you have updated the password, you will need to update all dashboards or")
		printf(c.App.Writer, yellow, "other integrations relying on this password.\n")
		printf(c.App.Writer, yellow, "Do you want to continue?")
		printf(c.App.Writer, "Continue: y/n")
		if err := c.Err(); err != nil {
			return err
		}

		rawInput, err := bufio.NewReader(c.App.Reader).ReadString('\n')
		if err != nil {
			return err
		}

		input := strings.ToUpper(strings.TrimSpace(rawInput))
		if input != "Y" {
			return errors.New("aborted")
		}
	}

	return nil
}

// DataConfigureDatabaseUser is the corresponding action for 'data database configure'.
func DataConfigureDatabaseUser(c *cli.Context, args dataConfigureDatabaseUserArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	if err := client.dataConfigureDatabaseUser(args.OrgID, args.Password); err != nil {
		return err
	}
	return nil
}

// dataConfigureDatabaseUser accepts a Viam organization ID and a password for the database user
// being configured. Viam uses gRPC over TLS, so the entire request will be encrypted while in
// flight, including the password.
func (c *viamClient) dataConfigureDatabaseUser(orgID, password string) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
	_, err := c.dataClient.ConfigureDatabaseUser(context.Background(),
		&datapb.ConfigureDatabaseUserRequest{OrganizationId: orgID, Password: password})
	if err != nil {
		return errors.Wrapf(err, serverErrorMessage)
	}
	printf(c.c.App.Writer, "Configured database user for org %s", orgID)
	return nil
}

type dataGetDatabaseConnectionArgs struct {
	OrgID string
}

// DataGetDatabaseConnection is the corresponding action for 'data database hostname'.
func DataGetDatabaseConnection(c *cli.Context, args dataGetDatabaseConnectionArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	res, err := client.dataGetDatabaseConnection(args.OrgID)
	if err != nil {
		return err
	}
	printf(client.c.App.Writer, "MongoDB Atlas Data Federation instance hostname: %s", res.GetHostname())
	printf(client.c.App.Writer, "MongoDB Atlas Data Federation instance connection URI: %s", res.GetMongodbUri())
	return nil
}

// dataGetDatabaseConnection gets the hostname of the MongoDB Atlas Data Federation instance
// for the given organization ID.
func (c *viamClient) dataGetDatabaseConnection(orgID string) (*datapb.GetDatabaseConnectionResponse, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	res, err := c.dataClient.GetDatabaseConnection(context.Background(), &datapb.GetDatabaseConnectionRequest{OrganizationId: orgID})
	if err != nil {
		return nil, errors.Wrapf(err, serverErrorMessage)
	}
	return res, nil
}
