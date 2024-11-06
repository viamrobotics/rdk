package inject

import (
	"context"

	datapb "go.viam.com/api/app/data/v1"
	"google.golang.org/grpc"
)

// DataServiceClient represents a fake instance of a data service client.
type DataServiceClient struct {
	datapb.DataServiceClient
	TabularDataByFilterFunc              func(ctx context.Context, in *datapb.TabularDataByFilterRequest, opts ...grpc.CallOption) (*datapb.TabularDataByFilterResponse, error)
	TabularDataBySQLFunc                 func(ctx context.Context, in *datapb.TabularDataBySQLRequest, opts ...grpc.CallOption) (*datapb.TabularDataBySQLResponse, error)
	TabularDataByMQLFunc                 func(ctx context.Context, in *datapb.TabularDataByMQLRequest, opts ...grpc.CallOption) (*datapb.TabularDataByMQLResponse, error)
	BinaryDataByFilterFunc               func(ctx context.Context, in *datapb.BinaryDataByFilterRequest, opts ...grpc.CallOption) (*datapb.BinaryDataByFilterResponse, error)
	BinaryDataByIDsFunc                  func(ctx context.Context, in *datapb.BinaryDataByIDsRequest, opts ...grpc.CallOption) (*datapb.BinaryDataByIDsResponse, error)
	DeleteTabularDataFunc                func(ctx context.Context, in *datapb.DeleteTabularDataRequest, opts ...grpc.CallOption) (*datapb.DeleteTabularDataResponse, error)
	DeleteBinaryDataByFilterFunc         func(ctx context.Context, in *datapb.DeleteBinaryDataByFilterRequest, opts ...grpc.CallOption) (*datapb.DeleteBinaryDataByFilterResponse, error)
	DeleteBinaryDataByIDsFunc            func(ctx context.Context, in *datapb.DeleteBinaryDataByIDsRequest, opts ...grpc.CallOption) (*datapb.DeleteBinaryDataByIDsResponse, error)
	AddTagsToBinaryDataByIDsFunc         func(ctx context.Context, in *datapb.AddTagsToBinaryDataByIDsRequest, opts ...grpc.CallOption) (*datapb.AddTagsToBinaryDataByIDsResponse, error)
	AddTagsToBinaryDataByFilterFunc      func(ctx context.Context, in *datapb.AddTagsToBinaryDataByFilterRequest, opts ...grpc.CallOption) (*datapb.AddTagsToBinaryDataByFilterResponse, error)
	RemoveTagsFromBinaryDataByIDsFunc    func(ctx context.Context, in *datapb.RemoveTagsFromBinaryDataByIDsRequest, opts ...grpc.CallOption) (*datapb.RemoveTagsFromBinaryDataByIDsResponse, error)
	RemoveTagsFromBinaryDataByFilterFunc func(ctx context.Context, in *datapb.RemoveTagsFromBinaryDataByFilterRequest, opts ...grpc.CallOption) (*datapb.RemoveTagsFromBinaryDataByFilterResponse, error)
	TagsByFilterFunc                     func(ctx context.Context, in *datapb.TagsByFilterRequest, opts ...grpc.CallOption) (*datapb.TagsByFilterResponse, error)
	AddBoundingBoxToImageByIDFunc        func(ctx context.Context, in *datapb.AddBoundingBoxToImageByIDRequest, opts ...grpc.CallOption) (*datapb.AddBoundingBoxToImageByIDResponse, error)
	RemoveBoundingBoxFromImageByIDFunc   func(ctx context.Context, in *datapb.RemoveBoundingBoxFromImageByIDRequest, opts ...grpc.CallOption) (*datapb.RemoveBoundingBoxFromImageByIDResponse, error)
	BoundingBoxLabelsByFilterFunc        func(ctx context.Context, in *datapb.BoundingBoxLabelsByFilterRequest, opts ...grpc.CallOption) (*datapb.BoundingBoxLabelsByFilterResponse, error)
	UpdateBoundingBoxFunc                func(ctx context.Context, in *datapb.UpdateBoundingBoxRequest, opts ...grpc.CallOption) (*datapb.UpdateBoundingBoxResponse, error)
	GetDatabaseConnectionFunc            func(ctx context.Context, in *datapb.GetDatabaseConnectionRequest, opts ...grpc.CallOption) (*datapb.GetDatabaseConnectionResponse, error)
	ConfigureDatabaseUserFunc            func(ctx context.Context, in *datapb.ConfigureDatabaseUserRequest, opts ...grpc.CallOption) (*datapb.ConfigureDatabaseUserResponse, error)
	AddBinaryDataToDatasetByIDsFunc      func(ctx context.Context, in *datapb.AddBinaryDataToDatasetByIDsRequest, opts ...grpc.CallOption) (*datapb.AddBinaryDataToDatasetByIDsResponse, error)
	RemoveBinaryDataFromDatasetByIDsFunc func(ctx context.Context, in *datapb.RemoveBinaryDataFromDatasetByIDsRequest, opts ...grpc.CallOption) (*datapb.RemoveBinaryDataFromDatasetByIDsResponse, error)
}

