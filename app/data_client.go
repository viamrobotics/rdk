//go:build !no_cgo

package app

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	pb "go.viam.com/api/app/data/v1"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/protoutils"
	utils "go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

//protobuf to type or type to protobuf (poseinframe to proto or proto to pose in frame)
//define the structs publically and a private function that does the conversion
//come back to "dest" path later to see if we wanna write to a file or not

// viamClient.dataClient.

// i want to wrap NewDataServiceClient define a new dataclient struct and call the wrappers of the functions
// // i want the user to call my dataClient struct w my wrappers and not the proto functions

type DataClient struct {
	client pb.DataServiceClient
}

// Order specifies the order in which data is returned.
type Order int32

const (
	Unspecified Order = 0
	Descending  Order = 1
	Ascending   Order = 2
)

// DataRequest encapsulates the filter for the data, a limit on the maximum results returned,
// a last string associated with the last returned document, and the sorting order by time.
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
	RobotId         string
	PartName        string
	PartId          string
	LocationIds     []string
	OrganizationIds []string
	MimeType        []string
	Interval        CaptureInterval
	TagsFilter      TagsFilter
	BboxLabels      []string
	DatasetId       string
}

// TagsFilter defines the type of filtering and, if applicable, over which tags to perform a logical OR.
type TagsFilterType int32

const (
	TagsFilterTypeUnspecified TagsFilterType = 0
	TagsFilterTypeMatchByOr   TagsFilterType = 1
	TagsFilterTypeTagged      TagsFilterType = 2
	TagsFilterTypeUntagged    TagsFilterType = 3
)

type TagsFilter struct {
	Type TagsFilterType
	Tags []string
}

