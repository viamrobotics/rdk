package inject

import (
	"context"

	datapb "go.viam.com/api/app/data/v1"
	"google.golang.org/grpc"
)

// DataServiceClient represents a fake instance of a data service client.
type DataServiceClient struct {
	datapb.DataServiceClient
	//nolint:deprecated,staticcheck
	TabularDataByFilterFunc func(ctx context.Context, in *datapb.TabularDataByFilterRequest,
		//nolint:deprecated,staticcheck
		opts ...grpc.CallOption) (*datapb.TabularDataByFilterResponse, error)
	TabularDataBySQLFunc func(ctx context.Context, in *datapb.TabularDataBySQLRequest,
		opts ...grpc.CallOption) (*datapb.TabularDataBySQLResponse, error)
	TabularDataByMQLFunc func(ctx context.Context, in *datapb.TabularDataByMQLRequest,
		opts ...grpc.CallOption) (*datapb.TabularDataByMQLResponse, error)
	GetLatestTabularDataFunc func(ctx context.Context, in *datapb.GetLatestTabularDataRequest,
		opts ...grpc.CallOption) (*datapb.GetLatestTabularDataResponse, error)
	ExportTabularDataFunc func(ctx context.Context, in *datapb.ExportTabularDataRequest,
		opts ...grpc.CallOption) (datapb.DataService_ExportTabularDataClient, error)
	BinaryDataByFilterFunc func(ctx context.Context, in *datapb.BinaryDataByFilterRequest,
		opts ...grpc.CallOption) (*datapb.BinaryDataByFilterResponse, error)
	BinaryDataByIDsFunc func(ctx context.Context, in *datapb.BinaryDataByIDsRequest,
		opts ...grpc.CallOption) (*datapb.BinaryDataByIDsResponse, error)
	DeleteTabularDataFunc func(ctx context.Context, in *datapb.DeleteTabularDataRequest,
		opts ...grpc.CallOption) (*datapb.DeleteTabularDataResponse, error)
	DeleteBinaryDataByFilterFunc func(ctx context.Context, in *datapb.DeleteBinaryDataByFilterRequest,
		opts ...grpc.CallOption) (*datapb.DeleteBinaryDataByFilterResponse, error)
	DeleteBinaryDataByIDsFunc func(ctx context.Context, in *datapb.DeleteBinaryDataByIDsRequest,
		opts ...grpc.CallOption) (*datapb.DeleteBinaryDataByIDsResponse, error)
	AddTagsToBinaryDataByIDsFunc func(ctx context.Context, in *datapb.AddTagsToBinaryDataByIDsRequest,
		opts ...grpc.CallOption) (*datapb.AddTagsToBinaryDataByIDsResponse, error)
	AddTagsToBinaryDataByFilterFunc func(ctx context.Context, in *datapb.AddTagsToBinaryDataByFilterRequest,
		opts ...grpc.CallOption) (*datapb.AddTagsToBinaryDataByFilterResponse, error)
	RemoveTagsFromBinaryDataByIDsFunc func(ctx context.Context, in *datapb.RemoveTagsFromBinaryDataByIDsRequest,
		opts ...grpc.CallOption) (*datapb.RemoveTagsFromBinaryDataByIDsResponse, error)
	RemoveTagsFromBinaryDataByFilterFunc func(ctx context.Context, in *datapb.RemoveTagsFromBinaryDataByFilterRequest,
		opts ...grpc.CallOption) (*datapb.RemoveTagsFromBinaryDataByFilterResponse, error)
	TagsByFilterFunc func(ctx context.Context, in *datapb.TagsByFilterRequest,
		opts ...grpc.CallOption) (*datapb.TagsByFilterResponse, error)
	AddBoundingBoxToImageByIDFunc func(ctx context.Context, in *datapb.AddBoundingBoxToImageByIDRequest,
		opts ...grpc.CallOption) (*datapb.AddBoundingBoxToImageByIDResponse, error)
	RemoveBoundingBoxFromImageByIDFunc func(ctx context.Context, in *datapb.RemoveBoundingBoxFromImageByIDRequest,
		opts ...grpc.CallOption) (*datapb.RemoveBoundingBoxFromImageByIDResponse, error)
	BoundingBoxLabelsByFilterFunc func(ctx context.Context, in *datapb.BoundingBoxLabelsByFilterRequest,
		opts ...grpc.CallOption) (*datapb.BoundingBoxLabelsByFilterResponse, error)
	UpdateBoundingBoxFunc func(ctx context.Context, in *datapb.UpdateBoundingBoxRequest,
		opts ...grpc.CallOption) (*datapb.UpdateBoundingBoxResponse, error)
	GetDatabaseConnectionFunc func(ctx context.Context, in *datapb.GetDatabaseConnectionRequest,
		opts ...grpc.CallOption) (*datapb.GetDatabaseConnectionResponse, error)
	ConfigureDatabaseUserFunc func(ctx context.Context, in *datapb.ConfigureDatabaseUserRequest,
		opts ...grpc.CallOption) (*datapb.ConfigureDatabaseUserResponse, error)
	AddBinaryDataToDatasetByIDsFunc func(ctx context.Context, in *datapb.AddBinaryDataToDatasetByIDsRequest,
		opts ...grpc.CallOption) (*datapb.AddBinaryDataToDatasetByIDsResponse, error)
	RemoveBinaryDataFromDatasetByIDsFunc func(ctx context.Context, in *datapb.RemoveBinaryDataFromDatasetByIDsRequest,
		opts ...grpc.CallOption) (*datapb.RemoveBinaryDataFromDatasetByIDsResponse, error)
}

