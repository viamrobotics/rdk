// Package dataset contains a gRPC based dataset client.
package dataset

import (
	"context"
	"time"

	pb "go.viam.com/api/app/datasync/v1"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/utils/rpc"
)

// Client implements the DataSyncServiceClient interface.
type Client struct {
	client pb.DataSyncServiceClient
	logger logging.Logger
}

type DataType int32

const (
	Unspecified DataType = iota
	BinarySensor
	TabularSensor
	File
)

type MimeType int32
const (
	Unspecified MimeType = iota
	JPEG                 //can i name things this???
	PNG
	PCD
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
	MethodParameters map[string]string
	FileExtension    string
	Tags             []string
}

type SensorMetadata struct {
	TimeRequested time.Time
	TimeReceived  time.Time
	MimeType      MimeType
	//annotations lives in the data client file...so maybe make a shared situation later on??
	Annotations Annotations
}

type TabularData struct {
	Data          map[string]interface{}
	MetadataIndex uint32
	Metadata      UploadMetadata //its usually capturemetadata and idk if this will work or do anything (probs remove this)
	TimeRequested time.Time
	TimeReceived  time.Time
}

type SensorData struct {
	//this is what can be filled by either tabular or binary data!!
	Metadata SensorMetadata
	//its one of, either binary or tabular ==> this needs help
	Binary  []byte
	Tabular TabularData //??? feels wrong
}

// NewDataClient constructs a new DataClient using the connection passed in by the viamClient and the provided logger.
func NewDataSyncClient(
	channel rpc.ClientConn,
	logger logging.Logger,
) (*Client, error) {
	d := pb.NewDataSyncServiceClient(channel)
	return &Client{
		client: d,
		logger: logger,
	}, nil
}

func uploadMetadataToProto(metadata UploadMetadata) *pb.UploadMetadata {
	methodParms, err := protoutils.ConvertStringMapToAnyPBMap(metadata.MethodParameters)
	if err != nil {
		return nil
	}
	return &pb.UploadMetadata{
		PartId:           metadata.PartID,
		ComponentType:    metadata.ComponentType,
		ComponentName:    metadata.ComponentName,
		MethodName:       metadata.MethodName,
		Type:             pb.DataType(metadata.Type),
		MethodParameters: methodParms,
		FileExtension:    metadata.FileExtension,
		Tags:             metadata.Tags,
	}
}


// DataCaptureUpload uploads the contents and metadata for tabular data.
/*
notes:

Metadata       *UploadMetadata
SensorContents []*SensorData

*/
func (d *Client) DataCaptureUpload(ctx context.Context, metadata UploadMetadata, sensorContents []SensorData) error {
	resp, err := d.client.DataCaptureUpload(ctx, &pb.DataCaptureUploadRequest{
		Metadata:       uploadMetadataToProto(metadata), //should be in proto form !!
		SensorContents: //sensorContents needs to go here or something,
	})
	if err != nil {
		return err
	}
	return resp

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

// StreamingDataCaptureUpload uploads the streaming contents and metadata for streaming binary (image + file) data,
// where the first packet must be the UploadMetadata.
func (d *Client) StreamingDataCaptureUpload(ctx context.Context) error {
	resp, err := d.client.FileUpload(ctx, &pb.StreamingDataCaptureUploadRequest{})
	if err != nil {
		return err
	}
	return nil
}