// CaptureMetadata contains information on the settings used for the data capture.
type CaptureMetadata struct {
	OrganizationId   string
	LocationId       string
	RobotName        string
	RobotId          string
	PartName         string
	PartId           string
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
type TabularData struct {
	Data          map[string]interface{}
	MetadataIndex uint32
	Metadata      CaptureMetadata
	TimeRequested time.Time
	TimeReceived  time.Time
}

type BinaryData struct {
	Binary   []byte
	Metadata BinaryMetadata
}

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

type BinaryID struct {
	FileId         string
	OrganizationId string
	LocationId     string
}
type BoundingBox struct {
	Id             string
	Label          string
	XMinNormalized float64
	YMinNormalized float64
	XMaxNormalized float64
	YMaxNormalized float64
}

type Annotations struct {
	Bboxes []BoundingBox
}

// NewDataClient constructs a new DataClient from connection passed in.
func NewDataClient(
	ctx context.Context,
	channel rpc.ClientConn, //this should just take a channel that the viamClient passes in
	logger logging.Logger,
) (*DataClient, error) {
	d := pb.NewDataServiceClient(channel)
	return &DataClient{
		client: d,
	}, nil
}

func BoundingBoxFromProto(proto *pb.BoundingBox) BoundingBox {
	return BoundingBox{
		Id:             proto.Id,
		Label:          proto.Label,
		XMinNormalized: proto.XMinNormalized,
		YMinNormalized: proto.YMinNormalized,
		XMaxNormalized: proto.XMaxNormalized,
		YMaxNormalized: proto.YMaxNormalized,
	}
}

func AnnotationsFromProto(proto *pb.Annotations) Annotations {
	if proto == nil {
		return Annotations{}
	}
	// Convert each BoundingBox from proto to native type
	bboxes := make([]BoundingBox, len(proto.Bboxes))
	for i, bboxProto := range proto.Bboxes {
		bboxes[i] = BoundingBoxFromProto(bboxProto)
	}
	return Annotations{
		Bboxes: bboxes,
	}
}

func AnnotationsToProto(annotations Annotations) *pb.Annotations {
	var protoBboxes []*pb.BoundingBox
	for _, bbox := range annotations.Bboxes {
		protoBboxes = append(protoBboxes, &pb.BoundingBox{
			Id:             bbox.Id,
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

func CaptureMetadataFromProto(proto *pb.CaptureMetadata) CaptureMetadata {
	if proto == nil {
		return CaptureMetadata{}
	}
	return CaptureMetadata{
		OrganizationId:   proto.OrganizationId,
		LocationId:       proto.LocationId,
		RobotName:        proto.RobotName,
		RobotId:          proto.RobotId,
		PartName:         proto.PartName,
		PartId:           proto.PartId,
		ComponentType:    proto.ComponentType,
		ComponentName:    proto.ComponentName,
		MethodName:       proto.MethodName,
		MethodParameters: methodParamsFromProto(proto.MethodParameters),
		Tags:             proto.Tags,
		MimeType:         proto.MimeType,
	}
}

func CaptureMetadataToProto(metadata CaptureMetadata) *pb.CaptureMetadata {
	methodParms, _ := protoutils.ConvertStringMapToAnyPBMap(metadata.MethodParameters)
	return &pb.CaptureMetadata{
		OrganizationId:   metadata.OrganizationId,
		LocationId:       metadata.LocationId,
		RobotName:        metadata.RobotName,
		RobotId:          metadata.RobotId,
		PartName:         metadata.PartName,
		PartId:           metadata.PartId,
		ComponentType:    metadata.ComponentType,
		ComponentName:    metadata.ComponentName,
		MethodName:       metadata.MethodName,
		MethodParameters: methodParms,
		Tags:             metadata.Tags,
		MimeType:         metadata.MimeType,
	}

}

func BinaryDataFromProto(proto *pb.BinaryData) BinaryData {
	return BinaryData{
		Binary:   proto.Binary,
		Metadata: BinaryMetadataFromProto(proto.Metadata),
	}
}

func BinaryDataToProto(binaryData BinaryData) *pb.BinaryData {
	return &pb.BinaryData{
		Binary:   binaryData.Binary,
		Metadata: BinaryMetadataToProto(binaryData.Metadata),
	}
}

func BinaryMetadataFromProto(proto *pb.BinaryMetadata) BinaryMetadata {
	return BinaryMetadata{
		ID:              proto.Id,
		CaptureMetadata: CaptureMetadataFromProto(proto.CaptureMetadata),
		TimeRequested:   proto.TimeRequested.AsTime(),
		TimeReceived:    proto.TimeReceived.AsTime(),
		FileName:        proto.FileName,
		FileExt:         proto.FileExt,
		URI:             proto.Uri,
		Annotations:     AnnotationsFromProto(proto.Annotations),
		DatasetIDs:      proto.DatasetIds,
	}
}

func BinaryMetadataToProto(binaryMetadata BinaryMetadata) *pb.BinaryMetadata {
	return &pb.BinaryMetadata{
		Id:              binaryMetadata.ID,
		CaptureMetadata: CaptureMetadataToProto(binaryMetadata.CaptureMetadata),
		TimeRequested:   timestamppb.New(binaryMetadata.TimeRequested),
		TimeReceived:    timestamppb.New(binaryMetadata.TimeReceived),
		FileName:        binaryMetadata.FileName,
		FileExt:         binaryMetadata.FileExt,
		Uri:             binaryMetadata.URI,
		Annotations:     AnnotationsToProto(binaryMetadata.Annotations),
		DatasetIds:      binaryMetadata.DatasetIDs,
	}
}

func TabularDataFromProto(proto *pb.TabularData, metadata *pb.CaptureMetadata) TabularData {
	return TabularData{
		Data:          proto.Data.AsMap(),
		MetadataIndex: proto.MetadataIndex,
		Metadata:      CaptureMetadataFromProto(metadata),
		TimeRequested: proto.TimeRequested.AsTime(),
		TimeReceived:  proto.TimeReceived.AsTime(),
	}
}

func TabularDataToProto(tabularData TabularData) *pb.TabularData {
	structData, err := utils.StructToStructPb(tabularData.Data)
	if err != nil {
		return nil
	}
	return &pb.TabularData{
		Data:          structData,
		MetadataIndex: tabularData.MetadataIndex,
		TimeRequested: timestamppb.New(tabularData.TimeRequested),
		TimeReceived:  timestamppb.New(tabularData.TimeReceived),
	}
}

func TabularDataToProtoList(tabularDatas []TabularData) []*pb.TabularData {
	var protoList []*pb.TabularData
	for _, tabularData := range tabularDatas {
		protoData := TabularDataToProto(tabularData)
		if protoData != nil {
			protoList = append(protoList, protoData)
		}
	}
	return protoList
}

func DataRequestToProto(dataRequest DataRequest) (*pb.DataRequest, error) {
	return &pb.DataRequest{
		Filter:    FilterToProto(dataRequest.Filter),
		Limit:     dataRequest.Limit,
		Last:      dataRequest.Last,
		SortOrder: OrderToProto(dataRequest.SortOrder),
	}, nil

}

func BinaryIdToProto(binaryId BinaryID) *pb.BinaryID {
	return &pb.BinaryID{
		FileId:         binaryId.FileId,
		OrganizationId: binaryId.OrganizationId,
		LocationId:     binaryId.LocationId,
	}
}

func BinaryIdsToProto(binaryIds []BinaryID) []*pb.BinaryID {
	var protoBinaryIds []*pb.BinaryID
	for _, binaryId := range binaryIds {
		protoBinaryIds = append(protoBinaryIds, BinaryIdToProto(binaryId))
	}
	return protoBinaryIds
}

func FilterToProto(filter Filter) *pb.Filter {
	return &pb.Filter{
		ComponentName:   filter.ComponentName,
		ComponentType:   filter.ComponentType,
		Method:          filter.Method,
		RobotName:       filter.RobotName,
		RobotId:         filter.RobotId,
		PartName:        filter.PartName,
		PartId:          filter.PartId,
		LocationIds:     filter.LocationIds,
		OrganizationIds: filter.OrganizationIds,
		MimeType:        filter.MimeType,
		Interval:        CaptureIntervalToProto(filter.Interval),
		TagsFilter:      TagsFilterToProto(filter.TagsFilter),
		BboxLabels:      filter.BboxLabels,
		DatasetId:       filter.DatasetId,
	}
}

func CaptureIntervalToProto(interval CaptureInterval) *pb.CaptureInterval {
	return &pb.CaptureInterval{
		Start: timestamppb.New(interval.Start),
		End:   timestamppb.New(interval.End),
	}
}

func TagsFilterToProto(tagsFilter TagsFilter) *pb.TagsFilter {
	return &pb.TagsFilter{
		Type: pb.TagsFilterType(tagsFilter.Type),
		Tags: tagsFilter.Tags,
	}
}

func OrderToProto(sortOrder Order) pb.Order {
	switch sortOrder {
	case Ascending:
		return pb.Order_ORDER_ASCENDING
	case Descending:
		return pb.Order_ORDER_DESCENDING
	default:
		return pb.Order_ORDER_UNSPECIFIED
	}
}

// convertBsonToNative recursively converts BSON types (e.g., DateTime, arrays, maps)
// into their native Go equivalents. This ensures all BSON data types are converted
// to the appropriate Go types like time.Time, slices, and maps.
func convertBsonToNative(data any) any {
	switch v := data.(type) {
	case primitive.DateTime:
		return v.Time().UTC()
	case primitive.A: // Handle BSON arrays/slices
		nativeArray := make([]any, len(v))
		for i, item := range v {
			nativeArray[i] = convertBsonToNative(item)
		}
		return nativeArray
	case bson.M: // Handle BSON maps
		convertedMap := make(map[string]any)
		for key, value := range v {
			convertedMap[key] = convertBsonToNative(value)
		}
		return convertedMap
	case map[string]any: // Handle Go maps
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
func BsonToGo(rawData [][]byte) ([]map[string]any, error) {
	dataObjects := []map[string]any{}
	for _, byteSlice := range rawData {
		// Unmarshal each BSON byte slice into a Go map
		obj := map[string]any{}
		if err := bson.Unmarshal(byteSlice, &obj); err != nil {
			return nil, err
		}
		// Convert the unmarshalled map to native Go types
		convertedObj := convertBsonToNative(obj).(map[string]any)
		dataObjects = append(dataObjects, convertedObj)
	}
	return dataObjects, nil
}

// TabularDataByFilter queries tabular data and metadata based on given filters.
func (d *DataClient) TabularDataByFilter(
	ctx context.Context,
	filter Filter,
	limit uint64,
	last string,
	sortOrder Order,
	countOnly bool,
	includeInternalData bool) ([]TabularData, uint64, string, error) {
	resp, err := d.client.TabularDataByFilter(ctx, &pb.TabularDataByFilterRequest{
		DataRequest: &pb.DataRequest{
			Filter:    FilterToProto(filter),
			Limit:     limit,
			Last:      last,
			SortOrder: OrderToProto(sortOrder)},
		CountOnly:           countOnly,
		IncludeInternalData: includeInternalData,
	})
	if err != nil {
		return nil, 0, "", err
	}
	dataArray := []TabularData{}
	var metadata *pb.CaptureMetadata
	for _, data := range resp.Data {
		if len(resp.Metadata) != 0 && int(data.MetadataIndex) >= len(resp.Metadata) {
			metadata = &pb.CaptureMetadata{}
		} else {
			metadata = resp.Metadata[data.MetadataIndex]

		}
		dataArray = append(dataArray, TabularDataFromProto(data, metadata))
	}
	return dataArray, resp.Count, resp.Last, nil
}

// TabularDataBySQL queries tabular data with a SQL query.
func (d *DataClient) TabularDataBySQL(ctx context.Context, organizationId string, sqlQuery string) ([]map[string]interface{}, error) {
	resp, err := d.client.TabularDataBySQL(ctx, &pb.TabularDataBySQLRequest{OrganizationId: organizationId, SqlQuery: sqlQuery})
	if err != nil {
		return nil, err
	}
	dataObjects, nil := BsonToGo(resp.RawData)
	return dataObjects, nil
}

// TabularDataByMQL queries tabular data with an MQL (MongoDB Query Language) query.
func (d *DataClient) TabularDataByMQL(ctx context.Context, organizationId string, mqlbinary [][]byte) ([]map[string]interface{}, error) {
	resp, err := d.client.TabularDataByMQL(ctx, &pb.TabularDataByMQLRequest{OrganizationId: organizationId, MqlBinary: mqlbinary})
	if err != nil {
		return nil, err
	}
	result, nil := BsonToGo(resp.RawData)
	return result, nil
}

// BinaryDataByFilter queries binary data and metadata based on given filters.
func (d *DataClient) BinaryDataByFilter(
	ctx context.Context,
	filter Filter,
	limit uint64,
	sortOrder Order,
	last string,
	includeBinary bool,
	countOnly bool,
	includeInternalData bool) ([]BinaryData, uint64, string, error) {
	resp, err := d.client.BinaryDataByFilter(ctx, &pb.BinaryDataByFilterRequest{
		DataRequest: &pb.DataRequest{
			Filter:    FilterToProto(filter),
			Limit:     limit,
			Last:      last,
			SortOrder: OrderToProto(sortOrder),
		},
		IncludeBinary:       includeBinary,
		CountOnly:           countOnly,
		IncludeInternalData: includeInternalData,
	})
	if err != nil {
		return nil, 0, "", err
	}
	data := make([]BinaryData, len(resp.Data))
	for i, protoData := range resp.Data {
		data[i] = BinaryDataFromProto(protoData)
	}
	return data, resp.Count, resp.Last, nil

}

// BinaryDataByIDs queries binary data and metadata based on given IDs.
func (d *DataClient) BinaryDataByIDs(ctx context.Context, binaryIds []BinaryID) ([]BinaryData, error) {
	resp, err := d.client.BinaryDataByIDs(ctx, &pb.BinaryDataByIDsRequest{
		IncludeBinary: true,
		BinaryIds:     BinaryIdsToProto(binaryIds),
	})
	if err != nil {
		return nil, err
	}
	data := make([]BinaryData, len(resp.Data))
	for i, protoData := range resp.Data {
		data[i] = BinaryDataFromProto(protoData)
	}
	return data, nil
}

// DeleteTabularData deletes tabular data older than a number of days, based on the given organization ID.
func (d *DataClient) DeleteTabularData(ctx context.Context, organizationId string, deleteOlderThanDays uint32) (uint64, error) {
	resp, err := d.client.DeleteTabularData(ctx, &pb.DeleteTabularDataRequest{
		OrganizationId:      organizationId,
		DeleteOlderThanDays: deleteOlderThanDays,
	})
	if err != nil {
		return 0, err
	}
	return resp.DeletedCount, nil
}

// DeleteBinaryDataByFilter deletes binary data based on given filters.
func (d *DataClient) DeleteBinaryDataByFilter(ctx context.Context, filter Filter) (uint64, error) {
	resp, err := d.client.DeleteBinaryDataByFilter(ctx, &pb.DeleteBinaryDataByFilterRequest{
		Filter:              FilterToProto(filter),
		IncludeInternalData: true,
	})
	if err != nil {
		return 0, err
	}
	return resp.DeletedCount, nil
}

// DeleteBinaryDataByIDs deletes binary data based on given IDs.
func (d *DataClient) DeleteBinaryDataByIDs(ctx context.Context, binaryIds []BinaryID) (uint64, error) {
	resp, err := d.client.DeleteBinaryDataByIDs(ctx, &pb.DeleteBinaryDataByIDsRequest{
		BinaryIds: BinaryIdsToProto(binaryIds),
	})
	if err != nil {
		return 0, err
	}
	return resp.DeletedCount, nil
}

// AddTagsToBinaryDataByIDs adds string tags, unless the tags are already present, to binary data based on given IDs.
func (d *DataClient) AddTagsToBinaryDataByIDs(ctx context.Context, tags []string, binaryIds []BinaryID) error {
	_, err := d.client.AddTagsToBinaryDataByIDs(ctx, &pb.AddTagsToBinaryDataByIDsRequest{BinaryIds: BinaryIdsToProto(binaryIds), Tags: tags})
	if err != nil {
		return err
	}
	return nil
}

// AddTagsToBinaryDataByFilter adds string tags, unless the tags are already present, to binary data based on the given filter.
func (d *DataClient) AddTagsToBinaryDataByFilter(ctx context.Context, tags []string, filter Filter) error {
	_, err := d.client.AddTagsToBinaryDataByFilter(ctx, &pb.AddTagsToBinaryDataByFilterRequest{Filter: FilterToProto(filter), Tags: tags})
	if err != nil {
		return err
	}
	return nil
}

// RemoveTagsToBinaryDataByIDs removes string tags from binary data based on given IDs.
func (d *DataClient) RemoveTagsFromBinaryDataByIDs(ctx context.Context, tags []string, binaryIds []BinaryID) (uint64, error) {
	resp, err := d.client.RemoveTagsFromBinaryDataByIDs(ctx, &pb.RemoveTagsFromBinaryDataByIDsRequest{BinaryIds: BinaryIdsToProto(binaryIds), Tags: tags})
	if err != nil {
		return 0, err
	}
	return resp.DeletedCount, nil
}

// RemoveTagsToBinaryDataByFilter removes string tags from binary data based on the given filter.
func (d *DataClient) RemoveTagsFromBinaryDataByFilter(ctx context.Context, tags []string, filter Filter) (uint64, error) {
	resp, err := d.client.RemoveTagsFromBinaryDataByFilter(ctx, &pb.RemoveTagsFromBinaryDataByFilterRequest{Filter: FilterToProto(filter), Tags: tags})
	if err != nil {
		return 0, err
	}
	return resp.DeletedCount, nil
}

// TagsByFilter gets all unique tags from data based on given filter.
func (d *DataClient) TagsByFilter(ctx context.Context, filter Filter) ([]string, error) {
	resp, err := d.client.TagsByFilter(ctx, &pb.TagsByFilterRequest{Filter: FilterToProto(filter)})
	if err != nil {
		return nil, err
	}
	return resp.Tags, nil
}

// AddBoundingBoxToImageByID adds a bounding box to an image with the given ID.
func (d *DataClient) AddBoundingBoxToImageByID(
	ctx context.Context,
	binaryId BinaryID,
	label string,
	xMinNormalized float64,
	yMinNormalized float64,
	xMaxNormalized float64,
	yMaxNormalized float64) (string, error) {
	resp, err := d.client.AddBoundingBoxToImageByID(ctx, &pb.AddBoundingBoxToImageByIDRequest{BinaryId: BinaryIdToProto(binaryId), Label: label, XMinNormalized: xMinNormalized, YMinNormalized: yMinNormalized, XMaxNormalized: xMaxNormalized, YMaxNormalized: yMaxNormalized})
	if err != nil {
		return "", err
	}
	return resp.BboxId, nil

}

// RemoveBoundingBoxFromImageByID removes a bounding box from an image with the given ID.
func (d *DataClient) RemoveBoundingBoxFromImageByID(ctx context.Context, bboxId string, binaryId BinaryID) error {
	_, err := d.client.RemoveBoundingBoxFromImageByID(ctx, &pb.RemoveBoundingBoxFromImageByIDRequest{BinaryId: BinaryIdToProto(binaryId), BboxId: bboxId})
	if err != nil {
		return err
	}
	return nil
}

// BoundingBoxLabelsByFilter gets all string labels for bounding boxes from data based on given filter.
func (d *DataClient) BoundingBoxLabelsByFilter(ctx context.Context, filter Filter) ([]string, error) {
	resp, err := d.client.BoundingBoxLabelsByFilter(ctx, &pb.BoundingBoxLabelsByFilterRequest{Filter: FilterToProto(filter)})
	if err != nil {
		return nil, err
	}
	return resp.Labels, nil
}

// UpdateBoundingBox updates the bounding box associated with a given binary ID and bounding box ID.
func (d *DataClient) UpdateBoundingBox(ctx context.Context,
	binaryId BinaryID,
	bboxId string,
	label *string, // optional
	xMinNormalized *float64, // optional
	yMinNormalized *float64, // optional
	xMaxNormalized *float64, // optional
	yMaxNormalized *float64, // optional
) error {
	_, err := d.client.UpdateBoundingBox(ctx, &pb.UpdateBoundingBoxRequest{BinaryId: BinaryIdToProto(binaryId), BboxId: bboxId, Label: label, XMinNormalized: xMinNormalized, YMinNormalized: yMinNormalized, XMaxNormalized: xMaxNormalized, YMaxNormalized: yMaxNormalized})
	if err != nil {
		return err
	}
	return nil
}

// GetDatabaseConnection gets a connection to access a MongoDB Atlas Data Federation instance. It
// returns the hostname of the federated database.
func (d *DataClient) GetDatabaseConnection(ctx context.Context, organizationId string) (string, error) {
	resp, err := d.client.GetDatabaseConnection(ctx, &pb.GetDatabaseConnectionRequest{OrganizationId: organizationId})
	if err != nil {
		return "", err
	}
	return resp.Hostname, nil
}

// ConfigureDatabaseUser configures a database user for the Viam organization's MongoDB Atlas Data
// Federation instance. It can also be used to reset the password of the existing database user.
func (d *DataClient) ConfigureDatabaseUser(ctx context.Context, organizationId string, password string) error {
	_, err := d.client.ConfigureDatabaseUser(ctx, &pb.ConfigureDatabaseUserRequest{OrganizationId: organizationId, Password: password})
	if err != nil {
		return err
	}
	return nil
}

// AddBinaryDataToDatasetByIDs adds the binary data with the given binary IDs to the dataset.
func (d *DataClient) AddBinaryDataToDatasetByIDs(ctx context.Context, binaryIds []BinaryID, datasetId string) error {
	_, err := d.client.AddBinaryDataToDatasetByIDs(ctx, &pb.AddBinaryDataToDatasetByIDsRequest{BinaryIds: BinaryIdsToProto(binaryIds), DatasetId: datasetId})
	if err != nil {
		return err
	}
	return nil
}

// RemoveBinaryDataFromDatasetByIDs removes the binary data with the given binary IDs from the dataset.
func (d *DataClient) RemoveBinaryDataFromDatasetByIDs(ctx context.Context, binaryIds []BinaryID, datasetId string) error {
	_, err := d.client.RemoveBinaryDataFromDatasetByIDs(ctx, &pb.RemoveBinaryDataFromDatasetByIDsRequest{BinaryIds: BinaryIdsToProto(binaryIds), DatasetId: datasetId})
	if err != nil {
		return err
	}
	return nil
}
