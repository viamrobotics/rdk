//go:build !no_cgo

package app

import (
	"context"
	"fmt"
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

// func convertBsonToNative(data interface{}) interface{} {
// 	switch v := data.(type) {
// 	case primitive.DateTime:
// 		// Convert BSON DateTime to Go time.Time
// 		return v.Time().UTC()
// 	// case primitive.ObjectID:
// 	// 	// Convert BSON ObjectId to string (or another suitable representation)
// 	// 	return v.Hex()
// 	case int32, int64, float32, float64, bool, string:
// 		return v

// 	case bson.A:
// 		// If it's a BSON array, convert each item inside it
// 		nativeArray := make([]interface{}, len(v))
// 		for i, item := range v {
// 			nativeArray[i] = convertArrayToNative([]interface{}(item))
// 		}
// 		// return nativeArray
// 		// return convertArrayToNative([]interface{}(data))

// 	case bson.M:
// 		convertedMap := make(map[string]interface{})
// 		for key, value := range v {
// 			convertedMap[key] = convertBsonToNative(value)
// 		}
// 		return convertedMap

// 	// case map[string]interface{}:
// 	// 	// If it's a BSON document (map), convert each value recursively
// 	// 	for k, val := range v {
// 	// 		v[k] = convertBsonToNative(val)
// 	// 	}
// 	// 	return v

// 	default:
// 		// For all other types, return the value as is
// 		return v
// 	}

// }
//RINMITVE.A W SLICE OF INTERFctx
//PRIMIVEDATAA WITH DTEIMTc
//dont do better than usinug any/interface for the containers of slices and maps

func convertBsonToNative(data any) any {
	// Check if the input is a slice (list) or a map, and process accordingly
	fmt.Printf("this is data at the TOP %T, value: %+v\n", data, data)
	switch v := data.(type) {
	case primitive.DateTime:
		return v.Time().UTC()
	case primitive.A: //arrays/slices
		nativeArray := make([]any, len(v))
		for i, item := range v {
			nativeArray[i] = convertBsonToNative(item)
		}
		return nativeArray
	case bson.M: //maps 
		convertedMap := make(map[string]any)
		for key, value := range v {
			convertedMap[key] = convertBsonToNative(value)
		}
		return convertedMap
	case map[string]any:
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

// case []map[string]interface{}:
// 	convertedArray := make([]map[string]interface{}, len(data))
// 	for i, item := range data {
// 		fmt.Printf("this is value inside new []map[string]interface{}: %T, value: %+v\n", item, item)
// 		convertedArray[i] = convertBsonToNative(item).(map[string]interface{})
// 	}
// 	return convertedArray
// case []interface{}:
// 	if len(data) > 0 {
// 		if _, ok := data[0].(map[string]interface{}); ok {
// 			// It’s a slice of maps: process accordingly
// 			convertedArray := make([]map[string]interface{}, len(data))
// 			for i, item := range data {
// 				convertedArray[i] = convertBsonToNative(item).(map[string]interface{})
// 			}
// 			return convertedArray
// 		}
// 	}
// 	fmt.Printf("this is data inside []interface{}: %T, value: %+v\n", data, data)
// 	return convertArrayToNative(data)
// case bson.A: // this is the same as []interface{}
// 	fmt.Printf("this is inside bson.A: %T, value: %+v\n", data, data)
// 	if len(data) > 0 {
// 		if _, ok := data[0].(map[string]interface{}); ok {
// 			// It’s a slice of maps: process accordingly
// 			convertedArray := make([]map[string]interface{}, len(data))
// 			for i, item := range data {
// 				convertedArray[i] = convertBsonToNative(item).(map[string]interface{})
// 			}
// 			return convertedArray
// 		}
// 	}
// 	return convertArrayToNative([]interface{}(data))
// case map[string]interface{}:
// 	convertedMap := make(map[string]interface{})
// 	for key, value := range data {
// 		fmt.Printf("this is value inside converted map inside map[string]interface{}: %T, value: %+v\n", value, value)
// 		convertedMap[key] = convertBsonToNative(value)
// 	}
// 	fmt.Printf("this is converted map inside map[string]interface{}: %T, value: %+v\n", convertedMap, convertedMap)
// 	return convertedMap

// case bson.M:
// 	convertedMap := make(map[string]interface{})
// 	for key, value := range data {
// 		convertedMap[key] = convertBsonToNative(value)
// 	}
// 	fmt.Printf("this is converted map inside bson.M: %T, value: %+v\n", convertedMap, convertedMap)
// 	return convertedMap
// default:
// 	return convertSingleValue(data)
// }
// }

// Helper function to handle homogeneous arrays
// func convertArrayToNative(array []interface{}) interface{} {
// 	if len(array) == 0 {
// 		return array
// 	}
// 	// check if all elements are of type int
// 	allInts := true
// 	for _, item := range array {
// 		if _, ok := item.(int32); !ok {
// 			allInts = false
// 			break
// 		}
// 	}
// 	// convert to []int if all elements are integers
// 	if allInts {
// 		intArray := make([]int, len(array))
// 		for i, item := range array {
// 			intArray[i] = int(item.(int32)) // assuming BSON uses int32 for integers
// 		}
// 		return intArray
// 	}
// 	// if not all integers then return []interface{} with recursive conversion
// 	nativeArray := make([]interface{}, len(array))
// 	for i, item := range array {
// 		nativeArray[i] = convertBsonToNative(item)
// 	}
// 	return nativeArray
// }

// // helper function to handle single BSON or primitive values
// func convertSingleValue(data interface{}) interface{} {
// 	switch v := data.(type) {
// 	case primitive.DateTime:
// 		return v.Time().UTC()
// 	case primitive.ObjectID:
// 		return v.Hex()
// 	case int32, int64, float32, float64, bool, string:
// 		return v
// 	default:
// 		return v
// 	}
// }

// func convertBsonToNative(data interface{}) interface{} {
// 	// Check if the input is a slice (list) or a map, and process accordingly
// 	fmt.Printf("this is data type: %T, value: %+v\n", data, data)

// 	switch data := data.(type) {
// 	case []interface{}: // If data is a generic list
// 		nativeArray := make([]interface{}, len(data))
// 		for i, item := range data {
// 			nativeArray[i] = convertBsonToNative(item) // Recursively convert each item
// 		}
// 		fmt.Printf("this is nativeArray in interface{}: %T, value: %+v\n", nativeArray, nativeArray)
// 		return nativeArray

// 	case bson.A: // If data is a BSON array
// 	//**we would need to change interface{} to be []int if we wanted that to work for us. and then our convertToBsonNative type would also have to not be data interface{}
// 		nativeArray := make([]interface{}, len(data))
// 		for i, item := range data {
// 			nativeArray[i] = convertBsonToNative(item) // Recursively convert each item
// 		}
// 		fmt.Printf("this is nativeArray in bsonA: %T, value: %+v\n", nativeArray, nativeArray)
// 		return nativeArray

// 	case map[string]interface{}: // If data is a generic map
// 		convertedMap := make(map[string]interface{})
// 		for key, value := range data {
// 			convertedMap[key] = convertBsonToNative(value) // Recursively convert each value
// 		}
// 		return convertedMap

// 	case bson.M: // If data is a BSON map
// 		convertedMap := make(map[string]interface{})
// 		for key, value := range data {
// 			convertedMap[key] = convertBsonToNative(value) // Recursively convert each value
// 		}
// 		return convertedMap

// 	default:
// 		// If data is not a list or map, handle BSON specific types and primitives
// 		return convertSingleValue(data)
// 	}
// }

func TabularDataBsonHelper(rawData [][]byte) ([]map[string]interface{}, error) {
	dataObjects := []map[string]interface{}{}
	// fmt.Printf("this is rawData type: %T, value: %+v\n", rawData, rawData)
	for _, byteSlice := range rawData {
		// fmt.Printf("the byteslice is: %+v\n", byteSlice)
		obj := map[string]interface{}{}
		// bson.Unmarshal(byteSlice, &obj) //we are getting mongodb datetime objects, and al monogdb types
		if err := bson.Unmarshal(byteSlice, &obj); err != nil {
			return nil, err
		}
		// for key, value := range obj {
		// 	if v, ok := value.(int32); ok {
		// 		obj[key] = int(v)
		// 	}
		// }
		fmt.Printf("this is the object before conversion: %T, value: %+v\n", obj, obj)
		convertedObj := convertBsonToNative(obj).(map[string]interface{})
		// fmt.Printf("this is object type: %T, value: %+v\n", convertedObj, convertedObj)
		dataObjects = append(dataObjects, convertedObj)
	}
	fmt.Printf("this is dataObjectd type: %T, value: %+v\n", dataObjects, dataObjects)
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

//NOTES:
// func convertBsonTypes(obj map[string]interface{}) map[string]interface{} {
// 	for key, value := range obj {
// 		switch v := value.(type) {
// 		case string:
// 			//no conversion needed
// 			return value
// 		case float64, int32, int64:
// 			//no conversion needed
// 			return v
// 		case bool:
// 			//no conversion needed
// 			return v
// 		case nil:
// 			// Null - Represent as nil in Go
// 			return nil
// 		case bson.A: // BSON array
// 			result := []interface{}{}
// 			for _, item := range v {
// 				result = append(result, convertBsonTypes(item))
// 			}
// 			return result
// 		case bson.M: // BSON object (map)
// 			result := map[string]interface{}{}
// 			for key, val := range v {
// 				result[key] = convertBsonTypes(val)
// 			}
// 			return result
// 		case time.Time:
// 			// Datetime - Convert BSON datetime to Go time.Time
// 			return v
// 		default:
// 			// Return other types as-is
// 			return v

//			// 	obj[key] = convertBsonTypes(v)
//			case map[string]interface{}:
//				// recursively convert nested maps
//				obj[key] = convertBsonTypes(v)
//			}
//		}
//		return obj
//	}