// TabularDataByFilter calls the injected TabularDataByFilter or the real version.
//
//nolint:deprecated,staticcheck
func (client *DataServiceClient) TabularDataByFilter(ctx context.Context, in *datapb.TabularDataByFilterRequest,
	opts ...grpc.CallOption,
	//nolint:deprecated,staticcheck
) (*datapb.TabularDataByFilterResponse, error) {
	if client.TabularDataByFilterFunc == nil {
		//nolint:deprecated,staticcheck
		return client.DataServiceClient.TabularDataByFilter(ctx, in, opts...)
	}
	return client.TabularDataByFilterFunc(ctx, in, opts...)
}

// TabularDataBySQL calls the injected TabularDataBySQL or the real version.
func (client *DataServiceClient) TabularDataBySQL(ctx context.Context, in *datapb.TabularDataBySQLRequest,
	opts ...grpc.CallOption,
) (*datapb.TabularDataBySQLResponse, error) {
	if client.TabularDataBySQLFunc == nil {
		return client.DataServiceClient.TabularDataBySQL(ctx, in, opts...)
	}
	return client.TabularDataBySQLFunc(ctx, in, opts...)
}

// TabularDataByMQL calls the injected TabularDataByMQL or the real version.
func (client *DataServiceClient) TabularDataByMQL(ctx context.Context, in *datapb.TabularDataByMQLRequest,
	opts ...grpc.CallOption,
) (*datapb.TabularDataByMQLResponse, error) {
	if client.TabularDataByMQLFunc == nil {
		return client.DataServiceClient.TabularDataByMQL(ctx, in, opts...)
	}
	return client.TabularDataByMQLFunc(ctx, in, opts...)
}

// GetLatestTabularData calls the injected GetLatestTabularData or the real version.
func (client *DataServiceClient) GetLatestTabularData(ctx context.Context, in *datapb.GetLatestTabularDataRequest,
	opts ...grpc.CallOption,
) (*datapb.GetLatestTabularDataResponse, error) {
	if client.GetLatestTabularDataFunc == nil {
		return client.DataServiceClient.GetLatestTabularData(ctx, in, opts...)
	}
	return client.GetLatestTabularDataFunc(ctx, in, opts...)
}

// DataServiceExportTabularDataClient represents a fake instance of a proto DataService_ExportTabularDataClient.
type DataServiceExportTabularDataClient struct {
	datapb.DataService_ExportTabularDataClient
	RecvFunc func() (*datapb.ExportTabularDataResponse, error)
}

