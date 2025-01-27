// Package app contains a gRPC based data client.
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	pb "go.viam.com/api/app/data/v1"
	setPb "go.viam.com/api/app/dataset/v1"
	syncPb "go.viam.com/api/app/datasync/v1"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/protoutils"
)

// Order specifies the order in which data is returned.
type Order int32

// Order constants define the possible ordering options.
const (
	Unspecified Order = iota
	Descending
	Ascending
)

// DataRequest encapsulates the filter for the data, a limit on the max results returned,
// a last string associated with the last returned document, and the sorting order by time.

//nolint:revive // stutter: Ignore the "stuttering" warning for this type name
type DataRequest struct {
	Filter    Filter
	Limit     int
	Last      string
	SortOrder Order
}

// Filter defines the fields over which we can filter data using a logic AND.
type Filter struct {
	ComponentName   string
	ComponentType   string
	Method          string
	RobotName       string
	RobotID         string
	PartName        string
	PartID          string
	LocationIDs     []string
	OrganizationIDs []string
	MimeType        []string
	Interval        CaptureInterval
	TagsFilter      TagsFilter
	BboxLabels      []string
	DatasetID       string
}

// TagsFilterType specifies how data can be filtered based on tags.
type TagsFilterType int32

// TagsFilterType constants define the ways data can be filtered based on tag matching criteria.
const (
	TagsFilterTypeUnspecified TagsFilterType = iota
	TagsFilterTypeMatchByOr
	TagsFilterTypeTagged
	TagsFilterTypeUntagged
)

// TagsFilter defines the type of filtering and, if applicable, over which tags to perform a logical OR.
type TagsFilter struct {
	Type TagsFilterType
	Tags []string
}

// CaptureMetadata contains information on the settings used for the data capture.
type CaptureMetadata struct {
	OrganizationID   string
	LocationID       string
	RobotName        string
	RobotID          string
	PartName         string
	PartID           string
	ComponentType    string
	ComponentName    string
	MethodName       string
	MethodParameters map[string]interface{}
	Tags             []string
	MimeType         string
}

// CaptureInterval describes the start and end time of the capture in this file.
type CaptureInterval struct {
	Start time.Time
	End   time.Time
}

// TabularData contains data and metadata associated with tabular data.
type TabularData struct {
	Data          map[string]interface{}
	MetadataIndex int
	Metadata      *CaptureMetadata
	TimeRequested time.Time
	TimeReceived  time.Time
}

// BinaryData contains data and metadata associated with binary data.
type BinaryData struct {
	Binary   []byte
	Metadata *BinaryMetadata
}

// BinaryMetadata is the metadata associated with binary data.
type BinaryMetadata struct {
	ID              string
	CaptureMetadata CaptureMetadata
	TimeRequested   time.Time
	TimeReceived    time.Time
	FileName        string
	FileExt         string
	URI             string
	Annotations     *Annotations
	DatasetIDs      []string
}

// BinaryID is the unique identifier for a file that one can request to be retrieved or modified.
type BinaryID struct {
	FileID         string
	OrganizationID string
	LocationID     string
}

// BoundingBox represents a labeled bounding box on an image.
// x and y values are normalized ratios between 0 and 1.
type BoundingBox struct {
	ID             string
	Label          string
	XMinNormalized float64
	YMinNormalized float64
	XMaxNormalized float64
	YMaxNormalized float64
}

// Annotations are data annotations used for machine learning.
type Annotations struct {
	Bboxes []*BoundingBox
}

// TabularDataByFilterResponse represents the result of a TabularDataByFilter query.
// It contains the retrieved tabular data and associated metadata,
// the total number of entries retrieved (Count), and the ID of the last returned page (Last).
type TabularDataByFilterResponse struct {
	TabularData []*TabularData
	Count       int
	Last        string
}

// BinaryDataByFilterResponse represents the result of a BinaryDataByFilter query.
// It contains the retrieved binary data and associated metadata,
// the total number of entries retrieved (Count), and the ID of the last returned page (Last).
type BinaryDataByFilterResponse struct {
	BinaryData []*BinaryData
	Count      int
	Last       string
}

// GetDatabaseConnectionResponse represents the response returned by GetDatabaseConnection.
// It contains the hostname endpoint, a URI for connecting to the MongoDB Atlas Data Federation instance,
// and a flag indicating whether a database user is configured for the Viam organization.
type GetDatabaseConnectionResponse struct {
	Hostname        string
	MongodbURI      string
	HasDatabaseUser bool
}

// GetLatestTabularDataResponse represents the response returned by GetLatestTabularData. It contains the most recently captured data
// payload, the time it was captured, and the time it was synced.
type GetLatestTabularDataResponse struct {
	TimeCaptured time.Time
	TimeSynced   time.Time
	Payload      map[string]interface{}
}

// ExportTabularDataResponse represents the result of an ExportTabularData API call.
type ExportTabularDataResponse struct {
	OrganizationID   string
	LocationID       string
	RobotID          string
	RobotName        string
	PartID           string
	PartName         string
	ResourceName     string
	ResourceSubtype  string
	MethodName       string
	TimeCaptured     time.Time
	MethodParameters map[string]interface{}
	Tags             []string
	Payload          map[string]interface{}
}

// DataSyncClient structs

// SensorMetadata contains the time the sensor data was requested and was received.
type SensorMetadata struct {
	TimeRequested time.Time
	TimeReceived  time.Time
	MimeType      MimeType
	Annotations   *Annotations
}

// SensorData contains the contents and metadata for tabular data.
type SensorData struct {
	Metadata SensorMetadata
	SDStruct map[string]interface{}
	SDBinary []byte
}

// DataType specifies the type of data uploaded.
type DataType int32

// DataType constants define the possible DataType options.
const (
	DataTypeUnspecified DataType = iota
	DataTypeBinarySensor
	DataTypeTabularSensor
	DataTypeFile
)

// MimeType specifies the format of a file being uploaded.
type MimeType int32

// MimeType constants define the possible MimeType options.
const (
	MimeTypeUnspecified MimeType = iota
	MimeTypeJPEG
	MimeTypePNG
	MimeTypePCD
)

// UploadMetadata contains the metadata for binary (image + file) data.
type UploadMetadata struct {
	PartID           string
	ComponentType    string
	ComponentName    string
	MethodName       string
	Type             DataType
	FileName         string
	MethodParameters map[string]interface{}
	FileExtension    string
	Tags             []string
}

// FileData contains the contents of binary (image + file) data.
type FileData struct {
	Data []byte
}