// TabularDataByFilter calls the injected TabularDataByFilter or the real version.
func (client *DataServiceClient) TabularDataByFilter(ctx context.Context, in *datapb.TabularDataByFilterRequest, opts ...grpc.CallOption,
) (*datapb.TabularDataByFilterResponse, error) {
	if client.TabularDataByFilterFunc == nil {
		return client.DataServiceClient.TabularDataByFilter(ctx, in, opts...)
	}
	return client.TabularDataByFilterFunc(ctx, in, opts...)
}

func (client *DataServiceClient) TabularDataBySQL(ctx context.Context, in *datapb.TabularDataBySQLRequest, opts ...grpc.CallOption) (*datapb.TabularDataBySQLResponse, error) {
	if client.TabularDataBySQLFunc == nil {
		return client.DataServiceClient.TabularDataBySQL(ctx, in, opts...)
	}
	return client.TabularDataBySQLFunc(ctx, in, opts...)
}

func (client *DataServiceClient) TabularDataByMQL(ctx context.Context, in *datapb.TabularDataByMQLRequest, opts ...grpc.CallOption) (*datapb.TabularDataByMQLResponse, error) {
	if client.TabularDataByMQLFunc == nil {
		return client.DataServiceClient.TabularDataByMQL(ctx, in, opts...)
	}
	return client.TabularDataByMQLFunc(ctx, in, opts...)
}

func (client *DataServiceClient) BinaryDataByFilter(ctx context.Context, in *datapb.BinaryDataByFilterRequest, opts ...grpc.CallOption) (*datapb.BinaryDataByFilterResponse, error) {
	if client.BinaryDataByFilterFunc == nil {
		return client.DataServiceClient.BinaryDataByFilter(ctx, in, opts...)
	}
	return client.BinaryDataByFilterFunc(ctx, in, opts...)
}

func (client *DataServiceClient) BinaryDataByIDs(ctx context.Context, in *datapb.BinaryDataByIDsRequest, opts ...grpc.CallOption) (*datapb.BinaryDataByIDsResponse, error) {
	if client.BinaryDataByIDsFunc == nil {
		return client.DataServiceClient.BinaryDataByIDs(ctx, in, opts...)
	}
	return client.BinaryDataByIDsFunc(ctx, in, opts...)
}

func (client *DataServiceClient) DeleteTabularData(ctx context.Context, in *datapb.DeleteTabularDataRequest, opts ...grpc.CallOption) (*datapb.DeleteTabularDataResponse, error) {
	if client.DeleteTabularDataFunc == nil {
		return client.DataServiceClient.DeleteTabularData(ctx, in, opts...)
	}
	return client.DeleteTabularDataFunc(ctx, in, opts...)
}

