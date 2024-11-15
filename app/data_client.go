// Package data contains a gRPC based data client.
package data

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	pb "go.viam.com/api/app/data/v1"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/protoutils"
)

// Client implements the DataServiceClient interface.
type Client struct {
	client pb.DataServiceClient
	logger logging.Logger
}

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
	Limit     uint64
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
	MethodParameters map[string]string
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
	MetadataIndex uint32
	Metadata      CaptureMetadata
	TimeRequested time.Time
	TimeReceived  time.Time
}

// BinaryData contains data and metadata associated with binary data.
type BinaryData struct {
	Binary   []byte
	Metadata BinaryMetadata
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
	Annotations     Annotations
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
	Bboxes []BoundingBox
}

// TabularDataReturn represents the result of a TabularDataByFilter query.
// It contains the retrieved tabular data and associated metadata,
// the total number of entries retrieved (Count), and the ID of the last returned page (Last).
type TabularDataReturn struct {
	TabularData []TabularData
	Count       uint64
	Last        string
}

// BinaryDataReturn represents the result of a BinaryDataByFilter query.
// It contains the retrieved binary data and associated metadata,
// the total number of entries retrieved (Count), and the ID of the last returned page (Last).
type BinaryDataReturn struct {
	BinaryData []BinaryData
	Count      uint64
	Last       string
}

// DatabaseConnReturn represents the response returned by GetDatabaseConnection.
// It contains the hostname endpoint, a URI for connecting to the MongoDB Atlas Data Federation instance,
// and a flag indicating whether a database user is configured for the Viam organization.
type DatabaseConnReturn struct {
	Hostname        string
	MongodbURI      string
	HasDatabaseUser bool
}

// NewDataClient constructs a new DataClient using the connection passed in by the viamClient and the provided logger.
func NewDataClient(
	channel rpc.ClientConn,
	logger logging.Logger,
) (*Client, error) {
	d := pb.NewDataServiceClient(channel)
	return &Client{
		client: d,
		logger: logger,
	}, nil
}

func boundingBoxFromProto(proto *pb.BoundingBox) BoundingBox {
	return BoundingBox{
		ID:             proto.Id,
		Label:          proto.Label,
		XMinNormalized: proto.XMinNormalized,
		YMinNormalized: proto.YMinNormalized,
		XMaxNormalized: proto.XMaxNormalized,
		YMaxNormalized: proto.YMaxNormalized,
	}
}

func annotationsFromProto(proto *pb.Annotations) Annotations {
	if proto == nil {
		return Annotations{}
	}
	bboxes := make([]BoundingBox, len(proto.Bboxes))
	for i, bboxProto := range proto.Bboxes {
		bboxes[i] = boundingBoxFromProto(bboxProto)
	}
	return Annotations{
		Bboxes: bboxes,
	}
}

func methodParamsFromProto(proto map[string]*anypb.Any) map[string]string {
	methodParameters := make(map[string]string)
	for key, value := range proto {
		structValue := &structpb.Value{}
		if err := value.UnmarshalTo(structValue); err != nil {
			return nil
		}
		methodParameters[key] = structValue.String()
	}
	return methodParameters
}

func captureMetadataFromProto(proto *pb.CaptureMetadata) CaptureMetadata {
	if proto == nil {
		return CaptureMetadata{}
	}
	return CaptureMetadata{
		OrganizationID:   proto.OrganizationId,
		LocationID:       proto.LocationId,
		RobotName:        proto.RobotName,
		RobotID:          proto.RobotId,
		PartName:         proto.PartName,
		PartID:           proto.PartId,
		ComponentType:    proto.ComponentType,
		ComponentName:    proto.ComponentName,
		MethodName:       proto.MethodName,
		MethodParameters: methodParamsFromProto(proto.MethodParameters),
		Tags:             proto.Tags,
		MimeType:         proto.MimeType,
	}
}

func captureMetadataToProto(metadata CaptureMetadata) *pb.CaptureMetadata {
	methodParms, err := protoutils.ConvertStringMapToAnyPBMap(metadata.MethodParameters)
	if err != nil {
		return nil
	}
	return &pb.CaptureMetadata{
		OrganizationId:   metadata.OrganizationID,
		LocationId:       metadata.LocationID,
		RobotName:        metadata.RobotName,
		RobotId:          metadata.RobotID,
		PartName:         metadata.PartName,
		PartId:           metadata.PartID,
		ComponentType:    metadata.ComponentType,
		ComponentName:    metadata.ComponentName,
		MethodName:       metadata.MethodName,
		MethodParameters: methodParms,
		Tags:             metadata.Tags,
		MimeType:         metadata.MimeType,
	}
}