// DataByFilterOptions contains optional parameters for TabularDataByFilter and BinaryDataByFilter.
type DataByFilterOptions struct {
	// No Filter implies all data.
	Filter *Filter
	// Limit is the maximum number of entries to include in a page. Limit defaults to 50 if unspecified.
	Limit int
	// Last indicates the object identifier of the Last-returned data.
	// This is returned by calls to TabularDataByFilter and BinaryDataByFilter as the `Last` value.
	// If provided, the server will return the next data entries after the last object identifier.
	Last                string
	SortOrder           Order
	CountOnly           bool
	IncludeInternalData bool
}

// BinaryDataCaptureUploadOptions represents optional parameters for the BinaryDataCaptureUpload method.
type BinaryDataCaptureUploadOptions struct {
	Type             *DataType
	FileName         *string
	MethodParameters map[string]interface{}
	Tags             []string
	DataRequestTimes *[2]time.Time
}

// TabularDataCaptureUploadOptions represents optional parameters for the TabularDataCaptureUpload method.
type TabularDataCaptureUploadOptions struct {
	Type             *DataType
	FileName         *string
	MethodParameters map[string]interface{}
	FileExtension    *string
	Tags             []string
}

// StreamingDataCaptureUploadOptions represents optional parameters for the StreamingDataCaptureUpload method.
type StreamingDataCaptureUploadOptions struct {
	ComponentType    *string
	ComponentName    *string
	MethodName       *string
	Type             *DataType
	FileName         *string
	MethodParameters map[string]interface{}
	Tags             []string
	DataRequestTimes *[2]time.Time
}

// FileUploadOptions represents optional parameters for the FileUploadFromPath & FileUploadFromBytes methods.
type FileUploadOptions struct {
	ComponentType    *string
	ComponentName    *string
	MethodName       *string
	FileName         *string
	MethodParameters map[string]interface{}
	FileExtension    *string
	Tags             []string
}

// UpdateBoundingBoxOptions contains optional parameters for UpdateBoundingBox.
type UpdateBoundingBoxOptions struct {
	Label *string

	// Normalized coordinates where all coordinates must be in the range [0, 1].
	XMinNormalized *float64
	YMinNormalized *float64
	XMaxNormalized *float64
	YMaxNormalized *float64
}

// Dataset contains the information of a dataset.
type Dataset struct {
	ID             string
	Name           string
	OrganizationID string
	TimeCreated    *time.Time
}

// DataClient implements the DataServiceClient interface.
type DataClient struct {
	dataClient     pb.DataServiceClient
	dataSyncClient syncPb.DataSyncServiceClient
	datasetClient  setPb.DatasetServiceClient
}

func newDataClient(conn rpc.ClientConn) *DataClient {
	dataClient := pb.NewDataServiceClient(conn)
	syncClient := syncPb.NewDataSyncServiceClient(conn)
	setClient := setPb.NewDatasetServiceClient(conn)
	return &DataClient{
		dataClient:     dataClient,
		dataSyncClient: syncClient,
		datasetClient:  setClient,
	}
}

// BsonToGo converts raw BSON data (as [][]byte) into native Go types and interfaces.
// Returns a slice of maps representing the data objects.
func BsonToGo(rawData [][]byte) ([]map[string]interface{}, error) {
	dataObjects := []map[string]interface{}{}
	for _, byteSlice := range rawData {
		// Unmarshal each BSON byte slice into a Go map
		obj := map[string]interface{}{}
		if err := bson.Unmarshal(byteSlice, &obj); err != nil {
			return nil, err
		}
		// Convert the unmarshalled map to native Go types
		convertedObj := convertBsonToNative(obj).(map[string]interface{})
		dataObjects = append(dataObjects, convertedObj)
	}
	return dataObjects, nil
}

// TabularDataByFilter queries tabular data and metadata based on given filters.
// Deprecated: This endpoint will be removed in a future version.
func (d *DataClient) TabularDataByFilter(ctx context.Context, opts *DataByFilterOptions) (*TabularDataByFilterResponse, error) {
	dataReq := pb.DataRequest{}
	var countOnly, includeInternalData bool
	if opts != nil {
		dataReq.Filter = filterToProto(opts.Filter)
		if opts.Limit != 0 {
			dataReq.Limit = uint64(opts.Limit)
		}
		if opts.Last != "" {
			dataReq.Last = opts.Last
		}
		dataReq.SortOrder = orderToProto(opts.SortOrder)
		countOnly = opts.CountOnly
		includeInternalData = opts.IncludeInternalData
	}
	//nolint:deprecated,staticcheck
	resp, err := d.dataClient.TabularDataByFilter(ctx, &pb.TabularDataByFilterRequest{
		DataRequest:         &dataReq,
		CountOnly:           countOnly,
		IncludeInternalData: includeInternalData,
	})
	if err != nil {
		return nil, err
	}
	// TabularData contains tabular data and associated metadata
	dataArray := []*TabularData{}
	var metadata *pb.CaptureMetadata
	for _, tabData := range resp.Data {
		if int(tabData.MetadataIndex) < len(resp.Metadata) {
			metadata = resp.Metadata[tabData.MetadataIndex]
		} else {
			metadata = &pb.CaptureMetadata{}
		}
		data, err := tabularDataFromProto(tabData, metadata)
		if err != nil {
			return nil, err
		}
		dataArray = append(dataArray, data)
	}

	return &TabularDataByFilterResponse{
		TabularData: dataArray,
		Count:       int(resp.Count),
		Last:        resp.Last,
	}, nil
}

// TabularDataBySQL queries tabular data with a SQL query.
func (d *DataClient) TabularDataBySQL(ctx context.Context, organizationID, sqlQuery string) ([]map[string]interface{}, error) {
	resp, err := d.dataClient.TabularDataBySQL(ctx, &pb.TabularDataBySQLRequest{
		OrganizationId: organizationID,
		SqlQuery:       sqlQuery,
	})
	if err != nil {
		return nil, err
	}
	dataObjects, err := BsonToGo(resp.RawData)
	if err != nil {
		return nil, err
	}
	return dataObjects, nil
}

// TabularDataByMQL queries tabular data with MQL (MongoDB Query Language) queries.
func (d *DataClient) TabularDataByMQL(
	ctx context.Context, organizationID string, query []map[string]interface{},
) ([]map[string]interface{}, error) {
	mqlBinary := [][]byte{}
	for _, q := range query {
		binary, err := bson.Marshal(q)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal BSON query: %w", err)
		}
		mqlBinary = append(mqlBinary, binary)
	}

	resp, err := d.dataClient.TabularDataByMQL(ctx, &pb.TabularDataByMQLRequest{
		OrganizationId: organizationID,
		MqlBinary:      mqlBinary,
	})
	if err != nil {
		return nil, err
	}

	result, err := BsonToGo(resp.RawData)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GetLatestTabularData gets the most recent tabular data captured from the specified data source, as well as the time that it was captured