func (client *DataServiceClient) DeleteBinaryDataByFilter(ctx context.Context, in *datapb.DeleteBinaryDataByFilterRequest, opts ...grpc.CallOption) (*datapb.DeleteBinaryDataByFilterResponse, error) {
	if client.DeleteBinaryDataByFilterFunc == nil {
		return client.DataServiceClient.DeleteBinaryDataByFilter(ctx, in, opts...)
	}
	return client.DeleteBinaryDataByFilterFunc(ctx, in, opts...)
}

func (client *DataServiceClient) DeleteBinaryDataByIDs(ctx context.Context, in *datapb.DeleteBinaryDataByIDsRequest, opts ...grpc.CallOption) (*datapb.DeleteBinaryDataByIDsResponse, error) {
	if client.DeleteBinaryDataByIDsFunc == nil {
		return client.DataServiceClient.DeleteBinaryDataByIDs(ctx, in, opts...)
	}
	return client.DeleteBinaryDataByIDsFunc(ctx, in, opts...)
}

func (client *DataServiceClient) AddTagsToBinaryDataByIDs(ctx context.Context, in *datapb.AddTagsToBinaryDataByIDsRequest, opts ...grpc.CallOption) (*datapb.AddTagsToBinaryDataByIDsResponse, error) {
	if client.AddTagsToBinaryDataByIDsFunc == nil {
		return client.DataServiceClient.AddTagsToBinaryDataByIDs(ctx, in, opts...)
	}
	return client.AddTagsToBinaryDataByIDsFunc(ctx, in, opts...)
}

func (client *DataServiceClient) AddTagsToBinaryDataByFilter(ctx context.Context, in *datapb.AddTagsToBinaryDataByFilterRequest, opts ...grpc.CallOption) (*datapb.AddTagsToBinaryDataByFilterResponse, error) {
	if client.AddTagsToBinaryDataByFilterFunc == nil {
		return client.DataServiceClient.AddTagsToBinaryDataByFilter(ctx, in, opts...)
	}
	return client.AddTagsToBinaryDataByFilterFunc(ctx, in, opts...)
}

func (client *DataServiceClient) RemoveTagsFromBinaryDataByIDs(ctx context.Context, in *datapb.RemoveTagsFromBinaryDataByIDsRequest, opts ...grpc.CallOption) (*datapb.RemoveTagsFromBinaryDataByIDsResponse, error) {
	if client.RemoveTagsFromBinaryDataByIDsFunc == nil {
		return client.DataServiceClient.RemoveTagsFromBinaryDataByIDs(ctx, in, opts...)
	}
	return client.RemoveTagsFromBinaryDataByIDsFunc(ctx, in, opts...)
}

func (client *DataServiceClient) RemoveTagsFromBinaryDataByFilter(ctx context.Context, in *datapb.RemoveTagsFromBinaryDataByFilterRequest, opts ...grpc.CallOption) (*datapb.RemoveTagsFromBinaryDataByFilterResponse, error) {
	if client.RemoveTagsFromBinaryDataByFilterFunc == nil {
		return client.DataServiceClient.RemoveTagsFromBinaryDataByFilter(ctx, in, opts...)
	}
	return client.RemoveTagsFromBinaryDataByFilterFunc(ctx, in, opts...)
}

func (client *DataServiceClient) TagsByFilter(ctx context.Context, in *datapb.TagsByFilterRequest, opts ...grpc.CallOption) (*datapb.TagsByFilterResponse, error) {
	if client.TagsByFilterFunc == nil {
		return client.DataServiceClient.TagsByFilter(ctx, in, opts...)
	}
	return client.TagsByFilterFunc(ctx, in, opts...)
}

