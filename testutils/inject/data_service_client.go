package inject

import (
	"context"

	"braces.dev/errtrace"
	datapb "go.viam.com/api/app/data/v1"
	"google.golang.org/grpc"
)

// DataServiceClient represents a fake instance of a data service client.
type DataServiceClient struct {
	datapb.DataServiceClient
	//nolint:staticcheck
	TabularDataByFilterFunc func(ctx context.Context, in *datapb.TabularDataByFilterRequest,
		//nolint:staticcheck
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
	CreateBinaryDataSignedURLFunc func(ctx context.Context, in *datapb.CreateBinaryDataSignedURLRequest,
		opts ...grpc.CallOption) (*datapb.CreateBinaryDataSignedURLResponse, error)
	DeleteTabularDataFunc func(ctx context.Context, in *datapb.DeleteTabularDataRequest,
		opts ...grpc.CallOption) (*datapb.DeleteTabularDataResponse, error)
	DeleteBinaryDataByFilterFunc func(ctx context.Context, in *datapb.DeleteBinaryDataByFilterRequest,
		opts ...grpc.CallOption) (*datapb.DeleteBinaryDataByFilterResponse, error)
	DeleteBinaryDataByIDsFunc func(ctx context.Context, in *datapb.DeleteBinaryDataByIDsRequest,
		opts ...grpc.CallOption) (*datapb.DeleteBinaryDataByIDsResponse, error)
	AddTagsToBinaryDataByIDsFunc func(ctx context.Context, in *datapb.AddTagsToBinaryDataByIDsRequest,
		opts ...grpc.CallOption) (*datapb.AddTagsToBinaryDataByIDsResponse, error)
	//nolint:staticcheck
	AddTagsToBinaryDataByFilterFunc func(ctx context.Context, in *datapb.AddTagsToBinaryDataByFilterRequest,
		//nolint:staticcheck
		opts ...grpc.CallOption) (*datapb.AddTagsToBinaryDataByFilterResponse, error)
	RemoveTagsFromBinaryDataByIDsFunc func(ctx context.Context, in *datapb.RemoveTagsFromBinaryDataByIDsRequest,
		opts ...grpc.CallOption) (*datapb.RemoveTagsFromBinaryDataByIDsResponse, error)
	//nolint:staticcheck
	RemoveTagsFromBinaryDataByFilterFunc func(ctx context.Context, in *datapb.RemoveTagsFromBinaryDataByFilterRequest,
		//nolint:staticcheck
		opts ...grpc.CallOption) (*datapb.RemoveTagsFromBinaryDataByFilterResponse, error)
	//nolint:staticcheck
	TagsByFilterFunc func(ctx context.Context, in *datapb.TagsByFilterRequest,
		//nolint:staticcheck
		opts ...grpc.CallOption) (*datapb.TagsByFilterResponse, error)
	AddBoundingBoxToImageByIDFunc func(ctx context.Context, in *datapb.AddBoundingBoxToImageByIDRequest,
		opts ...grpc.CallOption) (*datapb.AddBoundingBoxToImageByIDResponse, error)
	RemoveBoundingBoxFromImageByIDFunc func(ctx context.Context, in *datapb.RemoveBoundingBoxFromImageByIDRequest,
		opts ...grpc.CallOption) (*datapb.RemoveBoundingBoxFromImageByIDResponse, error)
	//nolint:staticcheck
	BoundingBoxLabelsByFilterFunc func(ctx context.Context, in *datapb.BoundingBoxLabelsByFilterRequest,
		//nolint:staticcheck
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
	CreateSequenceFunc func(ctx context.Context, in *datapb.CreateSequenceRequest,
		opts ...grpc.CallOption) (*datapb.CreateSequenceResponse, error)
	GetSequenceFunc func(ctx context.Context, in *datapb.GetSequenceRequest,
		opts ...grpc.CallOption) (*datapb.GetSequenceResponse, error)
	UpdateSequenceFunc func(ctx context.Context, in *datapb.UpdateSequenceRequest,
		opts ...grpc.CallOption) (*datapb.UpdateSequenceResponse, error)
	DeleteSequenceFunc func(ctx context.Context, in *datapb.DeleteSequenceRequest,
		opts ...grpc.CallOption) (*datapb.DeleteSequenceResponse, error)
	ListSequencesFunc func(ctx context.Context, in *datapb.ListSequencesRequest,
		opts ...grpc.CallOption) (*datapb.ListSequencesResponse, error)
	AddSequencesToDatasetFunc func(ctx context.Context, in *datapb.AddSequencesToDatasetRequest,
		opts ...grpc.CallOption) (*datapb.AddSequencesToDatasetResponse, error)
	RemoveSequencesFromDatasetFunc func(ctx context.Context, in *datapb.RemoveSequencesFromDatasetRequest,
		opts ...grpc.CallOption) (*datapb.RemoveSequencesFromDatasetResponse, error)
	SequencesByDatasetIDFunc func(ctx context.Context, in *datapb.SequencesByDatasetIDRequest,
		opts ...grpc.CallOption) (*datapb.SequencesByDatasetIDResponse, error)
}

// TabularDataByFilter calls the injected TabularDataByFilter or the real version.
//
//nolint:staticcheck
func (client *DataServiceClient) TabularDataByFilter(ctx context.Context, in *datapb.TabularDataByFilterRequest,
	opts ...grpc.CallOption,
	//nolint:staticcheck
) (*datapb.TabularDataByFilterResponse, error) {
	if client.TabularDataByFilterFunc == nil {
		//nolint:staticcheck
		return errtrace.Wrap2(client.DataServiceClient.TabularDataByFilter(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.TabularDataByFilterFunc(ctx, in, opts...))
}

// TabularDataBySQL calls the injected TabularDataBySQL or the real version.
func (client *DataServiceClient) TabularDataBySQL(ctx context.Context, in *datapb.TabularDataBySQLRequest,
	opts ...grpc.CallOption,
) (*datapb.TabularDataBySQLResponse, error) {
	if client.TabularDataBySQLFunc == nil {
		return errtrace.Wrap2(client.DataServiceClient.TabularDataBySQL(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.TabularDataBySQLFunc(ctx, in, opts...))
}

// TabularDataByMQL calls the injected TabularDataByMQL or the real version.
func (client *DataServiceClient) TabularDataByMQL(ctx context.Context, in *datapb.TabularDataByMQLRequest,
	opts ...grpc.CallOption,
) (*datapb.TabularDataByMQLResponse, error) {
	if client.TabularDataByMQLFunc == nil {
		return errtrace.Wrap2(client.DataServiceClient.TabularDataByMQL(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.TabularDataByMQLFunc(ctx, in, opts...))
}

// GetLatestTabularData calls the injected GetLatestTabularData or the real version.
func (client *DataServiceClient) GetLatestTabularData(ctx context.Context, in *datapb.GetLatestTabularDataRequest,
	opts ...grpc.CallOption,
) (*datapb.GetLatestTabularDataResponse, error) {
	if client.GetLatestTabularDataFunc == nil {
		return errtrace.Wrap2(client.DataServiceClient.GetLatestTabularData(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.GetLatestTabularDataFunc(ctx, in, opts...))
}

// DataServiceExportTabularDataClient represents a fake instance of a proto DataService_ExportTabularDataClient.
type DataServiceExportTabularDataClient struct {
	datapb.DataService_ExportTabularDataClient
	RecvFunc func() (*datapb.ExportTabularDataResponse, error)
}

// Recv calls the injected RecvFunc or the real version.
func (c *DataServiceExportTabularDataClient) Recv() (*datapb.ExportTabularDataResponse, error) {
	if c.RecvFunc == nil {
		return errtrace.Wrap2(c.DataService_ExportTabularDataClient.Recv())
	}
	return errtrace.Wrap2(c.RecvFunc())
}

// ExportTabularData calls the injected ExportTabularData or the real version.
func (client *DataServiceClient) ExportTabularData(ctx context.Context, in *datapb.ExportTabularDataRequest,
	opts ...grpc.CallOption,
) (datapb.DataService_ExportTabularDataClient, error) {
	if client.ExportTabularDataFunc == nil {
		return errtrace.Wrap2(client.DataServiceClient.ExportTabularData(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.ExportTabularDataFunc(ctx, in, opts...))
}

// BinaryDataByFilter calls the injected BinaryDataByFilter or the real version.
func (client *DataServiceClient) BinaryDataByFilter(ctx context.Context, in *datapb.BinaryDataByFilterRequest,
	opts ...grpc.CallOption,
) (*datapb.BinaryDataByFilterResponse, error) {
	if client.BinaryDataByFilterFunc == nil {
		return errtrace.Wrap2(client.DataServiceClient.BinaryDataByFilter(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.BinaryDataByFilterFunc(ctx, in, opts...))
}

// BinaryDataByIDs calls the injected BinaryDataByIDs or the real version.
func (client *DataServiceClient) BinaryDataByIDs(ctx context.Context, in *datapb.BinaryDataByIDsRequest,
	opts ...grpc.CallOption,
) (*datapb.BinaryDataByIDsResponse, error) {
	if client.BinaryDataByIDsFunc == nil {
		return errtrace.Wrap2(client.DataServiceClient.BinaryDataByIDs(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.BinaryDataByIDsFunc(ctx, in, opts...))
}

// CreateBinaryDataSignedURL calls the injected CreateBinaryDataSignedURL or the real version.
func (client *DataServiceClient) CreateBinaryDataSignedURL(ctx context.Context, in *datapb.CreateBinaryDataSignedURLRequest,
	opts ...grpc.CallOption,
) (*datapb.CreateBinaryDataSignedURLResponse, error) {
	if client.CreateBinaryDataSignedURLFunc == nil {
		return errtrace.Wrap2(client.DataServiceClient.CreateBinaryDataSignedURL(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.CreateBinaryDataSignedURLFunc(ctx, in, opts...))
}

// DeleteTabularData calls the injected DeleteTabularData or the real version.
func (client *DataServiceClient) DeleteTabularData(ctx context.Context, in *datapb.DeleteTabularDataRequest,
	opts ...grpc.CallOption,
) (*datapb.DeleteTabularDataResponse, error) {
	if client.DeleteTabularDataFunc == nil {
		return errtrace.Wrap2(client.DataServiceClient.DeleteTabularData(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.DeleteTabularDataFunc(ctx, in, opts...))
}

// DeleteBinaryDataByFilter calls the injected DeleteBinaryDataByFilter or the real version.
func (client *DataServiceClient) DeleteBinaryDataByFilter(ctx context.Context, in *datapb.DeleteBinaryDataByFilterRequest,
	opts ...grpc.CallOption,
) (*datapb.DeleteBinaryDataByFilterResponse, error) {
	if client.DeleteBinaryDataByFilterFunc == nil {
		return errtrace.Wrap2(client.DataServiceClient.DeleteBinaryDataByFilter(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.DeleteBinaryDataByFilterFunc(ctx, in, opts...))
}

// DeleteBinaryDataByIDs calls the injected DeleteBinaryDataByIDs or the real version.
func (client *DataServiceClient) DeleteBinaryDataByIDs(ctx context.Context, in *datapb.DeleteBinaryDataByIDsRequest,
	opts ...grpc.CallOption,
) (*datapb.DeleteBinaryDataByIDsResponse, error) {
	if client.DeleteBinaryDataByIDsFunc == nil {
		return errtrace.Wrap2(client.DataServiceClient.DeleteBinaryDataByIDs(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.DeleteBinaryDataByIDsFunc(ctx, in, opts...))
}

// AddTagsToBinaryDataByIDs calls the injected AddTagsToBinaryDataByIDs or the real version.
func (client *DataServiceClient) AddTagsToBinaryDataByIDs(ctx context.Context, in *datapb.AddTagsToBinaryDataByIDsRequest,
	opts ...grpc.CallOption,
) (*datapb.AddTagsToBinaryDataByIDsResponse, error) {
	if client.AddTagsToBinaryDataByIDsFunc == nil {
		return errtrace.Wrap2(client.DataServiceClient.AddTagsToBinaryDataByIDs(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.AddTagsToBinaryDataByIDsFunc(ctx, in, opts...))
}

// AddTagsToBinaryDataByFilter calls the injected AddTagsToBinaryDataByFilter or the real version.
//
//nolint:staticcheck
func (client *DataServiceClient) AddTagsToBinaryDataByFilter(ctx context.Context, in *datapb.AddTagsToBinaryDataByFilterRequest,
	opts ...grpc.CallOption,
	//nolint:staticcheck
) (*datapb.AddTagsToBinaryDataByFilterResponse, error) {
	if client.AddTagsToBinaryDataByFilterFunc == nil {
		//nolint:staticcheck
		return errtrace.Wrap2(client.DataServiceClient.AddTagsToBinaryDataByFilter(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.AddTagsToBinaryDataByFilterFunc(ctx, in, opts...))
}

// RemoveTagsFromBinaryDataByIDs calls the injected RemoveTagsFromBinaryDataByIDs or the real version.
func (client *DataServiceClient) RemoveTagsFromBinaryDataByIDs(ctx context.Context,
	in *datapb.RemoveTagsFromBinaryDataByIDsRequest,
	opts ...grpc.CallOption,
) (*datapb.RemoveTagsFromBinaryDataByIDsResponse, error) {
	if client.RemoveTagsFromBinaryDataByIDsFunc == nil {
		return errtrace.Wrap2(client.DataServiceClient.RemoveTagsFromBinaryDataByIDs(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.RemoveTagsFromBinaryDataByIDsFunc(ctx, in, opts...))
}

// RemoveTagsFromBinaryDataByFilter calls the injected RemoveTagsFromBinaryDataByFilter or the real version.
//
//nolint:staticcheck
func (client *DataServiceClient) RemoveTagsFromBinaryDataByFilter(ctx context.Context,
	//nolint:staticcheck
	in *datapb.RemoveTagsFromBinaryDataByFilterRequest,
	opts ...grpc.CallOption,
	//nolint:staticcheck
) (*datapb.RemoveTagsFromBinaryDataByFilterResponse, error) {
	if client.RemoveTagsFromBinaryDataByFilterFunc == nil {
		//nolint:staticcheck
		return errtrace.Wrap2(client.DataServiceClient.RemoveTagsFromBinaryDataByFilter(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.RemoveTagsFromBinaryDataByFilterFunc(ctx, in, opts...))
}

// AddBoundingBoxToImageByID calls the injected AddBoundingBoxToImageByID or the real version.
func (client *DataServiceClient) AddBoundingBoxToImageByID(ctx context.Context, in *datapb.AddBoundingBoxToImageByIDRequest,
	opts ...grpc.CallOption,
) (*datapb.AddBoundingBoxToImageByIDResponse, error) {
	if client.AddBoundingBoxToImageByIDFunc == nil {
		return errtrace.Wrap2(client.DataServiceClient.AddBoundingBoxToImageByID(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.AddBoundingBoxToImageByIDFunc(ctx, in, opts...))
}

// RemoveBoundingBoxFromImageByID calls the injected RemoveBoundingBoxFromImageByID or the real version.
func (client *DataServiceClient) RemoveBoundingBoxFromImageByID(ctx context.Context,
	in *datapb.RemoveBoundingBoxFromImageByIDRequest, opts ...grpc.CallOption,
) (*datapb.RemoveBoundingBoxFromImageByIDResponse, error) {
	if client.RemoveBoundingBoxFromImageByIDFunc == nil {
		return errtrace.Wrap2(client.DataServiceClient.RemoveBoundingBoxFromImageByID(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.RemoveBoundingBoxFromImageByIDFunc(ctx, in, opts...))
}

// BoundingBoxLabelsByFilter calls the injected BoundingBoxLabelsByFilter or the real version.
//
//nolint:staticcheck
func (client *DataServiceClient) BoundingBoxLabelsByFilter(ctx context.Context, in *datapb.BoundingBoxLabelsByFilterRequest,
	opts ...grpc.CallOption,
	//nolint:staticcheck
) (*datapb.BoundingBoxLabelsByFilterResponse, error) {
	if client.BoundingBoxLabelsByFilterFunc == nil {
		return errtrace.Wrap2(client.DataServiceClient.BoundingBoxLabelsByFilter(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.BoundingBoxLabelsByFilterFunc(ctx, in, opts...))
}

// UpdateBoundingBox calls the injected UpdateBoundingBox or the real version.
func (client *DataServiceClient) UpdateBoundingBox(ctx context.Context, in *datapb.UpdateBoundingBoxRequest,
	opts ...grpc.CallOption,
) (*datapb.UpdateBoundingBoxResponse, error) {
	if client.UpdateBoundingBoxFunc == nil {
		return errtrace.Wrap2(client.DataServiceClient.UpdateBoundingBox(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.UpdateBoundingBoxFunc(ctx, in, opts...))
}

// GetDatabaseConnection calls the injected GetDatabaseConnection or the real version.
func (client *DataServiceClient) GetDatabaseConnection(ctx context.Context, in *datapb.GetDatabaseConnectionRequest,
	opts ...grpc.CallOption,
) (*datapb.GetDatabaseConnectionResponse, error) {
	if client.GetDatabaseConnectionFunc == nil {
		return errtrace.Wrap2(client.DataServiceClient.GetDatabaseConnection(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.GetDatabaseConnectionFunc(ctx, in, opts...))
}

// ConfigureDatabaseUser calls the injected ConfigureDatabaseUser or the real version.
func (client *DataServiceClient) ConfigureDatabaseUser(ctx context.Context, in *datapb.ConfigureDatabaseUserRequest,
	opts ...grpc.CallOption,
) (*datapb.ConfigureDatabaseUserResponse, error) {
	if client.ConfigureDatabaseUserFunc == nil {
		return errtrace.Wrap2(client.DataServiceClient.ConfigureDatabaseUser(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.ConfigureDatabaseUserFunc(ctx, in, opts...))
}

// AddBinaryDataToDatasetByIDs calls the injected AddBinaryDataToDatasetByIDs or the real version.
func (client *DataServiceClient) AddBinaryDataToDatasetByIDs(ctx context.Context, in *datapb.AddBinaryDataToDatasetByIDsRequest,
	opts ...grpc.CallOption,
) (*datapb.AddBinaryDataToDatasetByIDsResponse, error) {
	if client.AddBinaryDataToDatasetByIDsFunc == nil {
		return errtrace.Wrap2(client.DataServiceClient.AddBinaryDataToDatasetByIDs(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.AddBinaryDataToDatasetByIDsFunc(ctx, in, opts...))
}

// RemoveBinaryDataFromDatasetByIDs calls the injected RemoveBinaryDataFromDatasetByIDs or the real version.
func (client *DataServiceClient) RemoveBinaryDataFromDatasetByIDs(ctx context.Context, in *datapb.RemoveBinaryDataFromDatasetByIDsRequest,
	opts ...grpc.CallOption,
) (*datapb.RemoveBinaryDataFromDatasetByIDsResponse, error) {
	if client.RemoveBinaryDataFromDatasetByIDsFunc == nil {
		return errtrace.Wrap2(client.DataServiceClient.RemoveBinaryDataFromDatasetByIDs(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.RemoveBinaryDataFromDatasetByIDsFunc(ctx, in, opts...))
}

// CreateSequence calls the injected CreateSequence or the real version.
func (client *DataServiceClient) CreateSequence(ctx context.Context, in *datapb.CreateSequenceRequest,
	opts ...grpc.CallOption,
) (*datapb.CreateSequenceResponse, error) {
	if client.CreateSequenceFunc == nil {
		return errtrace.Wrap2(client.DataServiceClient.CreateSequence(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.CreateSequenceFunc(ctx, in, opts...))
}

// GetSequence calls the injected GetSequence or the real version.
func (client *DataServiceClient) GetSequence(ctx context.Context, in *datapb.GetSequenceRequest,
	opts ...grpc.CallOption,
) (*datapb.GetSequenceResponse, error) {
	if client.GetSequenceFunc == nil {
		return errtrace.Wrap2(client.DataServiceClient.GetSequence(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.GetSequenceFunc(ctx, in, opts...))
}

// UpdateSequence calls the injected UpdateSequence or the real version.
func (client *DataServiceClient) UpdateSequence(ctx context.Context, in *datapb.UpdateSequenceRequest,
	opts ...grpc.CallOption,
) (*datapb.UpdateSequenceResponse, error) {
	if client.UpdateSequenceFunc == nil {
		return errtrace.Wrap2(client.DataServiceClient.UpdateSequence(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.UpdateSequenceFunc(ctx, in, opts...))
}

// DeleteSequence calls the injected DeleteSequence or the real version.
func (client *DataServiceClient) DeleteSequence(ctx context.Context, in *datapb.DeleteSequenceRequest,
	opts ...grpc.CallOption,
) (*datapb.DeleteSequenceResponse, error) {
	if client.DeleteSequenceFunc == nil {
		return errtrace.Wrap2(client.DataServiceClient.DeleteSequence(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.DeleteSequenceFunc(ctx, in, opts...))
}

// ListSequences calls the injected ListSequences or the real version.
func (client *DataServiceClient) ListSequences(ctx context.Context, in *datapb.ListSequencesRequest,
	opts ...grpc.CallOption,
) (*datapb.ListSequencesResponse, error) {
	if client.ListSequencesFunc == nil {
		return errtrace.Wrap2(client.DataServiceClient.ListSequences(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.ListSequencesFunc(ctx, in, opts...))
}

// AddSequencesToDataset calls the injected AddSequencesToDataset or the real version.
func (client *DataServiceClient) AddSequencesToDataset(ctx context.Context, in *datapb.AddSequencesToDatasetRequest,
	opts ...grpc.CallOption,
) (*datapb.AddSequencesToDatasetResponse, error) {
	if client.AddSequencesToDatasetFunc == nil {
		return errtrace.Wrap2(client.DataServiceClient.AddSequencesToDataset(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.AddSequencesToDatasetFunc(ctx, in, opts...))
}

// RemoveSequencesFromDataset calls the injected RemoveSequencesFromDataset or the real version.
func (client *DataServiceClient) RemoveSequencesFromDataset(ctx context.Context, in *datapb.RemoveSequencesFromDatasetRequest,
	opts ...grpc.CallOption,
) (*datapb.RemoveSequencesFromDatasetResponse, error) {
	if client.RemoveSequencesFromDatasetFunc == nil {
		return errtrace.Wrap2(client.DataServiceClient.RemoveSequencesFromDataset(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.RemoveSequencesFromDatasetFunc(ctx, in, opts...))
}

// SequencesByDatasetID calls the injected SequencesByDatasetID or the real version.
func (client *DataServiceClient) SequencesByDatasetID(ctx context.Context, in *datapb.SequencesByDatasetIDRequest,
	opts ...grpc.CallOption,
) (*datapb.SequencesByDatasetIDResponse, error) {
	if client.SequencesByDatasetIDFunc == nil {
		return errtrace.Wrap2(client.DataServiceClient.SequencesByDatasetID(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.SequencesByDatasetIDFunc(ctx, in, opts...))
}