// and synced. If no data was synced to the data source within the last year, LatestTabularDataReturn will be empty.
func (d *DataClient) GetLatestTabularData(ctx context.Context, partID, resourceName, resourceSubtype, methodName string) (
	*GetLatestTabularDataResponse, error,
) {
	resp, err := d.dataClient.GetLatestTabularData(ctx, &pb.GetLatestTabularDataRequest{
		PartId:          partID,
		ResourceName:    resourceName,
		ResourceSubtype: resourceSubtype,
		MethodName:      methodName,
	})
	if err != nil {
		return nil, err
	}

	return &GetLatestTabularDataResponse{
		TimeCaptured: resp.TimeCaptured.AsTime(),
		TimeSynced:   resp.TimeSynced.AsTime(),
		Payload:      resp.Payload.AsMap(),
	}, nil
}

// ExportTabularData returns a stream of ExportTabularDataResponses.
func (d *DataClient) ExportTabularData(
	ctx context.Context, partID, resourceName, resourceSubtype, method string, interval CaptureInterval,
) ([]*ExportTabularDataResponse, error) {
	stream, err := d.dataClient.ExportTabularData(ctx, &pb.ExportTabularDataRequest{
		PartId:          partID,
		ResourceName:    resourceName,
		ResourceSubtype: resourceSubtype,
		MethodName:      method,
		Interval:        captureIntervalToProto(interval),
	})
	if err != nil {
		return nil, err
	}

	var responses []*ExportTabularDataResponse

	for {
		response, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}

		responses = append(responses, exportTabularDataResponseFromProto(response))
	}

	return responses, nil
}

// BinaryDataByFilter queries binary data and metadata based on given filters.
func (d *DataClient) BinaryDataByFilter(
	ctx context.Context, includeBinary bool, opts *DataByFilterOptions,
) (*BinaryDataByFilterResponse, error) {
	dataReq := pb.DataRequest{}
	var countOnly, includeInternalData bool
	if opts != nil {
		dataReq.Filter = filterToProto(opts.Filter)
		if opts.Limit != 0 {
			dataReq.Limit = uint64(opts.Limit)
		}
		if opts.Last != "" {
			dataReq.Last = opts.Last
		}
		dataReq.SortOrder = orderToProto(opts.SortOrder)
		countOnly = opts.CountOnly
		includeInternalData = opts.IncludeInternalData
	}
	resp, err := d.dataClient.BinaryDataByFilter(ctx, &pb.BinaryDataByFilterRequest{
		DataRequest:         &dataReq,
		IncludeBinary:       includeBinary,
		CountOnly:           countOnly,
		IncludeInternalData: includeInternalData,
	})
	if err != nil {
		return nil, err
	}
	data := make([]*BinaryData, len(resp.Data))
	for i, protoData := range resp.Data {
		binData, err := binaryDataFromProto(protoData)
		if err != nil {
			return nil, err
		}
		data[i] = binData
	}
	return &BinaryDataByFilterResponse{
		BinaryData: data,
		Count:      int(resp.Count),
		Last:       resp.Last,
	}, nil
}

// BinaryDataByIDs queries binary data and metadata based on given IDs.
func (d *DataClient) BinaryDataByIDs(ctx context.Context, binaryIDs []*BinaryID) ([]*BinaryData, error) {
	resp, err := d.dataClient.BinaryDataByIDs(ctx, &pb.BinaryDataByIDsRequest{
		IncludeBinary: true,
		BinaryIds:     binaryIDsToProto(binaryIDs),
	})
	if err != nil {
		return nil, err
	}
	data := make([]*BinaryData, len(resp.Data))
	for i, protoData := range resp.Data {
		binData, err := binaryDataFromProto(protoData)
		if err != nil {
			return nil, err
		}
		data[i] = binData
	}
	return data, nil
}

// DeleteTabularData deletes tabular data older than a number of days, based on the given organization ID.
// It returns the number of tabular datapoints deleted.
func (d *DataClient) DeleteTabularData(ctx context.Context, organizationID string, deleteOlderThanDays int) (int, error) {
	resp, err := d.dataClient.DeleteTabularData(ctx, &pb.DeleteTabularDataRequest{
		OrganizationId:      organizationID,
		DeleteOlderThanDays: uint32(deleteOlderThanDays),
	})
	if err != nil {
		return 0, err
	}
	return int(resp.DeletedCount), nil
}

// DeleteBinaryDataByFilter deletes binary data based on given filters. If filter is empty, delete all data.
// It returns the number of binary datapoints deleted.
func (d *DataClient) DeleteBinaryDataByFilter(ctx context.Context, filter *Filter) (int, error) {
	resp, err := d.dataClient.DeleteBinaryDataByFilter(ctx, &pb.DeleteBinaryDataByFilterRequest{
		Filter:              filterToProto(filter),
		IncludeInternalData: true,
	})
	if err != nil {
		return 0, err
	}
	return int(resp.DeletedCount), nil
}

// DeleteBinaryDataByIDs deletes binary data based on given IDs.
// It returns the number of binary datapoints deleted.
func (d *DataClient) DeleteBinaryDataByIDs(ctx context.Context, binaryIDs []*BinaryID) (int, error) {
	resp, err := d.dataClient.DeleteBinaryDataByIDs(ctx, &pb.DeleteBinaryDataByIDsRequest{
		BinaryIds: binaryIDsToProto(binaryIDs),
	})
	if err != nil {
		return 0, err
	}
	return int(resp.DeletedCount), nil
}

// AddTagsToBinaryDataByIDs adds string tags, unless the tags are already present, to binary data based on given IDs.
func (d *DataClient) AddTagsToBinaryDataByIDs(ctx context.Context, tags []string, binaryIDs []*BinaryID) error {
	_, err := d.dataClient.AddTagsToBinaryDataByIDs(ctx, &pb.AddTagsToBinaryDataByIDsRequest{
		BinaryIds: binaryIDsToProto(binaryIDs),
		Tags:      tags,
	})
	return err
}