// Recv calls the injected RecvFunc or the real version.
func (c *DataServiceExportTabularDataClient) Recv() (*datapb.ExportTabularDataResponse, error) {
	if c.RecvFunc == nil {
		return c.DataService_ExportTabularDataClient.Recv()
	}
	return c.RecvFunc()
}

// ExportTabularData calls the injected ExportTabularData or the real version.
func (client *DataServiceClient) ExportTabularData(ctx context.Context, in *datapb.ExportTabularDataRequest,
	opts ...grpc.CallOption,
) (datapb.DataService_ExportTabularDataClient, error) {
	if client.ExportTabularDataFunc == nil {
		return client.DataServiceClient.ExportTabularData(ctx, in, opts...)
	}
	return client.ExportTabularDataFunc(ctx, in, opts...)
}

// BinaryDataByFilter calls the injected BinaryDataByFilter or the real version.
func (client *DataServiceClient) BinaryDataByFilter(ctx context.Context, in *datapb.BinaryDataByFilterRequest,
	opts ...grpc.CallOption,
) (*datapb.BinaryDataByFilterResponse, error) {
	if client.BinaryDataByFilterFunc == nil {
		return client.DataServiceClient.BinaryDataByFilter(ctx, in, opts...)
	}
	return client.BinaryDataByFilterFunc(ctx, in, opts...)
}

// BinaryDataByIDs calls the injected BinaryDataByIDs or the real version.
func (client *DataServiceClient) BinaryDataByIDs(ctx context.Context, in *datapb.BinaryDataByIDsRequest,
	opts ...grpc.CallOption,
) (*datapb.BinaryDataByIDsResponse, error) {
	if client.BinaryDataByIDsFunc == nil {
		return client.DataServiceClient.BinaryDataByIDs(ctx, in, opts...)
	}
	return client.BinaryDataByIDsFunc(ctx, in, opts...)
}

// DeleteTabularData calls the injected DeleteTabularData or the real version.
func (client *DataServiceClient) DeleteTabularData(ctx context.Context, in *datapb.DeleteTabularDataRequest,
	opts ...grpc.CallOption,
) (*datapb.DeleteTabularDataResponse, error) {
	if client.DeleteTabularDataFunc == nil {
		return client.DataServiceClient.DeleteTabularData(ctx, in, opts...)
	}
	return client.DeleteTabularDataFunc(ctx, in, opts...)
}

// DeleteBinaryDataByFilter calls the injected DeleteBinaryDataByFilter or the real version.
func (client *DataServiceClient) DeleteBinaryDataByFilter(ctx context.Context, in *datapb.DeleteBinaryDataByFilterRequest,
	opts ...grpc.CallOption,
) (*datapb.DeleteBinaryDataByFilterResponse, error) {
	if client.DeleteBinaryDataByFilterFunc == nil {
		return client.DataServiceClient.DeleteBinaryDataByFilter(ctx, in, opts...)
	}
	return client.DeleteBinaryDataByFilterFunc(ctx, in, opts...)
}

// DeleteBinaryDataByIDs calls the injected DeleteBinaryDataByIDs or the real version.
func (client *DataServiceClient) DeleteBinaryDataByIDs(ctx context.Context, in *datapb.DeleteBinaryDataByIDsRequest,
	opts ...grpc.CallOption,
) (*datapb.DeleteBinaryDataByIDsResponse, error) {
	if client.DeleteBinaryDataByIDsFunc == nil {
		return client.DataServiceClient.DeleteBinaryDataByIDs(ctx, in, opts...)
	}
	return client.DeleteBinaryDataByIDsFunc(ctx, in, opts...)
}