func binaryDataFromProto(proto *pb.BinaryData) BinaryData {
	return BinaryData{
		Binary:   proto.Binary,
		Metadata: binaryMetadataFromProto(proto.Metadata),
	}
}

func binaryMetadataFromProto(proto *pb.BinaryMetadata) BinaryMetadata {
	return BinaryMetadata{
		ID:              proto.Id,
		CaptureMetadata: captureMetadataFromProto(proto.CaptureMetadata),
		TimeRequested:   proto.TimeRequested.AsTime(),
		TimeReceived:    proto.TimeReceived.AsTime(),
		FileName:        proto.FileName,
		FileExt:         proto.FileExt,
		URI:             proto.Uri,
		Annotations:     annotationsFromProto(proto.Annotations),
		DatasetIDs:      proto.DatasetIds,
	}
}

func tabularDataFromProto(proto *pb.TabularData, metadata *pb.CaptureMetadata) TabularData {
	return TabularData{
		Data:          proto.Data.AsMap(),
		MetadataIndex: proto.MetadataIndex,
		Metadata:      captureMetadataFromProto(metadata),
		TimeRequested: proto.TimeRequested.AsTime(),
		TimeReceived:  proto.TimeReceived.AsTime(),
	}
}

func binaryIDToProto(binaryID BinaryID) *pb.BinaryID {
	return &pb.BinaryID{
		FileId:         binaryID.FileID,
		OrganizationId: binaryID.OrganizationID,
		LocationId:     binaryID.LocationID,
	}
}

func binaryIDsToProto(binaryIDs []BinaryID) []*pb.BinaryID {
	var protoBinaryIDs []*pb.BinaryID
	for _, binaryID := range binaryIDs {
		protoBinaryIDs = append(protoBinaryIDs, binaryIDToProto(binaryID))
	}
	return protoBinaryIDs
}

