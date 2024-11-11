//go:build !no_cgo

package app

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
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
	Tags []string
	MimeType string
}
type BinaryID struct {
	FileId         string
	OrganizationId string
	LocationId     string
}
type BoundingBox struct {
	id             string
	label          string
	xMinNormalized float64
	yMinNormalized float64
	xMaxNormalized float64
	yMaxNormalized float64
}
type DataRequest struct {
	Filter    Filter
	Limit     uint64
	Last      string
	SortOrder Order
}
type Annotations struct {
	bboxes []BoundingBox
}
type TabularData struct {
	Data map[string]interface{}
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
	ID string
	CaptureMetadata CaptureMetadata
	TimeRequested   time.Time
	TimeReceived    time.Time
	FileName        string
	FileExt         string
	URI             string
	Annotations     Annotations
	DatasetIDs []string
}
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

type TagsFilter struct {
	Type int32
	Tags []string
}
type CaptureInterval struct {
	Start time.Time
	End   time.Time
}

func BoundingBoxFromProto(proto *pb.BoundingBox) BoundingBox {
	return BoundingBox{
		id:             proto.Id,
		label:          proto.Label,
		xMinNormalized: proto.XMinNormalized,
		yMinNormalized: proto.YMinNormalized,
		xMaxNormalized: proto.XMaxNormalized,
		yMaxNormalized: proto.YMaxNormalized,
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
		bboxes: bboxes,
	}
}
func AnnotationsToProto(annotations Annotations) *pb.Annotations {
	var protoBboxes []*pb.BoundingBox
	for _, bbox := range annotations.bboxes {
		protoBboxes = append(protoBboxes, &pb.BoundingBox{
			Id:             bbox.id,
			Label:          bbox.label,
			XMinNormalized: bbox.xMinNormalized,
			YMinNormalized: bbox.yMinNormalized,
			XMaxNormalized: bbox.xMaxNormalized,
			YMaxNormalized: bbox.yMaxNormalized,
		})
	}
	return &pb.Annotations{
		Bboxes: protoBboxes,
	}
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

// returns tabular data and the associated metadata
func TabularDataFromProto(proto *pb.TabularData, metadata *pb.CaptureMetadata) TabularData {
	fmt.Printf("this is proto in tabulardatafrom proto: %+v\n", metadata)
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
func TabularDataBsonHelper(rawData [][]byte) ([]map[string]interface{}, error) {
	dataObjects := []map[string]interface{}{}
	for _, byteSlice := range rawData {
		fmt.Printf("the byteslice is: %+v\n", byteSlice)
		obj := map[string]interface{}{}
		bson.Unmarshal(byteSlice, &obj)
		for key, value := range obj {
			if v, ok := value.(int32); ok {
				obj[key] = int(v)
			}
		}
		dataObjects = append(dataObjects, obj)
	}
	return dataObjects, nil
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

type Order int32

const (
	Unspecified Order = 0
	Descending  Order = 1
	Ascending   Order = 2
)

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

type DataClient struct {
	//do we want this to be a public interface that defines the functions but does not include client and private details?
	//would not include client and private details
	client pb.DataServiceClient
}

// (private) dataClient implements DataServiceClient. **do we want this?
// type dataClient interface {
// 	// actual hold implementations of functions - how would the NewDataClient function work if we had private and public functions?
// 	// client      pb.DataServiceClient
// }

// NewDataClient constructs a new DataClient from connection passed in.
func NewDataClient(
	ctx context.Context,
	channel rpc.ClientConn, //this should just take a channek that the viamClient passes in
	logger logging.Logger,
) (*DataClient, error) {
	d := pb.NewDataServiceClient(channel)
	return &DataClient{
		client: d,
	}, nil
}

// TabularDataByFilter queries tabular data and metadata based on given filters.
// returns []TabularData, uint64, string, and error:  returns multiple things containing the following: List[TabularData]: The tabular data, int: The count (number of entries), str: The last-returned page ID.
func (d *DataClient) TabularDataByFilter(
	ctx context.Context,
	filter Filter,
	limit uint64, //optional
	last string, //optional
	sortOrder Order, //optional
	countOnly bool,
	includeInternalData bool) ([]TabularData, uint64, string, error) {
	if limit == 0 {
		limit = 50
	}
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
			metadata = &pb.CaptureMetadata{} // Create new metadata if index is out of range
		} else {
			metadata = resp.Metadata[data.MetadataIndex]

		}
		dataArray = append(dataArray, TabularDataFromProto(data, metadata))

	}
	return dataArray, resp.Count, resp.Last, nil
}

// returns an array of data objects
// interface{} is a special type in Go that represents any type.
// so map[string]interface{} is a map (aka a dictionary) where the keys are strings and the values are of any type
// a list of maps --> like we had in python a list of dictionaries
func (d *DataClient) TabularDataBySQL(ctx context.Context, organizationId string, sqlQuery string) ([]map[string]interface{}, error) {
	resp, err := d.client.TabularDataBySQL(ctx, &pb.TabularDataBySQLRequest{OrganizationId: organizationId, SqlQuery: sqlQuery})
	if err != nil {
		return nil, err
	}
	dataObjects, nil := TabularDataBsonHelper(resp.RawData)
	return dataObjects, nil
}

func (d *DataClient) TabularDataByMQL(ctx context.Context, organizationId string, mqlbinary [][]byte) ([]map[string]interface{}, error) {
	//need to double check this mqlbinary type***??
	resp, err := d.client.TabularDataByMQL(ctx, &pb.TabularDataByMQLRequest{OrganizationId: organizationId, MqlBinary: mqlbinary})
	if err != nil {
		return nil, err
	}

	// var result []map[string]interface{}
	// for _, bsonBytes := range resp.RawData {
	// 	var decodedData map[string]interface{}
	// 	err := bson.Unmarshal(bsonBytes, &decodedData)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("error decoding BSON: %v", err)
	// 	}
	// 	fmt.Printf("this is decoded data: %+v\n", decodedData)
	// 	result = append(result, decodedData)

	// }
	result, nil := TabularDataBsonHelper(resp.RawData)
	fmt.Printf("this is result: %+v\n", result)
	return result, nil

	// dataObjects := make([]map[string]interface{}, len(resp.RawData))
	// for i, rawData := range resp.RawData {
	// 	var obj map[string]interface{}
	// 	if err := bson.Unmarshal(rawData, &obj); err != nil {
	// 		fmt.Printf("(func) unmarshalling error %d: %v", i, err)
	// 		return nil, err
	// 	}
	// 	dataObjects[i] = obj
	// }

	// fmt.Println("printing Deserialized dataObjects here", dataObjects)
}

func (d *DataClient) BinaryDataByFilter(
	//dest string??
	ctx context.Context,
	filter Filter,
	limit uint64,
	sortOrder Order,
	last string,
	includeBinary bool,
	countOnly bool,
	includeInternalData bool) ([]BinaryData, uint64, string, error) {
	fmt.Println("client.BinaryDataByFilter was called")
	if limit == 0 {
		limit = 50
	}
	resp, err := d.client.BinaryDataByFilter(ctx, &pb.BinaryDataByFilterRequest{
		DataRequest: &pb.DataRequest{ //need to do checks to make sure it fits the constraints
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
	// Convert protobuf BinaryData to Go-native BinaryData
	data := make([]BinaryData, len(resp.Data))
	for i, protoData := range resp.Data {
		data[i] = BinaryDataFromProto(protoData)
	}
	// return resp.Data, resp.Count, resp.Last, nil
	return data, resp.Count, resp.Last, nil

}

// do i need to be including error as a return type for all of these?
func (d *DataClient) BinaryDataByIDs(ctx context.Context, binaryIds []BinaryID) ([]BinaryData, error) {
	resp, err := d.client.BinaryDataByIDs(ctx, &pb.BinaryDataByIDsRequest{
		IncludeBinary: true,
		BinaryIds:     BinaryIdsToProto(binaryIds),
	})
	if err != nil {
		return nil, err
	}
	// Convert protobuf BinaryData to Go-native BinaryData
	data := make([]BinaryData, len(resp.Data))
	for i, protoData := range resp.Data {
		data[i] = BinaryDataFromProto(protoData)
	}
	return data, nil //the return type of this is: var data []BinaryData --> is that okay??? , do we only want go native types does this count?
}
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

func (d *DataClient) DeleteBinaryDataByFilter(ctx context.Context, filter Filter) (uint64, error) {
	//should probably do some sort of check that filter isn't empty otherwise i need to do something
	// if filter == nil {
	// 	filter = &pb.Filter{}
	// }
	resp, err := d.client.DeleteBinaryDataByFilter(ctx, &pb.DeleteBinaryDataByFilterRequest{
		Filter:              FilterToProto(filter),
		IncludeInternalData: true,
	})
	if err != nil {
		return 0, err
	}
	return resp.DeletedCount, nil
}
func (d *DataClient) DeleteBinaryDataByIDs(ctx context.Context, binaryIds []BinaryID) (uint64, error) {
	resp, err := d.client.DeleteBinaryDataByIDs(ctx, &pb.DeleteBinaryDataByIDsRequest{
		BinaryIds: BinaryIdsToProto(binaryIds),
	})
	if err != nil {
		return 0, err
	}
	return resp.DeletedCount, nil
}
func (d *DataClient) AddTagsToBinaryDataByIDs(ctx context.Context, tags []string, binaryIds []BinaryID) error {
	_, err := d.client.AddTagsToBinaryDataByIDs(ctx, &pb.AddTagsToBinaryDataByIDsRequest{BinaryIds: BinaryIdsToProto(binaryIds), Tags: tags})
	if err != nil {
		return err
	}
	return nil
}
func (d *DataClient) AddTagsToBinaryDataByFilter(ctx context.Context, tags []string, filter Filter) error {
	_, err := d.client.AddTagsToBinaryDataByFilter(ctx, &pb.AddTagsToBinaryDataByFilterRequest{Filter: FilterToProto(filter), Tags: tags})
	if err != nil {
		return err
	}
	return nil
}
func (d *DataClient) RemoveTagsFromBinaryDataByIDs(ctx context.Context, tags []string, binaryIds []BinaryID) (uint64, error) {
	resp, err := d.client.RemoveTagsFromBinaryDataByIDs(ctx, &pb.RemoveTagsFromBinaryDataByIDsRequest{BinaryIds: BinaryIdsToProto(binaryIds), Tags: tags})
	if err != nil {
		return 0, err
	}
	return resp.DeletedCount, nil
}
func (d *DataClient) RemoveTagsFromBinaryDataByFilter(ctx context.Context, tags []string, filter Filter) (uint64, error) {
	// if filter == nil {
	// 	filter = &pb.Filter{}
	// }
	resp, err := d.client.RemoveTagsFromBinaryDataByFilter(ctx, &pb.RemoveTagsFromBinaryDataByFilterRequest{Filter: FilterToProto(filter), Tags: tags})
	if err != nil {
		return 0, err
	}
	return resp.DeletedCount, nil
}
func (d *DataClient) TagsByFilter(ctx context.Context, filter Filter) ([]string, error) {
	// if filter == nil {
	// 	filter = &pb.Filter{}
	// }
	resp, err := d.client.TagsByFilter(ctx, &pb.TagsByFilterRequest{Filter: FilterToProto(filter)})
	if err != nil {
		return nil, err
	}
	return resp.Tags, nil
}
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
func (d *DataClient) RemoveBoundingBoxFromImageByID(ctx context.Context, bboxId string, binaryId BinaryID) error {
	_, err := d.client.RemoveBoundingBoxFromImageByID(ctx, &pb.RemoveBoundingBoxFromImageByIDRequest{BinaryId: BinaryIdToProto(binaryId), BboxId: bboxId})
	if err != nil {
		return err
	}
	return nil
}
func (d *DataClient) BoundingBoxLabelsByFilter(ctx context.Context, filter Filter) ([]string, error) {
	resp, err := d.client.BoundingBoxLabelsByFilter(ctx, &pb.BoundingBoxLabelsByFilterRequest{Filter: FilterToProto(filter)})
	if err != nil {
		return nil, err
	}
	return resp.Labels, nil
}

// ***python and typescript did not implement this one!!!
func (d *DataClient) UpdateBoundingBox(ctx context.Context,
	binaryId BinaryID,
	bboxId string,
	label string,
	xMinNormalized float64,
	yMinNormalized float64,
	xMaxNormalized float64,
	yMaxNormalized float64) error {

	_, err := d.client.UpdateBoundingBox(ctx, &pb.UpdateBoundingBoxRequest{BinaryId: BinaryIdToProto(binaryId), BboxId: bboxId, Label: &label, XMinNormalized: &xMinNormalized, YMinNormalized: &yMinNormalized, XMaxNormalized: &xMaxNormalized, YMaxNormalized: &yMaxNormalized})
	if err != nil {
		return err
	}
	return nil
}

// do we want to return more than a hostname??
func (d *DataClient) GetDatabaseConnection(ctx context.Context, organizationId string) (string, error) {
	resp, err := d.client.GetDatabaseConnection(ctx, &pb.GetDatabaseConnectionRequest{OrganizationId: organizationId})
	if err != nil {
		return "", err
	}
	return resp.Hostname, nil
}

func (d *DataClient) ConfigureDatabaseUser(ctx context.Context, organizationId string, password string) error {
	_, err := d.client.ConfigureDatabaseUser(ctx, &pb.ConfigureDatabaseUserRequest{OrganizationId: organizationId, Password: password})
	if err != nil {
		return err
	}
	return nil
}
func (d *DataClient) AddBinaryDataToDatasetByIDs(ctx context.Context, binaryIds []BinaryID, datasetId string) error {
	_, err := d.client.AddBinaryDataToDatasetByIDs(ctx, &pb.AddBinaryDataToDatasetByIDsRequest{BinaryIds: BinaryIdsToProto(binaryIds), DatasetId: datasetId})
	if err != nil {
		return err
	}
	return nil
}
func (d *DataClient) RemoveBinaryDataFromDatasetByIDs(ctx context.Context, binaryIds []BinaryID, datasetId string) error {
	_, err := d.client.RemoveBinaryDataFromDatasetByIDs(ctx, &pb.RemoveBinaryDataFromDatasetByIDsRequest{BinaryIds: BinaryIdsToProto(binaryIds), DatasetId: datasetId})
	if err != nil {
		return err
	}
	return nil
}