// AddTagsToBinaryDataByFilter adds string tags, unless the tags are already present, to binary data based on the given filter.
// If no filter is given, all data will be tagged.
func (d *DataClient) AddTagsToBinaryDataByFilter(ctx context.Context, tags []string, filter *Filter) error {
	_, err := d.dataClient.AddTagsToBinaryDataByFilter(ctx, &pb.AddTagsToBinaryDataByFilterRequest{
		Filter: filterToProto(filter),
		Tags:   tags,
	})
	return err
}

// RemoveTagsFromBinaryDataByIDs removes string tags from binary data based on given IDs.
// It returns the number of binary files which had tags removed.
func (d *DataClient) RemoveTagsFromBinaryDataByIDs(ctx context.Context,
	tags []string, binaryIDs []*BinaryID,
) (int, error) {
	resp, err := d.dataClient.RemoveTagsFromBinaryDataByIDs(ctx, &pb.RemoveTagsFromBinaryDataByIDsRequest{
		BinaryIds: binaryIDsToProto(binaryIDs),
		Tags:      tags,
	})
	if err != nil {
		return 0, err
	}
	return int(resp.DeletedCount), nil
}

// RemoveTagsFromBinaryDataByFilter removes the specified string tags from binary data that match the given filter.
// If no filter is given, all data will be untagged.
// It returns the number of binary files from which tags were removed.
func (d *DataClient) RemoveTagsFromBinaryDataByFilter(ctx context.Context,
	tags []string, filter *Filter,
) (int, error) {
	resp, err := d.dataClient.RemoveTagsFromBinaryDataByFilter(ctx, &pb.RemoveTagsFromBinaryDataByFilterRequest{
		Filter: filterToProto(filter),
		Tags:   tags,
	})
	if err != nil {
		return 0, err
	}
	return int(resp.DeletedCount), nil
}

// TagsByFilter retrieves all unique tags associated with the data that match the specified filter.
// It returns the list of these unique tags. If no filter is given, all data tags are returned.
func (d *DataClient) TagsByFilter(ctx context.Context, filter *Filter) ([]string, error) {
	resp, err := d.dataClient.TagsByFilter(ctx, &pb.TagsByFilterRequest{
		Filter: filterToProto(filter),
	})
	if err != nil {
		return nil, err
	}
	return resp.Tags, nil
}

// AddBoundingBoxToImageByID adds a bounding box to an image with the specified ID,
// using the provided label and position in normalized coordinates.
// All normalized coordinates (xMin, yMin, xMax, yMax) must be float values in the range [0, 1].
func (d *DataClient) AddBoundingBoxToImageByID(
	ctx context.Context,
	binaryID *BinaryID,
	label string,
	xMinNormalized float64,
	yMinNormalized float64,
	xMaxNormalized float64,
	yMaxNormalized float64,
) (string, error) {
	resp, err := d.dataClient.AddBoundingBoxToImageByID(ctx, &pb.AddBoundingBoxToImageByIDRequest{
		BinaryId:       binaryIDToProto(binaryID),
		Label:          label,
		XMinNormalized: xMinNormalized,
		YMinNormalized: yMinNormalized,
		XMaxNormalized: xMaxNormalized,
		YMaxNormalized: yMaxNormalized,
	})
	if err != nil {
		return "", err
	}
	return resp.BboxId, nil
}

// RemoveBoundingBoxFromImageByID removes a bounding box from an image with the given ID.
func (d *DataClient) RemoveBoundingBoxFromImageByID(
	ctx context.Context,
	bboxID string,
	binaryID *BinaryID,
) error {
	_, err := d.dataClient.RemoveBoundingBoxFromImageByID(ctx, &pb.RemoveBoundingBoxFromImageByIDRequest{
		BinaryId: binaryIDToProto(binaryID),
		BboxId:   bboxID,
	})
	return err
}

// BoundingBoxLabelsByFilter retrieves all unique string labels for bounding boxes that match the specified filter.
// It returns a list of these labels. If no filter is given, all labels are returned.
func (d *DataClient) BoundingBoxLabelsByFilter(ctx context.Context, filter *Filter) ([]string, error) {
	resp, err := d.dataClient.BoundingBoxLabelsByFilter(ctx, &pb.BoundingBoxLabelsByFilterRequest{
		Filter: filterToProto(filter),
	})
	if err != nil {
		return nil, err
	}
	return resp.Labels, nil
}

// UpdateBoundingBox updates the bounding box for a given bbox ID for the file represented by the binary ID.
func (d *DataClient) UpdateBoundingBox(ctx context.Context, binaryID *BinaryID, bboxID string, opts *UpdateBoundingBoxOptions) error {
	var label *string
	var xMinNormalized, yMinNormalized, xMaxNormalized, yMaxNormalized *float64
	if opts != nil {
		label = opts.Label
		xMinNormalized = opts.XMinNormalized
		yMinNormalized = opts.YMinNormalized
		xMaxNormalized = opts.XMaxNormalized
		yMaxNormalized = opts.YMaxNormalized
	}

	_, err := d.dataClient.UpdateBoundingBox(ctx, &pb.UpdateBoundingBoxRequest{
		BinaryId:       binaryIDToProto(binaryID),
		BboxId:         bboxID,
		Label:          label,
		XMinNormalized: xMinNormalized,
		YMinNormalized: yMinNormalized,
		XMaxNormalized: xMaxNormalized,
		YMaxNormalized: yMaxNormalized,
	})
	return err
}

// GetDatabaseConnection establishes a connection to a MongoDB Atlas Data Federation instance.
// It returns the hostname endpoint, a URI for connecting to the database via MongoDB clients,
// and a flag indicating whether a database user is configured for the Viam organization.
func (d *DataClient) GetDatabaseConnection(ctx context.Context, organizationID string) (*GetDatabaseConnectionResponse, error) {
	resp, err := d.dataClient.GetDatabaseConnection(ctx, &pb.GetDatabaseConnectionRequest{
		OrganizationId: organizationID,
	})
	if err != nil {
		return nil, err
	}
	return &GetDatabaseConnectionResponse{
		Hostname:        resp.Hostname,
		MongodbURI:      resp.MongodbUri,
		HasDatabaseUser: resp.HasDatabaseUser,
	}, nil
}

// ConfigureDatabaseUser configures a database user for the Viam organization's MongoDB Atlas Data Federation instance.
func (d *DataClient) ConfigureDatabaseUser(
	ctx context.Context,
	organizationID string,
	password string,
) error {
	_, err := d.dataClient.ConfigureDatabaseUser(ctx, &pb.ConfigureDatabaseUserRequest{
		OrganizationId: organizationID,
		Password:       password,
	})
	return err
}