// AddTagsToBinaryDataByIDs calls the injected AddTagsToBinaryDataByIDs or the real version.
func (client *DataServiceClient) AddTagsToBinaryDataByIDs(ctx context.Context, in *datapb.AddTagsToBinaryDataByIDsRequest,
	opts ...grpc.CallOption,
) (*datapb.AddTagsToBinaryDataByIDsResponse, error) {
	if client.AddTagsToBinaryDataByIDsFunc == nil {
		return client.DataServiceClient.AddTagsToBinaryDataByIDs(ctx, in, opts...)
	}
	return client.AddTagsToBinaryDataByIDsFunc(ctx, in, opts...)
}

// AddTagsToBinaryDataByFilter calls the injected AddTagsToBinaryDataByFilter or the real version.
func (client *DataServiceClient) AddTagsToBinaryDataByFilter(ctx context.Context, in *datapb.AddTagsToBinaryDataByFilterRequest,
	opts ...grpc.CallOption,
) (*datapb.AddTagsToBinaryDataByFilterResponse, error) {
	if client.AddTagsToBinaryDataByFilterFunc == nil {
		return client.DataServiceClient.AddTagsToBinaryDataByFilter(ctx, in, opts...)
	}
	return client.AddTagsToBinaryDataByFilterFunc(ctx, in, opts...)
}

// RemoveTagsFromBinaryDataByIDs calls the injected RemoveTagsFromBinaryDataByIDs or the real version.
func (client *DataServiceClient) RemoveTagsFromBinaryDataByIDs(ctx context.Context,
	in *datapb.RemoveTagsFromBinaryDataByIDsRequest,
	opts ...grpc.CallOption,
) (*datapb.RemoveTagsFromBinaryDataByIDsResponse, error) {
	if client.RemoveTagsFromBinaryDataByIDsFunc == nil {
		return client.DataServiceClient.RemoveTagsFromBinaryDataByIDs(ctx, in, opts...)
	}
	return client.RemoveTagsFromBinaryDataByIDsFunc(ctx, in, opts...)
}

// RemoveTagsFromBinaryDataByFilter calls the injected RemoveTagsFromBinaryDataByFilter or the real version.
func (client *DataServiceClient) RemoveTagsFromBinaryDataByFilter(ctx context.Context,
	in *datapb.RemoveTagsFromBinaryDataByFilterRequest,
	opts ...grpc.CallOption,
) (*datapb.RemoveTagsFromBinaryDataByFilterResponse, error) {
	if client.RemoveTagsFromBinaryDataByFilterFunc == nil {
		return client.DataServiceClient.RemoveTagsFromBinaryDataByFilter(ctx, in, opts...)
	}
	return client.RemoveTagsFromBinaryDataByFilterFunc(ctx, in, opts...)
}

// TagsByFilter calls the injected TagsByFilter or the real version.
func (client *DataServiceClient) TagsByFilter(ctx context.Context, in *datapb.TagsByFilterRequest,
	opts ...grpc.CallOption,
) (*datapb.TagsByFilterResponse, error) {
	if client.TagsByFilterFunc == nil {
		return client.DataServiceClient.TagsByFilter(ctx, in, opts...)
	}
	return client.TagsByFilterFunc(ctx, in, opts...)
}

// AddBoundingBoxToImageByID calls the injected AddBoundingBoxToImageByID or the real version.
func (client *DataServiceClient) AddBoundingBoxToImageByID(ctx context.Context, in *datapb.AddBoundingBoxToImageByIDRequest,
	opts ...grpc.CallOption,
) (*datapb.AddBoundingBoxToImageByIDResponse, error) {
	if client.AddBoundingBoxToImageByIDFunc == nil {
		return client.DataServiceClient.AddBoundingBoxToImageByID(ctx, in, opts...)
	}
	return client.AddBoundingBoxToImageByIDFunc(ctx, in, opts...)
}

// RemoveBoundingBoxFromImageByID calls the injected RemoveBoundingBoxFromImageByID or the real version.
func (client *DataServiceClient) RemoveBoundingBoxFromImageByID(ctx context.Context,
	in *datapb.RemoveBoundingBoxFromImageByIDRequest, opts ...grpc.CallOption,
) (*datapb.RemoveBoundingBoxFromImageByIDResponse, error) {
	if client.RemoveBoundingBoxFromImageByIDFunc == nil {
		return client.DataServiceClient.RemoveBoundingBoxFromImageByID(ctx, in, opts...)
	}
	return client.RemoveBoundingBoxFromImageByIDFunc(ctx, in, opts...)
}

