//go:build !no_cgo

package app

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	pb "go.viam.com/api/app/data/v1"
	"go.viam.com/rdk/logging"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/structpb"
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
	filter *pb.Filter, //optional - no filter implies all tabular data
	limit uint64, //optional - max defaults to 50 if unspecified
	last string, //optional
	sortOrder pb.Order, //optional
	countOnly bool,
	includeInternalData bool) ([]TabularData, uint64, string, error) {
	// initialize limit if it's unspecified (zero value)
	if limit == 0 {
		limit = 50
	}
	// ensure filter is not nil to represent a query for "all data"
	if filter == nil {
		filter = &pb.Filter{}
	}
	resp, err := d.client.TabularDataByFilter(ctx, &pb.TabularDataByFilterRequest{
		DataRequest: &pb.DataRequest{ //need to do checks to make sure it fits the constraints
			Filter:    filter,
			Limit:     limit,
			Last:      last,
			SortOrder: sortOrder,
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
	dataObjects := make([]map[string]interface{}, len(resp.RawData))
	for i, rawData := range resp.RawData {
		obj := make(map[string]interface{})
		bson.Unmarshal(rawData, &obj)
		dataObjects[i] = obj
	}
	return dataObjects, nil
}

func (d *DataClient) BinaryDataByFilter(
	//dest string??
	ctx context.Context,
	filter *pb.Filter,
	limit uint64,
	last string,
	sortOrder pb.Order,
	includeBinary bool,
	countOnly bool,
	// includeInternalData bool) ([]*pb.BinaryData, uint64, string, error) {
	includeInternalData bool) ([]BinaryData, uint64, string, error) {
	// initialize limit if it's unspecified (zero value)
	if limit == 0 {
		limit = 50
	}
	// ensure filter is not nil to represent a query for "all data"
	if filter == nil {
		filter = &pb.Filter{}
	}
	resp, err := d.client.BinaryDataByFilter(ctx, &pb.BinaryDataByFilterRequest{
		DataRequest: &pb.DataRequest{ //need to do checks to make sure it fits the constraints
			Filter:    filter,
			Limit:     limit,
			Last:      last,
			SortOrder: sortOrder,
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
func (d *DataClient) BinaryDataByIDs(ctx context.Context, binaryIds []*pb.BinaryID) ([]BinaryData, error) {
	resp, err := d.client.BinaryDataByIDs(ctx, &pb.BinaryDataByIDsRequest{
		IncludeBinary: true,
		BinaryIds:     binaryIds,
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
	return data, nil
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

func (d *DataClient) DeleteBinaryDataByFilter(ctx context.Context, filter *pb.Filter) (uint64, error) {
	//should probably do some sort of check that filter isn't empty otherwise i need to do something
	if filter == nil {
		filter = &pb.Filter{}
	}
	resp, err := d.client.DeleteBinaryDataByFilter(ctx, &pb.DeleteBinaryDataByFilterRequest{
		Filter:              filter,
		IncludeInternalData: true,
	})
	if err != nil {
		return 0, err
	}
	return resp.DeletedCount, nil
}
func (d *DataClient) DeleteBinaryDataByIDs(ctx context.Context, binaryIds []*pb.BinaryID) (uint64, error) {
	resp, err := d.client.DeleteBinaryDataByIDs(ctx, &pb.DeleteBinaryDataByIDsRequest{
		BinaryIds: binaryIds,
	})
	if err != nil {
		return 0, err
	}
	return resp.DeletedCount, nil
}
func (d *DataClient) AddTagsToBinaryDataByIDs(ctx context.Context, tags []string, binaryIds []*pb.BinaryID) error {
	_, err := d.client.AddTagsToBinaryDataByIDs(ctx, &pb.AddTagsToBinaryDataByIDsRequest{BinaryIds: binaryIds, Tags: tags})
	if err != nil {
		return err
	}
	return nil
}
func (d *DataClient) AddTagsToBinaryDataByFilter(ctx context.Context, tags []string, filter *pb.Filter) error {
	if filter == nil {
		filter = &pb.Filter{}
	}
	_, err := d.client.AddTagsToBinaryDataByFilter(ctx, &pb.AddTagsToBinaryDataByFilterRequest{Filter: filter, Tags: tags})
	if err != nil {
		return err
	}
	return nil
}
func (d *DataClient) RemoveTagsFromBinaryDataByIDs(ctx context.Context, tags []string, binaryIds []*pb.BinaryID) (uint64, error) {
	resp, err := d.client.RemoveTagsFromBinaryDataByIDs(ctx, &pb.RemoveTagsFromBinaryDataByIDsRequest{BinaryIds: binaryIds, Tags: tags})
	if err != nil {
		return 0, err
	}
	return resp.DeletedCount, nil
}
func (d *DataClient) RemoveTagsFromBinaryDataByFilter(ctx context.Context, tags []string, filter *pb.Filter) (uint64, error) {
	if filter == nil {
		filter = &pb.Filter{}
	}
	resp, err := d.client.RemoveTagsFromBinaryDataByFilter(ctx, &pb.RemoveTagsFromBinaryDataByFilterRequest{Filter: filter, Tags: tags})
	if err != nil {
		return 0, err
	}
	return resp.DeletedCount, nil
}
func (d *DataClient) TagsByFilter(ctx context.Context, filter *pb.Filter) ([]string, error) {
	if filter == nil {
		filter = &pb.Filter{}
	}
	resp, err := d.client.TagsByFilter(ctx, &pb.TagsByFilterRequest{Filter: filter})
	if err != nil {
		return nil, err
	}
	return resp.Tags, nil
}
func (d *DataClient) AddBoundingBoxToImageByID(
	ctx context.Context,
	binaryIds []*pb.BinaryID,
	label string,
	xMinNormalized float64,
	yMinNormalized float64,
	xMaxNormalized float64,
	yMaxNormalized float64) (string, error) {
	resp, err := d.client.AddBoundingBoxToImageByID(ctx, &pb.AddBoundingBoxToImageByIDRequest{BinaryId: &pb.BinaryID{}, Label: label, XMinNormalized: xMinNormalized, YMinNormalized: yMinNormalized, XMaxNormalized: xMaxNormalized, YMaxNormalized: yMaxNormalized})
	if err != nil {
		return "", err
	}
	return resp.BboxId, nil

}
func (d *DataClient) RemoveBoundingBoxFromImageByID(ctx context.Context, bboxId string, binaryId *pb.BinaryID) error {
	_, err := d.client.RemoveBoundingBoxFromImageByID(ctx, &pb.RemoveBoundingBoxFromImageByIDRequest{BinaryId: binaryId, BboxId: bboxId})
	if err != nil {
		return err
	}
	return nil
}
func (d *DataClient) BoundingBoxLabelsByFilter(ctx context.Context, filter *pb.Filter) ([]string, error) {
	if filter == nil {
		filter = &pb.Filter{}
	}
	resp, err := d.client.BoundingBoxLabelsByFilter(ctx, &pb.BoundingBoxLabelsByFilterRequest{Filter: filter})
	if err != nil {
		return nil, err
	}
	return resp.Labels, nil
}

// ***python and typescript did not implement this one!!!
func (d *DataClient) UpdateBoundingBox(ctx context.Context,
	binaryId *pb.BinaryID,
	bboxId string,
	label string,
	xMinNormalized float64,
	yMinNormalized float64,
	xMaxNormalized float64,
	yMaxNormalized float64) error {

	_, err := d.client.UpdateBoundingBox(ctx, &pb.UpdateBoundingBoxRequest{BinaryId: binaryId, BboxId: bboxId, Label: &label, XMinNormalized: &xMinNormalized, YMinNormalized: &yMinNormalized, XMaxNormalized: &xMaxNormalized, YMaxNormalized: &yMaxNormalized})
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
func (d *DataClient) AddBinaryDataToDatasetByIDs(ctx context.Context, binaryIds []*pb.BinaryID, datasetId string) error {
	_, err := d.client.AddBinaryDataToDatasetByIDs(ctx, &pb.AddBinaryDataToDatasetByIDsRequest{BinaryIds: binaryIds, DatasetId: datasetId})
	if err != nil {
		return err
	}
	return nil
}
func (d *DataClient) RemoveBinaryDataFromDatasetByIDs(ctx context.Context, binaryIds []*pb.BinaryID, datasetId string) error {
	_, err := d.client.RemoveBinaryDataFromDatasetByIDs(ctx, &pb.RemoveBinaryDataFromDatasetByIDsRequest{BinaryIds: binaryIds, DatasetId: datasetId})
	if err != nil {
		return err
	}
	return nil
}