// AddBinaryDataToDatasetByIDs adds the binary data with the given binary IDs to the dataset.
func (d *DataClient) AddBinaryDataToDatasetByIDs(
	ctx context.Context,
	binaryIDs []*BinaryID,
	datasetID string,
) error {
	_, err := d.dataClient.AddBinaryDataToDatasetByIDs(ctx, &pb.AddBinaryDataToDatasetByIDsRequest{
		BinaryIds: binaryIDsToProto(binaryIDs),
		DatasetId: datasetID,
	})
	return err
}

// RemoveBinaryDataFromDatasetByIDs removes the binary data with the given binary IDs from the dataset.
func (d *DataClient) RemoveBinaryDataFromDatasetByIDs(
	ctx context.Context,
	binaryIDs []*BinaryID,
	datasetID string,
) error {
	_, err := d.dataClient.RemoveBinaryDataFromDatasetByIDs(ctx, &pb.RemoveBinaryDataFromDatasetByIDsRequest{
		BinaryIds: binaryIDsToProto(binaryIDs),
		DatasetId: datasetID,
	})
	return err
}

// BinaryDataCaptureUpload uploads the contents and metadata for binary data.
func (d *DataClient) BinaryDataCaptureUpload(
	ctx context.Context,
	binaryData []byte,
	partID string,
	componentType string,
	componentName string,
	methodName string,
	fileExtension string,
	options *BinaryDataCaptureUploadOptions,
) (string, error) {
	var sensorMetadata SensorMetadata
	metadata := UploadMetadata{
		PartID:        partID,
		ComponentType: componentType,
		ComponentName: componentName,
		MethodName:    methodName,
		Type:          DataTypeBinarySensor,
		FileExtension: formatFileExtension(fileExtension),
	}
	if options != nil {
		if options.FileName != nil {
			metadata.FileName = *options.FileName
		}
		if options.MethodParameters != nil {
			metadata.MethodParameters = options.MethodParameters
		}
		if options.Tags != nil {
			metadata.Tags = options.Tags
		}
		if options.DataRequestTimes != nil && len(options.DataRequestTimes) == 2 {
			sensorMetadata = SensorMetadata{
				TimeRequested: options.DataRequestTimes[0],
				TimeReceived:  options.DataRequestTimes[1],
			}
		}
	}
	sensorData := SensorData{
		Metadata: sensorMetadata,
		SDStruct: nil,
		SDBinary: binaryData,
	}

	response, err := d.dataCaptureUpload(ctx, metadata, []SensorData{sensorData})
	if err != nil {
		return "", err
	}
	return response, nil
}

// TabularDataCaptureUpload uploads the contents and metadata for tabular data.
func (d *DataClient) TabularDataCaptureUpload(
	ctx context.Context,
	tabularData []map[string]interface{},
	partID string,
	componentType string,
	componentName string,
	methodName string,
	dataRequestTimes [][2]time.Time,
	options *TabularDataCaptureUploadOptions,
) (string, error) {
	if len(dataRequestTimes) != len(tabularData) {
		return "", errors.New("dataRequestTimes and tabularData lengths must be equal")
	}
	var sensorContents []SensorData
	for i, tabData := range tabularData {
		sensorMetadata := SensorMetadata{}
		dates := dataRequestTimes[i]
		if len(dates) == 2 {
			sensorMetadata.TimeRequested = dates[0]
			sensorMetadata.TimeReceived = dates[1]
		}
		sensorData := SensorData{
			Metadata: sensorMetadata,
			SDStruct: tabData,
			SDBinary: nil,
		}
		sensorContents = append(sensorContents, sensorData)
	}
	metadata := UploadMetadata{
		PartID:        partID,
		ComponentType: componentType,
		ComponentName: componentName,
		MethodName:    methodName,
		Type:          DataTypeTabularSensor,
	}

	if options != nil {
		if options.FileName != nil {
			metadata.FileName = *options.FileName
		}
		if options.MethodParameters != nil {
			metadata.MethodParameters = options.MethodParameters
		}
		if options.FileExtension != nil {
			metadata.FileExtension = formatFileExtension(*options.FileExtension)
		}
		if options.Tags != nil {
			metadata.Tags = options.Tags
		}
	}
	response, err := d.dataCaptureUpload(ctx, metadata, sensorContents)
	if err != nil {
		return "", err
	}
	return response, nil
}

// dataCaptureUpload uploads the metadata and contents for either tabular or binary data,
// and returns the file ID associated with the uploaded data and metadata.
func (d *DataClient) dataCaptureUpload(ctx context.Context, metadata UploadMetadata, sensorContents []SensorData) (string, error) {
	sensorContentsPb, err := sensorContentsToProto(sensorContents)
	if err != nil {
		return "", err
	}
	resp, err := d.dataSyncClient.DataCaptureUpload(ctx, &syncPb.DataCaptureUploadRequest{
		Metadata:       uploadMetadataToProto(metadata),
		SensorContents: sensorContentsPb,
	})
	if err != nil {
		return "", err
	}
	return resp.FileId, nil
}

// StreamingDataCaptureUpload uploads metadata and streaming binary data in chunks.
func (d *DataClient) StreamingDataCaptureUpload(
	ctx context.Context,
	data []byte,
	partID string,
	fileExt string,
	options *StreamingDataCaptureUploadOptions,
) (string, error) {
	uploadMetadata := UploadMetadata{
		PartID:        partID,
		Type:          DataTypeBinarySensor,
		FileExtension: fileExt,
	}
	var sensorMetadata SensorMetadata
	if options != nil {
		if options.ComponentType != nil {
			uploadMetadata.ComponentType = *options.ComponentType
		}
		if options.ComponentName != nil {
			uploadMetadata.ComponentName = *options.ComponentName
		}
		if options.MethodName != nil {
			uploadMetadata.MethodName = *options.MethodName
		}
		if options.FileName != nil {
			uploadMetadata.FileName = *options.FileName
		}
		if options.MethodParameters != nil {
			uploadMetadata.MethodParameters = options.MethodParameters
		}
		if options.Tags != nil {
			uploadMetadata.Tags = options.Tags
		}
		if options.DataRequestTimes != nil && len(options.DataRequestTimes) == 2 {
			sensorMetadata = SensorMetadata{
				TimeRequested: options.DataRequestTimes[0],
				TimeReceived:  options.DataRequestTimes[1],
			}
		}
	}
	uploadMetadataPb := uploadMetadataToProto(uploadMetadata)
	sensorMetadataPb := sensorMetadataToProto(sensorMetadata)
	metadata := &syncPb.DataCaptureUploadMetadata{
		UploadMetadata: uploadMetadataPb,
		SensorMetadata: sensorMetadataPb,
	}
	// establish a streaming connection.
	stream, err := d.dataSyncClient.StreamingDataCaptureUpload(ctx)
	if err != nil {
		return "", err
	}
	// send the metadata as the first packet.
	metaReq := &syncPb.StreamingDataCaptureUploadRequest{
		UploadPacket: &syncPb.StreamingDataCaptureUploadRequest_Metadata{
			Metadata: metadata,
		},
	}
	if err := stream.Send(metaReq); err != nil {
		return "", err
	}

	// send the binary data in chunks.
	for start := 0; start < len(data); start += UploadChunkSize {
		end := start + UploadChunkSize
		if end > len(data) {
			end = len(data)
		}
		dataReq := &syncPb.StreamingDataCaptureUploadRequest{
			UploadPacket: &syncPb.StreamingDataCaptureUploadRequest_Data{
				Data: data[start:end],
			},
		}
		if err := stream.Send(dataReq); err != nil {
			return "", err
		}
	}
	// close the stream and get the response.
	resp, err := stream.CloseAndRecv()
	if err != nil {
		return "", err
	}
	return resp.FileId, nil
}

