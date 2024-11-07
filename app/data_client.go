//go:build !no_cgo

package app

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	pb "go.viam.com/api/app/data/v1"
	"go.viam.com/rdk/logging"
	"go.viam.com/utils/rpc"
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
	organization_id   string
	location_id2      string
	robot_name        string
	robot_id          string
	part_name         string
	part_id           string
	component_type    string
	component_name    string
	method_name       string
	method_parameters map[string]interface{}
	//^^ supposed to be: map<string, google.protobuf.Any> method_parameters = 11;
	tags []string
	//^^ repeated string tags = 12;
	mime_type string
	//^^ string mime_type = 13;
}
type BinaryID struct {
	FileId         string
	OrganizationId string
	LocationId     string
}
type BoundingBox struct {
	id               string
	label            string
	x_min_normalized float64 //should be double but no doubles in go
	y_min_normalized float64
	x_max_normalized float64
	y_max_normalized float64
}

// Annotations are data annotations used for machine learning.
type Annotations struct {
	//supposed to be repeated bounding boxes
	bboxes []BoundingBox
}
type TabularData struct {
	Data map[string]interface{}
	// Metadata *pb.CaptureMetadata //idk why i put a star here -- if we aren't returning it is it okay?
	Metadata      CaptureMetadata //idk why i put a star here
	TimeRequested time.Time
	TimeReceived  time.Time
}
type BinaryData struct {
	Binary   []byte
	Metadata BinaryMetadata
}

// can the return type be a struct called BinaryMetadata that I made up???
type BinaryMetadata struct {
	ID string
	//CaptureMetadata *pb.CaptureMetadata
	CaptureMetadata CaptureMetadata
	TimeRequested   time.Time
	TimeReceived    time.Time
	FileName        string
	FileExt         string
	URI             string
	Annotations     Annotations
	//Annotations *pb.Annotations
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
	Interval        CaptureInterval //asterix or no??
	TagsFilter      TagsFilter      //asterix or no??
	BboxLabels      []string
	DatasetId       string
}

// func (f Filter) IsEmpty() bool {
// 	return reflect.DeepEqual(f, Filter{})
// }

//notes for above::
// type TagsFilter struct {
// 	state         protoimpl.MessageState
// 	sizeCache     protoimpl.SizeCache
// 	unknownFields protoimpl.UnknownFields

// 	Type TagsFilterType `protobuf:"varint,1,opt,name=type,proto3,enum=viam.app.data.v1.TagsFilterType" json:"type,omitempty"`
// 	// Tags are used to match documents if `type` is UNSPECIFIED or MATCH_BY_OR.
// 	Tags []string `protobuf:"bytes,2,rep,name=tags,proto3" json:"tags,omitempty"`
// }

//type TagsFilterType int32

type TagsFilter struct {
	Type int32 //type TagsFilterType int32
	Tags []string
}
type CaptureInterval struct {
	Start time.Time
	End   time.Time
}