// BoundingBoxLabelsByFilter calls the injected BoundingBoxLabelsByFilter or the real version.
func (client *DataServiceClient) BoundingBoxLabelsByFilter(ctx context.Context, in *datapb.BoundingBoxLabelsByFilterRequest,
	opts ...grpc.CallOption,
) (*datapb.BoundingBoxLabelsByFilterResponse, error) {
	if client.BoundingBoxLabelsByFilterFunc == nil {
		return client.DataServiceClient.BoundingBoxLabelsByFilter(ctx, in, opts...)
	}
	return client.BoundingBoxLabelsByFilterFunc(ctx, in, opts...)
}

// UpdateBoundingBox calls the injected UpdateBoundingBox or the real version.
func (client *DataServiceClient) UpdateBoundingBox(ctx context.Context, in *datapb.UpdateBoundingBoxRequest,
	opts ...grpc.CallOption,
) (*datapb.UpdateBoundingBoxResponse, error) {
	if client.UpdateBoundingBoxFunc == nil {
		return client.DataServiceClient.UpdateBoundingBox(ctx, in, opts...)
	}
	return client.UpdateBoundingBoxFunc(ctx, in, opts...)
}

// GetDatabaseConnection calls the injected GetDatabaseConnection or the real version.
func (client *DataServiceClient) GetDatabaseConnection(ctx context.Context, in *datapb.GetDatabaseConnectionRequest,
	opts ...grpc.CallOption,
) (*datapb.GetDatabaseConnectionResponse, error) {
	if client.GetDatabaseConnectionFunc == nil {
		return client.DataServiceClient.GetDatabaseConnection(ctx, in, opts...)
	}
	return client.GetDatabaseConnectionFunc(ctx, in, opts...)
}

// ConfigureDatabaseUser calls the injected ConfigureDatabaseUser or the real version.
func (client *DataServiceClient) ConfigureDatabaseUser(ctx context.Context, in *datapb.ConfigureDatabaseUserRequest,
	opts ...grpc.CallOption,
) (*datapb.ConfigureDatabaseUserResponse, error) {
	if client.ConfigureDatabaseUserFunc == nil {
		return client.DataServiceClient.ConfigureDatabaseUser(ctx, in, opts...)
	}
	return client.ConfigureDatabaseUserFunc(ctx, in, opts...)
}

// AddBinaryDataToDatasetByIDs calls the injected AddBinaryDataToDatasetByIDs or the real version.
func (client *DataServiceClient) AddBinaryDataToDatasetByIDs(ctx context.Context, in *datapb.AddBinaryDataToDatasetByIDsRequest,
	opts ...grpc.CallOption,
) (*datapb.AddBinaryDataToDatasetByIDsResponse, error) {
	if client.AddBinaryDataToDatasetByIDsFunc == nil {
		return client.DataServiceClient.AddBinaryDataToDatasetByIDs(ctx, in, opts...)
	}
	return client.AddBinaryDataToDatasetByIDsFunc(ctx, in, opts...)
}

// RemoveBinaryDataFromDatasetByIDs calls the injected RemoveBinaryDataFromDatasetByIDs or the real version.
func (client *DataServiceClient) RemoveBinaryDataFromDatasetByIDs(ctx context.Context, in *datapb.RemoveBinaryDataFromDatasetByIDsRequest,
	opts ...grpc.CallOption,
) (*datapb.RemoveBinaryDataFromDatasetByIDsResponse, error) {
	if client.RemoveBinaryDataFromDatasetByIDsFunc == nil {
		return client.DataServiceClient.RemoveBinaryDataFromDatasetByIDs(ctx, in, opts...)
	}
	return client.RemoveBinaryDataFromDatasetByIDsFunc(ctx, in, opts...)
}