// FileUploadFromBytes uploads the contents and metadata for binary data such as encoded images or other data represented by bytes
// and returns the file id of the uploaded data.
func (d *DataClient) FileUploadFromBytes(
	ctx context.Context,
	partID string,
	data []byte,
	opts *FileUploadOptions,
) (string, error) {
	metadata := &syncPb.UploadMetadata{
		PartId: partID,
		Type:   syncPb.DataType_DATA_TYPE_FILE,
	}
	if opts != nil {
		if opts.MethodParameters != nil {
			methodParams, err := protoutils.ConvertMapToProtoAny(opts.MethodParameters)
			if err != nil {
				return "", err
			}
			metadata.MethodParameters = methodParams
		}
		if opts.ComponentType != nil {
			metadata.ComponentType = *opts.ComponentType
		}
		if opts.ComponentName != nil {
			metadata.ComponentName = *opts.ComponentName
		}
		if opts.MethodName != nil {
			metadata.MethodName = *opts.MethodName
		}
		if opts.FileName != nil {
			metadata.FileName = *opts.FileName
		}
		if opts.FileExtension != nil {
			metadata.FileExtension = formatFileExtension(*opts.FileExtension)
		}
		if opts.Tags != nil {
			metadata.Tags = opts.Tags
		}
	}
	return d.fileUploadStreamResp(metadata, data)
}

// FileUploadFromPath uploads the contents and metadata for binary data created from a filepath
// and returns the file id of the uploaded data.
func (d *DataClient) FileUploadFromPath(
	ctx context.Context,
	partID string,
	filePath string,
	opts *FileUploadOptions,
) (string, error) {
	metadata := &syncPb.UploadMetadata{
		PartId: partID,
		Type:   syncPb.DataType_DATA_TYPE_FILE,
	}
	if opts != nil {
		if opts.MethodParameters != nil {
			methodParams, err := protoutils.ConvertMapToProtoAny(opts.MethodParameters)
			if err != nil {
				return "", err
			}
			metadata.MethodParameters = methodParams
		}
		if opts.ComponentType != nil {
			metadata.ComponentType = *opts.ComponentType
		}
		if opts.ComponentName != nil {
			metadata.ComponentName = *opts.ComponentName
		}
		if opts.MethodName != nil {
			metadata.MethodName = *opts.MethodName
		}
		if opts.FileExtension != nil {
			metadata.FileExtension = formatFileExtension(*opts.FileExtension)
		}
		if opts.Tags != nil {
			metadata.Tags = opts.Tags
		}
		if opts.FileName != nil {
			metadata.FileName = *opts.FileName
		} else if filePath != "" {
			metadata.FileName = filepath.Base(filePath)
			metadata.FileExtension = filepath.Ext(filePath)
		}
	}

	var data []byte
	// Prepare file data from filepath
	if filePath != "" {
		//nolint:gosec
		fileData, err := os.ReadFile(filePath)
		if err != nil {
			return "", err
		}
		data = fileData
	}
	return d.fileUploadStreamResp(metadata, data)
}

func (d *DataClient) fileUploadStreamResp(metadata *syncPb.UploadMetadata, data []byte) (string, error) {
	// establish a streaming connection.
	stream, err := d.dataSyncClient.FileUpload(context.Background())
	if err != nil {
		return "", err
	}
	// send the metadata as the first packet.
	metaReq := &syncPb.FileUploadRequest{
		UploadPacket: &syncPb.FileUploadRequest_Metadata{
			Metadata: metadata,
		},
	}
	if err := stream.Send(metaReq); err != nil {
		return "", fmt.Errorf("failed to send metadata: %w", err)
	}
	// send file contents in chunks
	for start := 0; start < len(data); start += UploadChunkSize {
		end := start + UploadChunkSize
		if end > len(data) {
			end = len(data)
		}
		dataReq := &syncPb.FileUploadRequest{
			UploadPacket: &syncPb.FileUploadRequest_FileContents{
				FileContents: &syncPb.FileData{
					Data: data[start:end],
				},
			},
		}
		if err := stream.Send(dataReq); err != nil {
			return "", err
		}
	}
	// close stream and get response
	resp, err := stream.CloseAndRecv()
	if err != nil {
		return "", err
	}
	return resp.FileId, nil
}

// CreateDataset makes a new dataset.
func (d *DataClient) CreateDataset(ctx context.Context, name, organizationID string) (string, error) {
	resp, err := d.datasetClient.CreateDataset(ctx, &setPb.CreateDatasetRequest{
		Name:           name,
		OrganizationId: organizationID,
	})
	if err != nil {
		return "", err
	}
	return resp.Id, nil
}

// DeleteDataset deletes an existing dataset.
func (d *DataClient) DeleteDataset(ctx context.Context, id string) error {
	_, err := d.datasetClient.DeleteDataset(ctx, &setPb.DeleteDatasetRequest{
		Id: id,
	})
	return err
}

// RenameDataset modifies the name of an existing dataset.
func (d *DataClient) RenameDataset(ctx context.Context, id, name string) error {
	_, err := d.datasetClient.RenameDataset(ctx, &setPb.RenameDatasetRequest{
		Id:   id,
		Name: name,
	})
	return err
}