func (client *DataServiceClient) AddBoundingBoxToImageByID(ctx context.Context, in *datapb.AddBoundingBoxToImageByIDRequest, opts ...grpc.CallOption) (*datapb.AddBoundingBoxToImageByIDResponse, error) {
	if client.AddBoundingBoxToImageByIDFunc == nil {
		return client.DataServiceClient.AddBoundingBoxToImageByID(ctx, in, opts...)
	}
	return client.AddBoundingBoxToImageByIDFunc(ctx, in, opts...)
}

func (client *DataServiceClient) RemoveBoundingBoxFromImageByID(ctx context.Context, in *datapb.RemoveBoundingBoxFromImageByIDRequest, opts ...grpc.CallOption) (*datapb.RemoveBoundingBoxFromImageByIDResponse, error) {
	if client.RemoveBoundingBoxFromImageByIDFunc == nil {
		return client.DataServiceClient.RemoveBoundingBoxFromImageByID(ctx, in, opts...)
	}
	return client.RemoveBoundingBoxFromImageByIDFunc(ctx, in, opts...)
}

func (client *DataServiceClient) BoundingBoxLabelsByFilter(ctx context.Context, in *datapb.BoundingBoxLabelsByFilterRequest, opts ...grpc.CallOption) (*datapb.BoundingBoxLabelsByFilterResponse, error) {
	if client.BoundingBoxLabelsByFilterFunc == nil {
		return client.DataServiceClient.BoundingBoxLabelsByFilter(ctx, in, opts...)
	}
	return client.BoundingBoxLabelsByFilterFunc(ctx, in, opts...)
}

func (client *DataServiceClient) UpdateBoundingBox(ctx context.Context, in *datapb.UpdateBoundingBoxRequest, opts ...grpc.CallOption) (*datapb.UpdateBoundingBoxResponse, error) {
	if client.UpdateBoundingBoxFunc == nil {
		return client.DataServiceClient.UpdateBoundingBox(ctx, in, opts...)
	}
	return client.UpdateBoundingBoxFunc(ctx, in, opts...)
}

func (client *DataServiceClient) GetDatabaseConnection(ctx context.Context, in *datapb.GetDatabaseConnectionRequest, opts ...grpc.CallOption) (*datapb.GetDatabaseConnectionResponse, error) {
	if client.GetDatabaseConnectionFunc == nil {
		return client.DataServiceClient.GetDatabaseConnection(ctx, in, opts...)
	}
	return client.GetDatabaseConnectionFunc(ctx, in, opts...)
}

func (client *DataServiceClient) ConfigureDatabaseUser(ctx context.Context, in *datapb.ConfigureDatabaseUserRequest, opts ...grpc.CallOption) (*datapb.ConfigureDatabaseUserResponse, error) {
	if client.ConfigureDatabaseUserFunc == nil {
		return client.DataServiceClient.ConfigureDatabaseUser(ctx, in, opts...)
	}
	return client.ConfigureDatabaseUserFunc(ctx, in, opts...)
}

func (client *DataServiceClient) AddBinaryDataToDatasetByIDs(ctx context.Context, in *datapb.AddBinaryDataToDatasetByIDsRequest, opts ...grpc.CallOption) (*datapb.AddBinaryDataToDatasetByIDsResponse, error) {
	if client.AddBinaryDataToDatasetByIDsFunc == nil {
		return client.DataServiceClient.AddBinaryDataToDatasetByIDs(ctx, in, opts...)
	}
	return client.AddBinaryDataToDatasetByIDsFunc(ctx, in, opts...)
}

func (client *DataServiceClient) RemoveBinaryDataFromDatasetByIDs(ctx context.Context, in *datapb.RemoveBinaryDataFromDatasetByIDsRequest, opts ...grpc.CallOption) (*datapb.RemoveBinaryDataFromDatasetByIDsResponse, error) {
	if client.RemoveBinaryDataFromDatasetByIDsFunc == nil {
		return client.DataServiceClient.RemoveBinaryDataFromDatasetByIDs(ctx, in, opts...)
	}
	return client.RemoveBinaryDataFromDatasetByIDsFunc(ctx, in, opts...)
}
