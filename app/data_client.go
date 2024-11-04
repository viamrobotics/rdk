//go:build !no_cgo

// Package arm contains a gRPC based arm client.
package arm

import (
	"context"
	"errors"
	"net/url"

	"go.mongodb.org/mongo-driver/bson"
	pb "go.viam.com/api/app/data/v1"
	"go.viam.com/rdk/logging"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"
)

// type TabularData struct {
// 	// Data          map[string]interface{}
// 	// Metadata      *pb.CaptureMetadata
// 	// TimeRequested time.Time
// 	// TimeReceived  time.Time
// 	//str
// 	//equals
// }

// viamClient.dataClient.

// i want to wrap NewDataServiceClient define a new dataclient struct and call the wrappers of the functions
// i want the user to call my dataClient struct w my wrappers and not the proto functions
type TabularData struct {
	//actual requested data ==> Data 	map[string]interface{}
	//metadata associated w/ actual data
	//time data were requested
	//time data were recieved
}
type DataClient struct {
	client      pb.DataServiceClient
	TabularData TabularData
}

// NewDataClient initializes the DataClient by calling getDataClient
func NewDataClient(
	ctx context.Context,
	viamBaseURL,
	viamAPIKeyID,
	viamAPIKey string,
	logger logging.Logger,
) (*DataClient, error) {
	u, err := url.Parse(viamBaseURL + ":443")
	if err != nil {
		return nil, err
	}
	opts := rpc.WithEntityCredentials(
		viamAPIKeyID,
		rpc.Credentials{
			Type:    rpc.CredentialsTypeAPIKey,
			Payload: viamAPIKey,
		},
	)
	conn, err := rpc.DialDirectGRPC(ctx, u.Host, logger.AsZap(), opts)
	if err != nil {
		return nil, err
	}

	d := pb.NewDataServiceClient(conn)
	return &DataClient{
		client: d,
	}, nil
}

func (d *DataClient) TabularDataByFilter() error {
	return errors.New("unimplemented")
}

// returns an array of data objects
// interface{} is a special type in Go that represents any type.
// so map[string]interface{} is a map (aka a dictionary) where the keys are strings and the values are of any type
// a list of maps --> like we had in python a list of dictionaries
func (d *DataClient) TabularDataBySQL(ctx context.Context, organizationId string, sqlQuery string) ([]map[string]interface{}, error) {
	//idk what the return type here is
	resp, _ := d.client.TabularDataBySQL(ctx, &pb.TabularDataBySQLRequest{OrganizationId: organizationId, SqlQuery: sqlQuery})
	// return bson.Unmarshal(resp.RawData)
	// var dataObjects []map[string]interface{}
	// Initialize a slice to hold the data objects
	dataObjects := make([]map[string]interface{}, len(resp.RawData))
	// for _, rawData := range resp.RawData {
	// 	var obj map[string]interface{}
	// 	if err := bson.Unmarshal(rawData, &obj); err != nil {
	// 		return nil, fmt.Errorf("failed to unmarshal BSON: %w", err)
	// 	}
	// 	dataObjects = append(dataObjects, obj)
	// }
	// Loop over each BSON byte array in the response and unmarshal directly into the dataObjects slice
	for i, rawData := range resp.RawData {
		obj := make(map[string]interface{})
		bson.Unmarshal(rawData, &obj)
		//do we want an error message for bson.Unmarshal...?
		dataObjects[i] = obj
	}
	return dataObjects, nil

}

func (d *DataClient) TabularDataByMQL() error {
	return errors.New("unimplemented")
}

func (d *DataClient) BinaryDataByFilter() error {
	return errors.New("unimplemented")
}

func (d *DataClient) BinaryDataByIDs() error {
	return errors.New("unimplemented")
}
func (d *DataClient) DeleteTabularData(ctx context.Context, organizationId string, deleteOlderThanDays uint32) (deletedCount uint64, err error) {
	//so many things wrong w this - this does not feel like a safe wrapper?
	// grpc.EmptyCallOption() feels wrong!
	resp, _ := d.client.DeleteTabularData(ctx, &pb.DeleteTabularDataRequest{OrganizationId: organizationId, DeleteOlderThanDays: deleteOlderThanDays}, grpc.EmptyCallOption{})

	return resp.DeletedCount, nil
}

func (d *DataClient) DeleteBinaryDataByFilter() error {
	return errors.New("unimplemented")
}
func (d *DataClient) DeleteBinaryDataByIDs() error {
	return errors.New("unimplemented")
}
func (d *DataClient) AddTagsToBinaryDataByIDs() error {
	return errors.New("unimplemented")
}
func (d *DataClient) AddTagsToBinaryDataByFilter() error {
	return errors.New("unimplemented")
}
func (d *DataClient) RemoveTagsFromBinaryDataByIDs() error {
	return errors.New("unimplemented")
}
func (d *DataClient) RemoveTagsFromBinaryDataByFilter() error {
	return errors.New("unimplemented")
}
func (d *DataClient) TagsByFilter() error {
	return errors.New("unimplemented")
}
func (d *DataClient) AddBoundingBoxToImageByID() error {
	return errors.New("unimplemented")
}
func (d *DataClient) RemoveBoundingBoxFromImageByID() error {
	return errors.New("unimplemented")
}
func (d *DataClient) BoundingBoxLabelsByFilter() error {
	return errors.New("unimplemented")
}
func (d *DataClient) UpdateBoundingBox() error {
	return errors.New("unimplemented")
}
func (d *DataClient) GetDatabaseConnection() error {
	return errors.New("unimplemented")
}
func (d *DataClient) ConfigureDatabaseUser() error {
	return errors.New("unimplemented")
}
func (d *DataClient) AddBinaryDataToDatasetByIDs() error {
	return errors.New("unimplemented")
}
func (d *DataClient) RemoveBinaryDataFromDatasetByIDs() error {
	return errors.New("unimplemented")
}