// ListDatasetsByOrganizationID lists all of the datasets for an organization.
func (d *DataClient) ListDatasetsByOrganizationID(ctx context.Context, organizationID string) ([]*Dataset, error) {
	resp, err := d.datasetClient.ListDatasetsByOrganizationID(ctx, &setPb.ListDatasetsByOrganizationIDRequest{
		OrganizationId: organizationID,
	})
	if err != nil {
		return nil, err
	}
	var datasets []*Dataset
	for _, dataset := range resp.Datasets {
		datasets = append(datasets, datasetFromProto(dataset))
	}
	return datasets, nil
}

// ListDatasetsByIDs lists all of the datasets specified by the given dataset IDs.
func (d *DataClient) ListDatasetsByIDs(ctx context.Context, ids []string) ([]*Dataset, error) {
	resp, err := d.datasetClient.ListDatasetsByIDs(ctx, &setPb.ListDatasetsByIDsRequest{
		Ids: ids,
	})
	if err != nil {
		return nil, err
	}
	var datasets []*Dataset
	for _, dataset := range resp.Datasets {
		datasets = append(datasets, datasetFromProto(dataset))
	}
	return datasets, nil
}

func boundingBoxFromProto(proto *pb.BoundingBox) *BoundingBox {
	if proto == nil {
		return nil
	}
	return &BoundingBox{
		ID:             proto.Id,
		Label:          proto.Label,
		XMinNormalized: proto.XMinNormalized,
		YMinNormalized: proto.YMinNormalized,
		XMaxNormalized: proto.XMaxNormalized,
		YMaxNormalized: proto.YMaxNormalized,
	}
}

func exportTabularDataResponseFromProto(proto *pb.ExportTabularDataResponse) *ExportTabularDataResponse {
	return &ExportTabularDataResponse{
		OrganizationID:   proto.OrganizationId,
		LocationID:       proto.LocationId,
		RobotID:          proto.RobotId,
		RobotName:        proto.RobotName,
		PartID:           proto.PartId,
		PartName:         proto.PartName,
		ResourceName:     proto.ResourceName,
		ResourceSubtype:  proto.ResourceSubtype,
		MethodName:       proto.MethodName,
		TimeCaptured:     proto.TimeCaptured.AsTime(),
		MethodParameters: proto.MethodParameters.AsMap(),
		Tags:             proto.Tags,
		Payload:          proto.Payload.AsMap(),
	}
}

func annotationsFromProto(proto *pb.Annotations) *Annotations {
	if proto == nil {
		return nil
	}
	bboxes := make([]*BoundingBox, len(proto.Bboxes))
	for i, bboxProto := range proto.Bboxes {
		bboxes[i] = boundingBoxFromProto(bboxProto)
	}
	return &Annotations{
		Bboxes: bboxes,
	}
}

func methodParamsFromProto(proto map[string]*anypb.Any) (map[string]interface{}, error) {
	methodParameters := make(map[string]interface{})
	for key, value := range proto {
		if value == nil {
			methodParameters[key] = nil
		}
		structValue := &structpb.Value{}
		if err := value.UnmarshalTo(structValue); err != nil {
			return nil, err
		}
		methodParameters[key] = structValue.String()
	}
	return methodParameters, nil
}

func captureMetadataFromProto(proto *pb.CaptureMetadata) (*CaptureMetadata, error) {
	if proto == nil {
		return nil, nil
	}
	params, err := methodParamsFromProto(proto.MethodParameters)
	if err != nil {
		return nil, err
	}
	return &CaptureMetadata{
		OrganizationID:   proto.OrganizationId,
		LocationID:       proto.LocationId,
		RobotName:        proto.RobotName,
		RobotID:          proto.RobotId,
		PartName:         proto.PartName,
		PartID:           proto.PartId,
		ComponentType:    proto.ComponentType,
		ComponentName:    proto.ComponentName,
		MethodName:       proto.MethodName,
		MethodParameters: params,
		Tags:             proto.Tags,
		MimeType:         proto.MimeType,
	}, nil
}

func binaryDataFromProto(proto *pb.BinaryData) (*BinaryData, error) {
	if proto == nil {
		return nil, nil
	}
	metadata, err := binaryMetadataFromProto(proto.Metadata)
	if err != nil {
		return nil, err
	}
	return &BinaryData{
		Binary:   proto.Binary,
		Metadata: metadata,
	}, nil
}

func binaryMetadataFromProto(proto *pb.BinaryMetadata) (*BinaryMetadata, error) {
	if proto == nil {
		return nil, nil
	}
	captureMetadata, err := captureMetadataFromProto(proto.CaptureMetadata)
	if err != nil {
		return nil, err
	}
	return &BinaryMetadata{
		ID:              proto.Id,
		CaptureMetadata: *captureMetadata,
		TimeRequested:   proto.TimeRequested.AsTime(),
		TimeReceived:    proto.TimeReceived.AsTime(),
		FileName:        proto.FileName,
		FileExt:         proto.FileExt,
		URI:             proto.Uri,
		Annotations:     annotationsFromProto(proto.Annotations),
		DatasetIDs:      proto.DatasetIds,
	}, nil
}

//nolint:deprecated,staticcheck
func tabularDataFromProto(proto *pb.TabularData, metadata *pb.CaptureMetadata) (*TabularData, error) {
	if proto == nil {
		return nil, nil
	}
	md, err := captureMetadataFromProto(metadata)
	if err != nil {
		return nil, err
	}
	return &TabularData{
		Data:          proto.Data.AsMap(),
		MetadataIndex: int(proto.MetadataIndex),
		Metadata:      md,
		TimeRequested: proto.TimeRequested.AsTime(),
		TimeReceived:  proto.TimeReceived.AsTime(),
	}, nil
}

func binaryIDToProto(binaryID *BinaryID) *pb.BinaryID {
	if binaryID == nil {
		return nil
	}
	return &pb.BinaryID{
		FileId:         binaryID.FileID,
		OrganizationId: binaryID.OrganizationID,
		LocationId:     binaryID.LocationID,
	}
}

func binaryIDsToProto(binaryIDs []*BinaryID) []*pb.BinaryID {
	var protoBinaryIDs []*pb.BinaryID
	for _, binaryID := range binaryIDs {
		protoBinaryIDs = append(protoBinaryIDs, binaryIDToProto(binaryID))
	}
	return protoBinaryIDs
}

