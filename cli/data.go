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
	"github.com/urfave/cli/v2"
	datapb "go.viam.com/api/app/data/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	dataDir                  = "data"
	metadataDir              = "metadata"
	defaultParallelDownloads = 100
	maxRetryCount            = 5
	logEveryN                = 100
	maxLimit                 = 100

	// DataFlagDestination is the output directory for downloaded data.
	DataFlagDestination = "destination"
	// DataFlagDataType is the data type to be downloaded: either binary or tabular.
	DataFlagDataType = "data-type"
	// DataFlagOrgIDs is the orgs filter.
	DataFlagOrgIDs = "org-ids"
	// DataFlagLocationIDs is the location filter.
	DataFlagLocationIDs = "location-ids"
	// DataFlagRobotID is the robot-id filter.
	DataFlagRobotID = "robot-id"
	// DataFlagPartID is the robot-id filter.
	DataFlagPartID = "part-id"
	// DataFlagRobotName is the robot name filter.
	DataFlagRobotName = "robot-name"
	// DataFlagPartName is the part name filter.
	DataFlagPartName = "part-name"
	// DataFlagComponentType is the component type filter.
	DataFlagComponentType = "component-type"
	// DataFlagComponentName is the component name filter.
	DataFlagComponentName = "component-name"
	// DataFlagMethod is the method filter.
	DataFlagMethod = "method"
	// DataFlagMimeTypes is the mime types filter.
	DataFlagMimeTypes = "mime-types"
	// DataFlagStart is an ISO-8601 timestamp indicating the start of the interval filter.
	DataFlagStart = "start"
	// DataFlagEnd is an ISO-8601 timestamp indicating the end of the interval filter.
	DataFlagEnd = "end"
	// DataFlagParallelDownloads is the number of download requests to make in parallel.
	DataFlagParallelDownloads = "parallel"
	// DataFlagTags is the tags filter.
	DataFlagTags = "tags"
	// DataFlagBboxLabels is the bbox labels filter.
	DataFlagBboxLabels = "bbox-labels"

	dataTypeBinary  = "binary"
	dataTypeTabular = "tabular"
)

// DataExportAction is the corresponding action for 'data export'.
func DataExportAction(c *cli.Context) error {
	filter, err := createDataFilter(c)
	if err != nil {
		return err
	}

	client, err := newAppClient(c)
	if err != nil {
		return err
	}

	switch c.String(DataFlagDataType) {
	case dataTypeBinary:
		if err := client.binaryData(c.Path(DataFlagDestination), filter, c.Uint(DataFlagParallelDownloads)); err != nil {
			return err
		}
	case dataTypeTabular:
		if err := client.tabularData(c.Path(DataFlagDestination), filter); err != nil {
			return err
		}
	default:
		return errors.Errorf("%s must be binary or tabular, got %q", DataFlagDataType, c.String(DataFlagDataType))
	}
	return nil
}

// DataDeleteAction is the corresponding action for 'data delete'.
func DataDeleteAction(c *cli.Context) error {
	filter, err := createDataFilter(c)
	if err != nil {
		return err
	}

	client, err := newAppClient(c)
	if err != nil {
		return err
	}

	switch c.String(DataFlagDataType) {
	case dataTypeBinary:
		if err := client.deleteBinaryData(filter); err != nil {
			return err
		}
	case dataTypeTabular:
		if err := client.deleteTabularData(filter); err != nil {
			return err
		}
	default:
		return errors.Errorf("%s must be binary or tabular, got %q", DataFlagDataType, c.String(DataFlagDataType))
	}

	return nil
}