func BoundingBoxFromProto(proto *pb.BoundingBox) BoundingBox {
	return BoundingBox{
		id:               proto.Id,
		label:            proto.Label,
		x_min_normalized: proto.XMinNormalized, // cast if i want int, or use float64 for precision
		y_min_normalized: proto.YMinNormalized,
		x_max_normalized: proto.XMaxNormalized,
		y_max_normalized: proto.YMaxNormalized,
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

func CaptureMetadataFromProto(proto *pb.CaptureMetadata) CaptureMetadata {
	if proto == nil {
		return CaptureMetadata{}
	}
	// Convert method parameters from protobuf to native map
	methodParameters := make(map[string]interface{})
	// Convert MethodParameters map[string]*anypb.Any to map[string]interface{}
	for key, value := range proto.MethodParameters {
		structValue := &structpb.Value{}
		if err := value.UnmarshalTo(structValue); err != nil {
			return CaptureMetadata{} // return error??
		}
		methodParameters[key] = structValue.AsInterface()
	}
	return CaptureMetadata{
		organization_id:   proto.OrganizationId,
		location_id2:      proto.LocationId,
		robot_name:        proto.RobotName,
		robot_id:          proto.RobotId,
		part_name:         proto.PartName,
		part_id:           proto.PartId,
		component_type:    proto.ComponentType,
		component_name:    proto.ComponentName,
		method_name:       proto.MethodName,
		method_parameters: methodParameters,
		tags:              proto.Tags, // repeated string
		mime_type:         proto.MimeType,
	}
}
func BinaryDataFromProto(proto *pb.BinaryData) BinaryData {
	return BinaryData{
		Binary:   proto.Binary,
		Metadata: BinaryMetadataFromProto(proto.Metadata),
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

// PropertiesToProtoResponse takes a map of features to struct and converts it
// to a GetPropertiesResponse.
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
		Interval:        CaptureIntervalToProto(filter.Interval), //check this ??
		TagsFilter:      TagsFilterToProto(filter.TagsFilter),    //check this ??
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

// shadow type for Order
type Order int32

// do i even need these below???
const (
	Unspecified Order = 0
	Descending  Order = 1
	Ascending   Order = 2
)

// Map SortOrder to corresponding values in pb.Order
func OrderToProto(sortOrder Order) pb.Order {
	switch sortOrder {
	case Ascending:
		return pb.Order_ORDER_ASCENDING
	case Descending:
		return pb.Order_ORDER_DESCENDING
	default:
		return pb.Order_ORDER_UNSPECIFIED // default or error handling
	}
}

// LocationIds     []string
// OrganizationIds []string
// MimeType        []string
// Interval        *CaptureInterval
// TagsFilter      *TagsFilter
// BboxLabels      []string

type DataClient struct {
	//do we want this to be a public interface that defines the functions but does not include client and private details?
	//would not include client and private details
	client pb.DataServiceClient
}

// (private) dataClient implements DataServiceClient. **do we want this?
type dataClient interface {
	// actual hold implementations of functions - how would the NewDataClient function work if we had private and public functions?
	// client      pb.DataServiceClient
}

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
	//include dest?
	ctx context.Context,
	// filter *pb.Filter, //optional - no filter implies all tabular data
	filter Filter,
	limit uint64, //optional - max defaults to 50 if unspecified
	last string, //optional
	sortOrder Order, //optional
	countOnly bool,
	includeInternalData bool) ([]TabularData, uint64, string, error) {
	// initialize limit if it's unspecified (zero value)
	if limit == 0 {
		limit = 50
	}

	// // ensure filter is not nil to represent a query for "all data"
	// if filter.IsEmpty(){
	// 	filter = Filter{} //i think if it is empty than it just implies that ALL tabular data??
	// }
	resp, err := d.client.TabularDataByFilter(ctx, &pb.TabularDataByFilterRequest{
		DataRequest: &pb.DataRequest{ //need to do checks to make sure it fits the constraints
			Filter:    FilterToProto(filter),
			Limit:     limit,
			Last:      last,
			SortOrder: OrderToProto(sortOrder),
		},
		CountOnly:           countOnly,
		IncludeInternalData: includeInternalData,
	})
	//do we want this?
	if err != nil {
		return nil, 0, "", err
	}
	//need to undo the repeated tabularData in resp.Data and return it
	dataArray := make([]TabularData, len(resp.Data))
	for i, data := range resp.Data {
		mdIndex := data.MetadataIndex
		// var metadata *pb.CaptureMetadata
		var metadata CaptureMetadata
		//is this check necessary??
		// Ensure the metadata index is within bounds
		if len(resp.Metadata) != 0 && int(mdIndex) < len(resp.Metadata) {
			metadata = CaptureMetadataFromProto(resp.Metadata[mdIndex])
		}
		//creating a list of tabularData
		dataArray[i] = TabularData{
			Data:          data.Data.AsMap(),
			Metadata:      metadata,
			TimeRequested: data.TimeRequested.AsTime(),
			TimeReceived:  data.TimeReceived.AsTime(),
		}
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
	// Initialize a an array of maps to hold the data objects (in python we had list of dicts)
	dataObjects := make([]map[string]interface{}, len(resp.RawData))
	// Loop over each BSON byte array in the response and unmarshal directly into the dataObjects slice
	for i, rawData := range resp.RawData {
		obj := make(map[string]interface{})
		bson.Unmarshal(rawData, &obj)
		//do we want an error message for bson.Unmarshal...?
		dataObjects[i] = obj
	}
	return dataObjects, nil
}

func (d *DataClient) TabularDataByMQL(ctx context.Context, organizationId string, mqlbinary [][]byte) ([]map[string]interface{}, error) {
	//need to double check this mqlbinary type***??
	resp, err := d.client.TabularDataByMQL(ctx, &pb.TabularDataByMQLRequest{OrganizationId: organizationId, MqlBinary: mqlbinary})
	if err != nil {
		return nil, err
	}
	// Debugging output to verify RawData content
	fmt.Printf("Response RawData: %v\n", resp.RawData)
	//loop thru rawData
	//for each rawData byte slice you will need to unmarshall it into map[string]interface
	//then add each unmarshalled map to a list and return it
	dataObjects := make([]map[string]interface{}, len(resp.RawData))
	for i, rawData := range resp.RawData {
		var obj map[string]interface{}
		if err := bson.Unmarshal(rawData, &obj); err != nil {
			fmt.Printf("(func) unmarshalling error %d: %v", i, err)
			return nil, err
		}
		dataObjects[i] = obj
	}
	// dataObjects := make([]map[string]interface{}, len(resp.RawData))
	// for i, rawData := range resp.RawData {
	// 	obj := make(map[string]interface{})
	// 	bson.Unmarshal(rawData, &obj)
	// 	dataObjects[i] = obj
	// }
	// Unmarshal all raw data at once as an array of maps
	// var dataObjects []map[string]interface{}
	// for _, rawData := range resp.RawData {
	// 	var singleData []map[string]interface{} // This should match your expected structure
	// 	if err := bson.Unmarshal(rawData, &singleData); err != nil {
	// 		return nil, err
	// 	}
	// 	dataObjects = append(dataObjects, singleData...)
	// }
	// Unmarshal each raw data entry as a separate map
	// dataObjects := make([]map[string]interface{}, len(resp.RawData))
	// for i, rawData := range resp.RawData {
	// 	var obj map[string]interface{}
	// 	if err := bson.Unmarshal(rawData, &obj); err != nil {
	// 		fmt.Printf("Unmarshalling error at index %d: %v\n", i, err)
	// 		return nil, err
	// 	}
	// 	dataObjects[i] = obj
	// }
	fmt.Println("printing Deserialized dataObjects here", dataObjects)
	return dataObjects, nil
}

func (d *DataClient) BinaryDataByFilter(
	//dest string??
	ctx context.Context,
	filter Filter,
	limit uint64,
	last string,
	sortOrder Order,
	includeBinary bool,
	countOnly bool,
	// includeInternalData bool) ([]*pb.BinaryData, uint64, string, error) {
	includeInternalData bool) ([]BinaryData, uint64, string, error) {
	// initialize limit if it's unspecified (zero value)
	if limit == 0 {
		limit = 50
	}
	// ensure filter is not nil to represent a query for "all data"
	// if filter == nil {
	// 	filter = &pb.Filter{}
	// }
	resp, err := d.client.BinaryDataByFilter(ctx, &pb.BinaryDataByFilterRequest{
		DataRequest: &pb.DataRequest{ //need to do checks to make sure it fits the constraints
			Filter:    FilterToProto(filter),
			Limit:     limit,
			Last:      last,
			SortOrder: OrderToProto(sortOrder),
		},
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
	// return resp.Data, nil
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
	// if filter == nil {
	// 	filter = &pb.Filter{}
	// }
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
	// if filter == nil {
	// 	filter = &pb.Filter{}
	// }
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