func filterToProto(filter *Filter) *pb.Filter {
	if filter == nil {
		return nil
	}
	return &pb.Filter{
		ComponentName:   filter.ComponentName,
		ComponentType:   filter.ComponentType,
		Method:          filter.Method,
		RobotName:       filter.RobotName,
		RobotId:         filter.RobotID,
		PartName:        filter.PartName,
		PartId:          filter.PartID,
		LocationIds:     filter.LocationIDs,
		OrganizationIds: filter.OrganizationIDs,
		MimeType:        filter.MimeType,
		Interval:        captureIntervalToProto(filter.Interval),
		TagsFilter:      tagsFilterToProto(filter.TagsFilter),
		BboxLabels:      filter.BboxLabels,
		DatasetId:       filter.DatasetID,
	}
}

func captureIntervalToProto(interval CaptureInterval) *pb.CaptureInterval {
	return &pb.CaptureInterval{
		Start: timestamppb.New(interval.Start),
		End:   timestamppb.New(interval.End),
	}
}

func tagsFilterToProto(tagsFilter TagsFilter) *pb.TagsFilter {
	return &pb.TagsFilter{
		Type: pb.TagsFilterType(tagsFilter.Type),
		Tags: tagsFilter.Tags,
	}
}

func orderToProto(sortOrder Order) pb.Order {
	switch sortOrder {
	case Ascending:
		return pb.Order_ORDER_ASCENDING
	case Descending:
		return pb.Order_ORDER_DESCENDING
	case Unspecified:
		return pb.Order_ORDER_UNSPECIFIED
	}
	return pb.Order_ORDER_UNSPECIFIED
}

// convertBsonToNative recursively converts BSON datetime objects to Go DateTime and BSON arrays to slices of interface{}.
// For slices and maps of specific types, the best we can do is use interface{} as the container type.
func convertBsonToNative(data interface{}) interface{} {
	switch v := data.(type) {
	case primitive.DateTime:
		return v.Time().UTC()
	case primitive.A: // Handle BSON arrays/slices
		nativeArray := make([]interface{}, len(v))
		for i, item := range v {
			nativeArray[i] = convertBsonToNative(item)
		}
		return nativeArray
	case bson.M: // Handle BSON maps
		convertedMap := make(map[string]interface{})
		for key, value := range v {
			convertedMap[key] = convertBsonToNative(value)
		}
		return convertedMap
	case map[string]interface{}: // Handle Go maps
		for key, value := range v {
			v[key] = convertBsonToNative(value)
		}
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	default:
		return v
	}
}

func datasetFromProto(dataset *setPb.Dataset) *Dataset {
	if dataset == nil {
		return nil
	}
	var timeCreated *time.Time
	if dataset.TimeCreated != nil {
		t := dataset.TimeCreated.AsTime()
		timeCreated = &t
	}
	return &Dataset{
		ID:             dataset.Id,
		Name:           dataset.Name,
		OrganizationID: dataset.OrganizationId,
		TimeCreated:    timeCreated,
	}
}

func uploadMetadataToProto(metadata UploadMetadata) *syncPb.UploadMetadata {
	var methodParams map[string]*anypb.Any
	if metadata.MethodParameters != nil {
		var err error
		methodParams, err = protoutils.ConvertMapToProtoAny(metadata.MethodParameters)
		if err != nil {
			return nil
		}
	}
	return &syncPb.UploadMetadata{
		PartId:           metadata.PartID,
		ComponentType:    metadata.ComponentType,
		ComponentName:    metadata.ComponentName,
		MethodName:       metadata.MethodName,
		Type:             syncPb.DataType(metadata.Type),
		FileName:         metadata.FileName,
		MethodParameters: methodParams,
		FileExtension:    metadata.FileExtension,
		Tags:             metadata.Tags,
	}
}

func annotationsToProto(annotations *Annotations) *pb.Annotations {
	if annotations == nil {
		return nil
	}
	var protoBboxes []*pb.BoundingBox
	for _, bbox := range annotations.Bboxes {
		protoBboxes = append(protoBboxes, &pb.BoundingBox{
			Id:             bbox.ID,
			Label:          bbox.Label,
			XMinNormalized: bbox.XMinNormalized,
			YMinNormalized: bbox.YMinNormalized,
			XMaxNormalized: bbox.XMaxNormalized,
			YMaxNormalized: bbox.YMaxNormalized,
		})
	}
	return &pb.Annotations{
		Bboxes: protoBboxes,
	}
}

func sensorMetadataToProto(metadata SensorMetadata) *syncPb.SensorMetadata {
	return &syncPb.SensorMetadata{
		TimeRequested: timestamppb.New(metadata.TimeRequested),
		TimeReceived:  timestamppb.New(metadata.TimeReceived),
		MimeType:      syncPb.MimeType(metadata.MimeType),
		Annotations:   annotationsToProto(metadata.Annotations),
	}
}

// Ensure only one of SDStruct or SDBinary is set.
func validateSensorData(sensorData SensorData) error {
	if sensorData.SDStruct != nil && len(sensorData.SDBinary) > 0 {
		return errors.New("sensorData cannot have both SDStruct and SDBinary set")
	}
	return nil
}

func sensorDataToProto(sensorData SensorData) (*syncPb.SensorData, error) {
	if err := validateSensorData(sensorData); err != nil {
		return nil, err
	}
	switch {
	case len(sensorData.SDBinary) > 0:
		return &syncPb.SensorData{
			Metadata: sensorMetadataToProto(sensorData.Metadata),
			Data: &syncPb.SensorData_Binary{
				Binary: sensorData.SDBinary,
			},
		}, nil
	case sensorData.SDStruct != nil:
		pbStruct, err := structpb.NewStruct(sensorData.SDStruct)
		if err != nil {
			return nil, err
		}
		return &syncPb.SensorData{
			Metadata: sensorMetadataToProto(sensorData.Metadata),
			Data: &syncPb.SensorData_Struct{
				Struct: pbStruct,
			},
		}, nil
	default:
		return nil, errors.New("sensorData must have either SDStruct or SDBinary set")
	}
}

func sensorContentsToProto(sensorContents []SensorData) ([]*syncPb.SensorData, error) {
	var protoSensorContents []*syncPb.SensorData
	for _, item := range sensorContents {
		protoItem, err := sensorDataToProto(item)
		if err != nil {
			return nil, err // Propagate the error
		}
		protoSensorContents = append(protoSensorContents, protoItem)
	}
	return protoSensorContents, nil
}

func formatFileExtension(fileExt string) string {
	if fileExt == "" {
		return fileExt
	}
	if fileExt[0] == '.' {
		return fileExt
	}
	return "." + fileExt
}