func createDataFilter(c *cli.Context) (*datapb.Filter, error) {
	filter := &datapb.Filter{}

	if c.StringSlice(DataFlagOrgIDs) != nil {
		filter.OrganizationIds = c.StringSlice(DataFlagOrgIDs)
	}
	if c.StringSlice(DataFlagLocationIDs) != nil {
		filter.LocationIds = c.StringSlice(DataFlagLocationIDs)
	}
	if c.String(DataFlagRobotID) != "" {
		filter.RobotId = c.String(DataFlagRobotID)
	}
	if c.String(DataFlagPartID) != "" {
		filter.PartId = c.String(DataFlagPartID)
	}
	if c.String(DataFlagRobotName) != "" {
		filter.RobotName = c.String(DataFlagRobotName)
	}
	if c.String(DataFlagPartName) != "" {
		filter.PartName = c.String(DataFlagPartName)
	}
	if c.String(DataFlagComponentType) != "" {
		filter.ComponentType = c.String(DataFlagComponentType)
	}
	if c.String(DataFlagComponentName) != "" {
		filter.ComponentName = c.String(DataFlagComponentName)
	}
	if c.String(DataFlagMethod) != "" {
		filter.Method = c.String(DataFlagMethod)
	}
	if len(c.StringSlice(DataFlagMimeTypes)) != 0 {
		filter.MimeType = c.StringSlice(DataFlagMimeTypes)
	}
	if c.StringSlice(DataFlagTags) != nil {
		switch {
		case len(c.StringSlice(DataFlagTags)) == 1 && c.StringSlice(DataFlagTags)[0] == "tagged":
			filter.TagsFilter = &datapb.TagsFilter{
				Type: datapb.TagsFilterType_TAGS_FILTER_TYPE_TAGGED,
			}
		case len(c.StringSlice(DataFlagTags)) == 1 && c.StringSlice(DataFlagTags)[0] == "untagged":
			filter.TagsFilter = &datapb.TagsFilter{
				Type: datapb.TagsFilterType_TAGS_FILTER_TYPE_UNTAGGED,
			}
		default:
			filter.TagsFilter = &datapb.TagsFilter{
				Type: datapb.TagsFilterType_TAGS_FILTER_TYPE_MATCH_BY_OR,
				Tags: c.StringSlice(DataFlagTags),
			}
		}
	}
	if len(c.StringSlice(DataFlagBboxLabels)) != 0 {
		filter.BboxLabels = c.StringSlice(DataFlagBboxLabels)
	}
	var start *timestamppb.Timestamp
	var end *timestamppb.Timestamp
	timeLayout := time.RFC3339
	if c.String(DataFlagStart) != "" {
		t, err := time.Parse(timeLayout, c.String(DataFlagStart))
		if err != nil {
			return nil, errors.Wrap(err, "could not parse start flag")
		}
		start = timestamppb.New(t)
	}
	if c.String(DataFlagEnd) != "" {
		t, err := time.Parse(timeLayout, c.String(DataFlagEnd))
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
func (c *appClient) binaryData(dst string, filter *datapb.Filter, parallelDownloads uint) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	if err := makeDestinationDirs(dst); err != nil {
		return errors.Wrapf(err, "could not create destination directories")
	}

	if parallelDownloads == 0 {
		parallelDownloads = defaultParallelDownloads
	}

	ids := make(chan *datapb.BinaryID, parallelDownloads)
	// Give channel buffer of 1+parallelDownloads because that is the number of goroutines that may be passing an
	// error into this channel (1 get ids routine + parallelDownloads download routines).
	errs := make(chan error, 1+parallelDownloads)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup

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
		var nextID *datapb.BinaryID
		var done bool
		var numFilesDownloaded atomic.Int32
		var downloadWG sync.WaitGroup
		for {
			for i := uint(0); i < parallelDownloads; i++ {
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

func downloadBinary(ctx context.Context, client datapb.DataServiceClient, dst string, id *datapb.BinaryID) error {
	var resp *datapb.BinaryDataByIDsResponse
	var err error
	for count := 0; count < maxRetryCount; count++ {
		resp, err = client.BinaryDataByIDs(ctx, &datapb.BinaryDataByIDsRequest{
			BinaryIds:     []*datapb.BinaryID{id},
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
		return errors.Wrapf(err, fmt.Sprintf("could not create file for datum %s", datum.GetMetadata().GetId()))
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

// tabularData downloads binary data matching filter to dst.
func (c *appClient) tabularData(dst string, filter *datapb.Filter) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	if err := makeDestinationDirs(dst); err != nil {
		return errors.Wrapf(err, "could not create destination directories")
	}

	var err error
	var resp *datapb.TabularDataByFilterResponse
	// TODO(DATA-640): Support export in additional formats.
	//nolint:gosec
	dataFile, err := os.Create(filepath.Join(dst, dataDir, "data.ndjson"))
	if err != nil {
		return errors.Wrapf(err, "could not create data file")
	}
	w := bufio.NewWriter(dataFile)

	fmt.Fprintf(c.c.App.Writer, "downloading..")
	var last string
	mdIndexes := make(map[string]int)
	mdIndex := 0
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
				return errors.Wrapf(err, fmt.Sprintf("could not create metadata file for metadata index %d", mdIndex))
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

	fmt.Fprintf(c.c.App.Writer, "\n")
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

func (c *appClient) deleteBinaryData(filter *datapb.Filter) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
	resp, err := c.dataClient.DeleteBinaryDataByFilter(context.Background(),
		&datapb.DeleteBinaryDataByFilterRequest{Filter: filter})
	if err != nil {
		return errors.Wrapf(err, "received error from server")
	}
	fmt.Fprintf(c.c.App.Writer, "deleted %d files\n", resp.GetDeletedCount())
	return nil
}

// deleteTabularData delete tabular data matching filter.
func (c *appClient) deleteTabularData(filter *datapb.Filter) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
	resp, err := c.dataClient.DeleteTabularDataByFilter(context.Background(),
		&datapb.DeleteTabularDataByFilterRequest{Filter: filter})
	if err != nil {
		return errors.Wrapf(err, "received error from server")
	}
	fmt.Fprintf(c.c.App.Writer, "deleted %d datapoints\n", resp.GetDeletedCount())
	return nil
}