func filterToProto(filter Filter) *pb.Filter {
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
func (d *Client) TabularDataByFilter(
	ctx context.Context,
	filter Filter,
	limit uint64,
	last string,
	sortOrder Order,
	countOnly bool,
	includeInternalData bool,
) (TabularDataReturn, error) {
	resp, err := d.client.TabularDataByFilter(ctx, &pb.TabularDataByFilterRequest{
		DataRequest: &pb.DataRequest{
			Filter:    filterToProto(filter),
			Limit:     limit,
			Last:      last,
			SortOrder: orderToProto(sortOrder),
		},
		CountOnly:           countOnly,
		IncludeInternalData: includeInternalData,
	})
	if err != nil {
		return TabularDataReturn{}, err
	}
	// TabularData contains tabular data and associated metadata
	dataArray := []TabularData{}
	var metadata *pb.CaptureMetadata
	for _, data := range resp.Data {
		if len(resp.Metadata) > 0 && int(data.MetadataIndex) < len(resp.Metadata) {
			metadata = resp.Metadata[data.MetadataIndex]
		} else {
			// Use an empty CaptureMetadata as a fallback
			metadata = &pb.CaptureMetadata{}
		}
		dataArray = append(dataArray, tabularDataFromProto(data, metadata))
	}

	return TabularDataReturn{
		TabularData: dataArray,
		Count:       resp.Count,
		Last:        resp.Last,
	}, nil
}

// TabularDataBySQL queries tabular data with a SQL query.
func (d *Client) TabularDataBySQL(ctx context.Context, organizationID, sqlQuery string) ([]map[string]interface{}, error) {
	resp, err := d.client.TabularDataBySQL(ctx, &pb.TabularDataBySQLRequest{
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

// TabularDataByMQL queries tabular data with an MQL (MongoDB Query Language) query.
func (d *Client) TabularDataByMQL(ctx context.Context, organizationID string, mqlbinary [][]byte) ([]map[string]interface{}, error) {
	resp, err := d.client.TabularDataByMQL(ctx, &pb.TabularDataByMQLRequest{
		OrganizationId: organizationID,
		MqlBinary:      mqlbinary,
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

// BinaryDataByFilter queries binary data and metadata based on given filters.
func (d *Client) BinaryDataByFilter(
	ctx context.Context,
	filter Filter,
	limit uint64,
	sortOrder Order,
	last string,
	includeBinary bool,
	countOnly bool,
	includeInternalData bool,
) (BinaryDataReturn, error) {
	resp, err := d.client.BinaryDataByFilter(ctx, &pb.BinaryDataByFilterRequest{
		DataRequest: &pb.DataRequest{
			Filter:    filterToProto(filter),
			Limit:     limit,
			Last:      last,
			SortOrder: orderToProto(sortOrder),
		},
		IncludeBinary:       includeBinary,
		CountOnly:           countOnly,
		IncludeInternalData: includeInternalData,
	})
	if err != nil {
		return BinaryDataReturn{}, err
	}
	data := make([]BinaryData, len(resp.Data))
	for i, protoData := range resp.Data {
		data[i] = binaryDataFromProto(protoData)
	}
	return BinaryDataReturn{
		BinaryData: data,
		Count:      resp.Count,
		Last:       resp.Last,
	}, nil
}

// BinaryDataByIDs queries binary data and metadata based on given IDs.
func (d *Client) BinaryDataByIDs(ctx context.Context, binaryIDs []BinaryID) ([]BinaryData, error) {
	resp, err := d.client.BinaryDataByIDs(ctx, &pb.BinaryDataByIDsRequest{
		IncludeBinary: true,
		BinaryIds:     binaryIDsToProto(binaryIDs),
	})
	if err != nil {
		return nil, err
	}
	data := make([]BinaryData, len(resp.Data))
	for i, protoData := range resp.Data {
		data[i] = binaryDataFromProto(protoData)
	}
	return data, nil
}

// DeleteTabularData deletes tabular data older than a number of days, based on the given organization ID.
// It returns the number of tabular datapoints deleted.
func (d *Client) DeleteTabularData(ctx context.Context, organizationID string, deleteOlderThanDays uint32) (uint64, error) {
	resp, err := d.client.DeleteTabularData(ctx, &pb.DeleteTabularDataRequest{
		OrganizationId:      organizationID,
		DeleteOlderThanDays: deleteOlderThanDays,
	})
	if err != nil {
		return 0, err
	}
	return resp.DeletedCount, nil
}

// DeleteBinaryDataByFilter deletes binary data based on given filters.
// It returns the number of binary datapoints deleted.
func (d *Client) DeleteBinaryDataByFilter(ctx context.Context, filter Filter) (uint64, error) {
	resp, err := d.client.DeleteBinaryDataByFilter(ctx, &pb.DeleteBinaryDataByFilterRequest{
		Filter:              filterToProto(filter),
		IncludeInternalData: true,
	})
	if err != nil {
		return 0, err
	}
	return resp.DeletedCount, nil
}

// DeleteBinaryDataByIDs deletes binary data based on given IDs.
// It returns the number of binary datapoints deleted.
func (d *Client) DeleteBinaryDataByIDs(ctx context.Context, binaryIDs []BinaryID) (uint64, error) {
	resp, err := d.client.DeleteBinaryDataByIDs(ctx, &pb.DeleteBinaryDataByIDsRequest{
		BinaryIds: binaryIDsToProto(binaryIDs),
	})
	if err != nil {
		return 0, err
	}
	return resp.DeletedCount, nil
}

// AddTagsToBinaryDataByIDs adds string tags, unless the tags are already present, to binary data based on given IDs.
func (d *Client) AddTagsToBinaryDataByIDs(ctx context.Context, tags []string, binaryIDs []BinaryID) error {
	_, err := d.client.AddTagsToBinaryDataByIDs(ctx, &pb.AddTagsToBinaryDataByIDsRequest{
		BinaryIds: binaryIDsToProto(binaryIDs),
		Tags:      tags,
	})
	return err
}

// AddTagsToBinaryDataByFilter adds string tags, unless the tags are already present, to binary data based on the given filter.
func (d *Client) AddTagsToBinaryDataByFilter(ctx context.Context, tags []string, filter Filter) error {
	_, err := d.client.AddTagsToBinaryDataByFilter(ctx, &pb.AddTagsToBinaryDataByFilterRequest{
		Filter: filterToProto(filter),
		Tags:   tags,
	})
	return err
}

// RemoveTagsFromBinaryDataByIDs removes string tags from binary data based on given IDs.
// It returns the number of binary files which had tags removed.
func (d *Client) RemoveTagsFromBinaryDataByIDs(ctx context.Context,
	tags []string, binaryIDs []BinaryID,
) (uint64, error) {
	resp, err := d.client.RemoveTagsFromBinaryDataByIDs(ctx, &pb.RemoveTagsFromBinaryDataByIDsRequest{
		BinaryIds: binaryIDsToProto(binaryIDs),
		Tags:      tags,
	})
	if err != nil {
		return 0, err
	}
	return resp.DeletedCount, nil
}

// RemoveTagsFromBinaryDataByFilter removes the specified string tags from binary data that match the given filter.
// It returns the number of binary files from which tags were removed.
func (d *Client) RemoveTagsFromBinaryDataByFilter(ctx context.Context,
	tags []string, filter Filter,
) (uint64, error) {
	resp, err := d.client.RemoveTagsFromBinaryDataByFilter(ctx, &pb.RemoveTagsFromBinaryDataByFilterRequest{
		Filter: filterToProto(filter),
		Tags:   tags,
	})
	if err != nil {
		return 0, err
	}
	return resp.DeletedCount, nil
}

// TagsByFilter retrieves all unique tags associated with the data that match the specified filter.
// It returns the list of these unique tags.
func (d *Client) TagsByFilter(ctx context.Context, filter Filter) ([]string, error) {
	resp, err := d.client.TagsByFilter(ctx, &pb.TagsByFilterRequest{
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
func (d *Client) AddBoundingBoxToImageByID(
	ctx context.Context,
	binaryID BinaryID,
	label string,
	xMinNormalized float64,
	yMinNormalized float64,
	xMaxNormalized float64,
	yMaxNormalized float64,
) (string, error) {
	resp, err := d.client.AddBoundingBoxToImageByID(ctx, &pb.AddBoundingBoxToImageByIDRequest{
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
func (d *Client) RemoveBoundingBoxFromImageByID(
	ctx context.Context,
	bboxID string,
	binaryID BinaryID,
) error {
	_, err := d.client.RemoveBoundingBoxFromImageByID(ctx, &pb.RemoveBoundingBoxFromImageByIDRequest{
		BinaryId: binaryIDToProto(binaryID),
		BboxId:   bboxID,
	})
	return err
}

// BoundingBoxLabelsByFilter retrieves all unique string labels for bounding boxes that match the specified filter.
// It returns a list of these labels.
func (d *Client) BoundingBoxLabelsByFilter(ctx context.Context, filter Filter) ([]string, error) {
	resp, err := d.client.BoundingBoxLabelsByFilter(ctx, &pb.BoundingBoxLabelsByFilterRequest{
		Filter: filterToProto(filter),
	})
	if err != nil {
		return nil, err
	}
	return resp.Labels, nil
}

// UpdateBoundingBox updates the bounding box associated with a given binary ID and bounding box ID.
func (d *Client) UpdateBoundingBox(ctx context.Context,
	binaryID BinaryID,
	bboxID string,
	label *string, // optional
	xMinNormalized *float64, // optional
	yMinNormalized *float64, // optional
	xMaxNormalized *float64, // optional
	yMaxNormalized *float64, // optional
) error {
	_, err := d.client.UpdateBoundingBox(ctx, &pb.UpdateBoundingBoxRequest{
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
func (d *Client) GetDatabaseConnection(ctx context.Context, organizationID string) (DatabaseConnReturn, error) {
	resp, err := d.client.GetDatabaseConnection(ctx, &pb.GetDatabaseConnectionRequest{
		OrganizationId: organizationID,
	})
	if err != nil {
		return DatabaseConnReturn{}, err
	}
	return DatabaseConnReturn{
		Hostname:        resp.Hostname,
		MongodbURI:      resp.MongodbUri,
		HasDatabaseUser: resp.HasDatabaseUser,
	}, nil
}

// ConfigureDatabaseUser configures a database user for the Viam organization's MongoDB Atlas Data Federation instance.
func (d *Client) ConfigureDatabaseUser(
	ctx context.Context,
	organizationID string,
	password string,
) error {
	_, err := d.client.ConfigureDatabaseUser(ctx, &pb.ConfigureDatabaseUserRequest{
		OrganizationId: organizationID,
		Password:       password,
	})
	return err
}

// AddBinaryDataToDatasetByIDs adds the binary data with the given binary IDs to the dataset.
func (d *Client) AddBinaryDataToDatasetByIDs(
	ctx context.Context,
	binaryIDs []BinaryID,
	datasetID string,
) error {
	_, err := d.client.AddBinaryDataToDatasetByIDs(ctx, &pb.AddBinaryDataToDatasetByIDsRequest{
		BinaryIds: binaryIDsToProto(binaryIDs),
		DatasetId: datasetID,
	})
	return err
}

// RemoveBinaryDataFromDatasetByIDs removes the binary data with the given binary IDs from the dataset.
func (d *Client) RemoveBinaryDataFromDatasetByIDs(
	ctx context.Context,
	binaryIDs []BinaryID,
	datasetID string,
) error {
	_, err := d.client.RemoveBinaryDataFromDatasetByIDs(ctx, &pb.RemoveBinaryDataFromDatasetByIDsRequest{
		BinaryIds: binaryIDsToProto(binaryIDs),
		DatasetId: datasetID,
	})
	return err
}
