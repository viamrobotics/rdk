// Package app contains a gRPC based datasync client.
package app

import (
	"context"
	"errors"
	"time"

	pb "go.viam.com/api/app/datasync/v1"
	"go.viam.com/rdk/logging"

	// "go.viam.com/rdk/protoutils"

	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Client implements the DataSyncServiceClient interface.
type Client struct {
	client pb.DataSyncServiceClient
	logger logging.Logger
}

type DataType int32

const (
	DataTypeUnspecified DataType = iota
	DataTypeBinarySensor
	DataTypeTabularSensor
	DataTypeFile
)

type MimeType int32

const (
	MimeTypeUnspecified MimeType = iota
	MimeTypeJPEG                 //can i name things this???
	MimeTypePNG
	MimeTypePCD
)

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
type UploadMetadata struct {
	PartID           string
	ComponentType    string
	ComponentName    string
	MethodName       string
	Type             DataType
	FileName         string
	MethodParameters map[string]interface{} //or map[string]string??
	FileExtension    string
	Tags             []string
}

type TabularData struct {
	Data          map[string]interface{}
	MetadataIndex uint32
	Metadata      UploadMetadata //its usually capturemetadata and idk if this will work or do anything (probs remove this)
	TimeRequested time.Time
	TimeReceived  time.Time
}

// figure out if mimetype and annotations should be included or not
type SensorMetadata struct {
	TimeRequested time.Time
	TimeReceived  time.Time
	// MimeType      MimeType
	//annotations lives in the data client file...so maybe make a shared situation later on??
	// Annotations Annotations
}
type SensorData struct {
	//this is what can be filled by either tabular or binary data!!
	Metadata SensorMetadata
	//its one of, either binary or tabular ==> this needs help
	SDStruct map[string]interface{} //or should it be TabularData.data ??
	SDBinary []byte
}

// NewDataClient constructs a new DataClient using the connection passed in by the viamClient and the provided logger.
func NewDataSyncClient(conn rpc.ClientConn) *Client {
	d := pb.NewDataSyncServiceClient(conn)
	return &Client{
		client: d,
	}
}

// ConvertMapToProtobufAny converts a map[string]interface{} to a map[string]*anypb.Any
func convertMapToProtoAny(input map[string]interface{}) (map[string]*anypb.Any, error) {
	protoMap := make(map[string]*anypb.Any)
	for key, value := range input {
		// Convert the value to a protobuf Struct-compatible type
		structValue, err := structpb.NewValue(value)
		if err != nil {
			return nil, err
		}
		// Pack the structpb.Value into an anypb.Any
		anyValue, err := anypb.New(structValue)
		if err != nil {
			return nil, err
		}
		// Assign the packed value to the map
		protoMap[key] = anyValue
	}
	return protoMap, nil
}

func uploadMetadataToProto(metadata UploadMetadata) *pb.UploadMetadata {
	// methodParms, err := protoutils.ConvertStringMapToAnyPBMap(metadata.MethodParameters)
	methodParams, err := convertMapToProtoAny(metadata.MethodParameters)

	if err != nil {
		return nil
	}
	return &pb.UploadMetadata{
		PartId:           metadata.PartID,
		ComponentType:    metadata.ComponentType,
		ComponentName:    metadata.ComponentName,
		MethodName:       metadata.MethodName,
		Type:             pb.DataType(metadata.Type),
		MethodParameters: methodParams,
		FileExtension:    metadata.FileExtension,
		Tags:             metadata.Tags,
	}
}

// why doesnt this protoype have mime type and annotations with it??
func sensorMetadataToProto(metadata SensorMetadata) *pb.SensorMetadata {
	return &pb.SensorMetadata{
		TimeRequested: timestamppb.New(metadata.TimeRequested),
		TimeReceived:  timestamppb.New(metadata.TimeReceived),
	}
}

func sensorDataToProto(sensorData SensorData) *pb.SensorData {
	protoSensorData := &pb.SensorData{
		Metadata: sensorMetadataToProto(sensorData.Metadata),
	}
	if sensorData.SDBinary != nil && len(sensorData.SDBinary) > 0 {
		protoSensorData.Data = &pb.SensorData_Binary{
			Binary: sensorData.SDBinary,
		}
	} else if sensorData.SDStruct != nil {
		pbStruct, _ := structpb.NewStruct(sensorData.SDStruct)
		protoSensorData.Data = &pb.SensorData_Struct{
			Struct: pbStruct,
		}
	} else {
		return nil //should an error message be set instead??
	}
	return protoSensorData
}
func sensorContentsToProto(sensorContents []SensorData) []*pb.SensorData {
	var protoSensorContents []*pb.SensorData
	for _, item := range sensorContents {
		protoSensorContents = append(protoSensorContents, sensorDataToProto(item))
	}
	return protoSensorContents
}

func (d *Client) BinaryDataCaptureUpload(
	ctx context.Context,
	binaryData []byte,
	partID string,
	componentType string,
	componentName string,
	methodName string,
	fileExtension string,
	methodParameters map[string]interface{},
	tags []string,
	dataRequestTimes [2]time.Time, // Assuming two time values, [0] is timeRequested, [1] is timeReceived
) (string, error) {
	// Validate file extension
	if fileExtension != "" && fileExtension[0] != '.' {
		fileExtension = "." + fileExtension
	}
	// Create SensorMetadata based on the provided times
	var sensorMetadata SensorMetadata
	if len(dataRequestTimes) == 2 { //can i have a better check here? like if dataRequestTimes != [2]time.Time{}
		sensorMetadata = SensorMetadata{
			TimeRequested: dataRequestTimes[0],
			TimeReceived:  dataRequestTimes[1],
		}
	}
	// Create SensorData
	sensorData := SensorData{
		Metadata: sensorMetadata,
		SDStruct: nil,        // Assuming no struct is needed for binary data
		SDBinary: binaryData, // Attach the binary data
	}
	// Create UploadMetadata
	metadata := UploadMetadata{
		PartID:           partID,
		ComponentType:    componentType,
		ComponentName:    componentName,
		MethodName:       methodName,
		Type:             DataTypeBinarySensor, // assuming this is the correct type??
		MethodParameters: methodParameters,
		Tags:             tags,
	}
	response, err := d.DataCaptureUpload(ctx, metadata, []SensorData{sensorData})
	if err != nil {
		return "", err
	}
	return response, nil
}

func (d *Client) tabularDataCaptureUpload(
	ctx context.Context,
	tabularData []map[string]interface{},
	partID string,
	componentType string,
	componentName string,
	methodName string,
	dataRequestTimes [][2]time.Time, // Assuming two time values, [0] is timeRequested, [1] is timeReceived
	// fileExtension string,
	methodParameters map[string]interface{},
	tags []string,
) (string, error) {
	if len(dataRequestTimes) != len(tabularData) {
		errors.New("dataRequestTimes and tabularData lengths must be equal")
	}
	var sensorContents []SensorData
	// Iterate through the tabular data
	for i, tabData := range tabularData {
		sensorMetadata := SensorMetadata{}
		dates := dataRequestTimes[i]
		if len(dates) == 2 {
			sensorMetadata.TimeRequested = dates[0]
			sensorMetadata.TimeReceived = dates[1]
		}
		// Create SensorData
		sensorData := SensorData{
			Metadata: sensorMetadata,
			SDStruct: tabData,
			SDBinary: nil,
		}
		sensorContents = append(sensorContents, sensorData)
	}

	// Create UploadMetadata
	metadata := UploadMetadata{
		PartID:           partID,
		ComponentType:    componentType,
		ComponentName:    componentName,
		MethodName:       methodName,
		Type:             DataTypeTabularSensor, // assuming this is the correct type??
		MethodParameters: methodParameters,
		Tags:             tags,
	}
	response, err := d.DataCaptureUpload(ctx, metadata, sensorContents)
	if err != nil {
		return "", err
	}
	return response, nil
}

// DataCaptureUpload uploads the metadata and contents for either tabular or binary data,
// and returns the file ID associated with the uploaded data and metadata.
func (d *Client) DataCaptureUpload(ctx context.Context, metadata UploadMetadata, sensorContents []SensorData) (string, error) {
	resp, err := d.client.DataCaptureUpload(ctx, &pb.DataCaptureUploadRequest{
		Metadata:       uploadMetadataToProto(metadata), //should be in proto form !!
		SensorContents: sensorContentsToProto(sensorContents),
	})
	if err != nil {
		return "", err
	}
	return resp.FileId, nil

}

// FileUpload uploads the contents and metadata for binary (image + file) data,
// where the first packet must be the UploadMetadata.
func (d *Client) FileUpload(ctx context.Context) error {
	resp, err := d.client.FileUpload(ctx, &pb.FileUploadRequest{})
	if err != nil {
		return err
	}
	return nil
}

// FileUpload uploads the contents and metadata for binary (image + file) data,
// where the first packet must be the UploadMetadata.
func (d *Client) FileUploadFromPath(ctx context.Context) error {
	// resp, err := d.client.FileUpload(ctx, &pb.FileUploadRequest{})
	// if err != nil {
	// 	return err
	// }
	return nil
}

// StreamingDataCaptureUpload uploads the streaming contents and metadata for streaming binary (image + file) data,
// where the first packet must be the UploadMetadata.
func (d *Client) StreamingDataCaptureUpload(ctx context.Context) error {
	resp, err := d.client.FileUpload(ctx, &pb.StreamingDataCaptureUploadRequest{})
	if err != nil {
		return err
	}
	return nil
}
