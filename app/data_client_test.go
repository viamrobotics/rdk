package app

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	pb "go.viam.com/api/app/data/v1"
	datapipelinesPb "go.viam.com/api/app/datapipelines/v1"
	setPb "go.viam.com/api/app/dataset/v1"
	syncPb "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	utils "go.viam.com/utils/protoutils"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	password     = "password"
	mimeType     = "mime_type"
	uri          = "some.robot.uri"
	bboxLabel    = "bbox_label"
	binaryMetaID = "binary_id"
	mongodbURI   = "mongo_uri"
	last         = "last"
	fileID       = "file_id"
)

var (
	binaryDataType      = DataTypeBinarySensor
	tabularDataType     = DataTypeTabularSensor
	locationIDs         = []string{locationID}
	orgIDs              = []string{organizationID}
	mimeTypes           = []string{mimeType}
	bboxLabels          = []string{bboxLabel}
	methodParameters    = map[string]interface{}{}
	dataRequestTimes    = [2]time.Time{start, end}
	count               = 5
	countOnly           = true
	includeInternalData = true
	data                = map[string]interface{}{
		"key": "value",
	}
	fileName        = "file_name"
	fileExt         = ".file_ext.ext"
	componentName   = "component_name"
	componentType   = "component_type"
	method          = "method"
	tabularMetadata = CaptureMetadata{
		OrganizationID:   organizationID,
		LocationID:       locationID,
		RobotName:        robotName,
		RobotID:          robotID,
		PartName:         partName,
		PartID:           partID,
		ComponentType:    componentType,
		ComponentName:    componentName,
		MethodName:       method,
		MethodParameters: methodParameters,
		Tags:             tags,
		MimeType:         mimeType,
	}
	tabularData = TabularData{
		Data:          data,
		MetadataIndex: 0,
		Metadata:      &tabularMetadata,
		TimeRequested: start,
		TimeReceived:  end,
	}
	exportTabularResponse = &pb.ExportTabularDataResponse{
		PartId:           partID,
		ResourceName:     componentName,
		ResourceSubtype:  componentType,
		MethodName:       method,
		TimeCaptured:     timestamppb.Now(),
		OrganizationId:   organizationID,
		LocationId:       parentLocationID,
		RobotName:        robotName,
		RobotId:          robotID,
		PartName:         partName,
		MethodParameters: &structpb.Struct{},
		Tags:             tags,
		Payload:          &structpb.Struct{},
	}

	binaryDataID   = "binary_data_id"
	binaryDataIDs  = []string{binaryDataID}
	binaryDataByte = []byte("BYTE")
	sqlQuery       = "SELECT * FROM readings WHERE organization_id='e76d1b3b-0468-4efd-bb7f-fb1d2b352fcb' LIMIT 1"
	rawData        = []map[string]interface{}{
		{
			"key1": start,
			"key2": "2",
			"key3": []interface{}{1, 2, 3},
			"key4": map[string]interface{}{
				"key4sub1": end,
			},
			"key5": 4.05,
			"key6": []interface{}{true, false, true},
			"key7": []interface{}{
				map[string]interface{}{
					"nestedKey1": "simpleValue",
				},
				map[string]interface{}{
					"nestedKey2": start,
				},
			},
		},
	}
	datasetIDs = []string{datasetID}
	dataset    = Dataset{
		ID:             datasetID,
		Name:           name,
		OrganizationID: organizationID,
		TimeCreated:    &createdOn,
	}
	datasets   = []*Dataset{&dataset}
	pbDatasets = []*setPb.Dataset{
		{
			Id:             dataset.ID,
			Name:           dataset.Name,
			OrganizationId: dataset.OrganizationID,
			TimeCreated:    pbCreatedOn,
		},
	}
	annotations = Annotations{
		Bboxes: []*BoundingBox{
			{
				ID:             "bbox1",
				Label:          "label1",
				XMinNormalized: 0.1,
				YMinNormalized: 0.2,
				XMaxNormalized: 0.8,
				YMaxNormalized: 0.9,
			},
			{
				ID:             "bbox2",
				Label:          "label2",
				XMinNormalized: 0.15,
				YMinNormalized: 0.25,
				XMaxNormalized: 0.75,
				YMaxNormalized: 0.85,
			},
		},
		Classifications: []*Classification{},
	}

	dataPipelineID = "data_pipeline_id"
	dataPipeline   = DataPipeline{
		ID:             dataPipelineID,
		Name:           name,
		OrganizationID: organizationID,
		Schedule:       "0 0 * * *",
		MqlBinary:      [][]byte{[]byte("mql_binary")},
		Enabled:        true,
		CreatedOn:      time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2023, 1, 1, 12, 30, 0, 0, time.UTC),
		DataSourceType: TabularDataSourceTypeStandard,
	}

	pbDataSourceType = pb.TabularDataSourceType_TABULAR_DATA_SOURCE_TYPE_STANDARD
	pbDataPipeline   = &datapipelinesPb.DataPipeline{
		Id:             dataPipelineID,
		Name:           name,
		OrganizationId: organizationID,
		Schedule:       "0 0 * * *",
		MqlBinary:      [][]byte{[]byte("mql_binary")},
		Enabled:        true,
		CreatedOn:      timestamppb.New(time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)),
		UpdatedAt:      timestamppb.New(time.Date(2023, 1, 1, 12, 30, 0, 0, time.UTC)),
		DataSourceType: &pbDataSourceType,
	}
	pbDataPipelines    = []*datapipelinesPb.DataPipeline{pbDataPipeline}
	dataPipelines      = []*DataPipeline{&dataPipeline}
	pbDataPipelineRuns = []*datapipelinesPb.DataPipelineRun{
		{
			Id:            "run1",
			Status:        datapipelinesPb.DataPipelineRunStatus_DATA_PIPELINE_RUN_STATUS_STARTED,
			StartTime:     timestamppb.New(time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)),
			EndTime:       timestamppb.New(time.Date(2023, 1, 1, 12, 5, 0, 0, time.UTC)),
			DataStartTime: timestamppb.New(time.Date(2023, 1, 1, 11, 0, 0, 0, time.UTC)),
			DataEndTime:   timestamppb.New(time.Date(2023, 1, 1, 12, 30, 0, 0, time.UTC)),
		},
		{
			Id:            "run2",
			Status:        datapipelinesPb.DataPipelineRunStatus_DATA_PIPELINE_RUN_STATUS_FAILED,
			StartTime:     timestamppb.New(time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)),
			EndTime:       timestamppb.New(time.Date(2023, 1, 1, 12, 5, 0, 0, time.UTC)),
			DataStartTime: timestamppb.New(time.Date(2023, 1, 1, 11, 0, 0, 0, time.UTC)),
			DataEndTime:   timestamppb.New(time.Date(2023, 1, 1, 12, 30, 0, 0, time.UTC)),
			ErrorMessage:  "error message",
		},
	}
	dataPipelineRuns = []*DataPipelineRun{
		{
			ID:            "run1",
			Status:        DataPipelineRunStatusStarted,
			StartTime:     time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			EndTime:       time.Date(2023, 1, 1, 12, 5, 0, 0, time.UTC),
			DataStartTime: time.Date(2023, 1, 1, 11, 0, 0, 0, time.UTC),
			DataEndTime:   time.Date(2023, 1, 1, 12, 30, 0, 0, time.UTC),
		},
		{
			ID:            "run2",
			Status:        DataPipelineRunStatusFailed,
			StartTime:     time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			EndTime:       time.Date(2023, 1, 1, 12, 5, 0, 0, time.UTC),
			DataStartTime: time.Date(2023, 1, 1, 11, 0, 0, 0, time.UTC),
			DataEndTime:   time.Date(2023, 1, 1, 12, 30, 0, 0, time.UTC),
			ErrorMessage:  "error message",
		},
	}
)

func binaryDataToProto(binaryData BinaryData) *pb.BinaryData {
	return &pb.BinaryData{
		Binary:   binaryData.Binary,
		Metadata: binaryMetadataToProto(binaryData.Metadata),
	}
}

func captureMetadataToProto(metadata CaptureMetadata) *pb.CaptureMetadata {
	methodParams, err := protoutils.ConvertMapToProtoAny(metadata.MethodParameters)
	if err != nil {
		return nil
	}
	return &pb.CaptureMetadata{
		OrganizationId:   metadata.OrganizationID,
		LocationId:       metadata.LocationID,
		RobotName:        metadata.RobotName,
		RobotId:          metadata.RobotID,
		PartName:         metadata.PartName,
		PartId:           metadata.PartID,
		ComponentType:    metadata.ComponentType,
		ComponentName:    metadata.ComponentName,
		MethodName:       metadata.MethodName,
		MethodParameters: methodParams,
		Tags:             metadata.Tags,
		MimeType:         metadata.MimeType,
	}
}

func binaryMetadataToProto(binaryMetadata *BinaryMetadata) *pb.BinaryMetadata {
	return &pb.BinaryMetadata{
		Id:              binaryMetadata.ID,
		BinaryDataId:    binaryMetadata.BinaryDataID,
		CaptureMetadata: captureMetadataToProto(binaryMetadata.CaptureMetadata),
		TimeRequested:   timestamppb.New(binaryMetadata.TimeRequested),
		TimeReceived:    timestamppb.New(binaryMetadata.TimeReceived),
		FileName:        binaryMetadata.FileName,
		FileExt:         binaryMetadata.FileExt,
		Uri:             binaryMetadata.URI,
		Annotations:     annotationsToProto(binaryMetadata.Annotations),
		DatasetIds:      binaryMetadata.DatasetIDs,
	}
}

func dataRequestToProto(dataRequest DataRequest) *pb.DataRequest {
	return &pb.DataRequest{
		Filter:    filterToProto(&dataRequest.Filter),
		Limit:     uint64(dataRequest.Limit),
		Last:      dataRequest.Last,
		SortOrder: orderToProto(dataRequest.SortOrder),
	}
}

func createDataGrpcClient() *inject.DataServiceClient {
	return &inject.DataServiceClient{}
}

func createDataSyncGrpcClient() *inject.DataSyncServiceClient {
	return &inject.DataSyncServiceClient{}
}

func createDatasetGrpcClient() *inject.DatasetServiceClient {
	return &inject.DatasetServiceClient{}
}

func createDataPipelineGrpcClient() *inject.DataPipelinesServiceClient {
	return &inject.DataPipelinesServiceClient{}
}

func TestDataClient(t *testing.T) {
	grpcClient := createDataGrpcClient()
	client := DataClient{dataClient: grpcClient}

	captureInterval := CaptureInterval{
		Start: time.Now(),
		End:   time.Now(),
	}
	tagsFilter := TagsFilter{
		Type: TagsFilterTypeUnspecified,
		Tags: []string{"tag1", "tag2"},
	}

	filter := Filter{
		ComponentName:   componentName,
		ComponentType:   componentType,
		Method:          method,
		RobotName:       robotName,
		RobotID:         robotID,
		PartName:        partName,
		PartID:          partID,
		LocationIDs:     locationIDs,
		OrganizationIDs: orgIDs,
		MimeType:        mimeTypes,
		Interval:        captureInterval,
		TagsFilter:      tagsFilter,
		BboxLabels:      bboxLabels,
		DatasetID:       datasetID,
	}
	pbFilter := filterToProto(&filter)

	binaryMetadata := BinaryMetadata{
		ID:              binaryMetaID,
		BinaryDataID:    binaryDataID,
		CaptureMetadata: tabularMetadata,
		TimeRequested:   start,
		TimeReceived:    end,
		FileName:        fileName,
		FileExt:         fileExt,
		URI:             uri,
		Annotations:     &annotations,
		DatasetIDs:      datasetIDs,
	}

	dataRequest := DataRequest{
		Filter:    filter,
		Limit:     limit,
		Last:      last,
		SortOrder: Unspecified,
	}

	pbCount := uint64(count)

	binaryData := BinaryData{
		Binary:   binaryDataByte,
		Metadata: &binaryMetadata,
	}

	t.Run("TabularDataByFilter", func(t *testing.T) {
		dataStruct, _ := utils.StructToStructPb(data)
		//nolint:staticcheck
		tabularDataPb := &pb.TabularData{
			Data:          dataStruct,
			MetadataIndex: 0,
			TimeRequested: timestamppb.New(start),
			TimeReceived:  timestamppb.New(end),
		}
		//nolint:staticcheck
		grpcClient.TabularDataByFilterFunc = func(ctx context.Context, in *pb.TabularDataByFilterRequest,
			opts ...grpc.CallOption,
			//nolint:staticcheck
		) (*pb.TabularDataByFilterResponse, error) {
			test.That(t, in.DataRequest, test.ShouldResemble, dataRequestToProto(dataRequest))
			test.That(t, in.CountOnly, test.ShouldBeTrue)
			test.That(t, in.IncludeInternalData, test.ShouldBeTrue)
			//nolint:staticcheck
			return &pb.TabularDataByFilterResponse{
				//nolint:staticcheck
				Data:     []*pb.TabularData{tabularDataPb},
				Count:    pbCount,
				Last:     last,
				Metadata: []*pb.CaptureMetadata{captureMetadataToProto(tabularMetadata)},
			}, nil
		}
		resp, err := client.TabularDataByFilter(context.Background(), &DataByFilterOptions{
			&filter, limit, last, dataRequest.SortOrder, countOnly, includeInternalData,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.TabularData[0], test.ShouldResemble, &tabularData)
		test.That(t, resp.Count, test.ShouldEqual, count)
		test.That(t, resp.Last, test.ShouldEqual, last)
	})

	t.Run("TabularDataBySQL", func(t *testing.T) {
		// convert rawData to BSON
		var expectedRawDataPb [][]byte
		for _, byte := range rawData {
			bsonByte, err := bson.Marshal(byte)
			if err != nil {
				t.Fatalf("Failed to marshal expectedRawData: %v", err)
			}
			expectedRawDataPb = append(expectedRawDataPb, bsonByte)
		}
		grpcClient.TabularDataBySQLFunc = func(ctx context.Context, in *pb.TabularDataBySQLRequest,
			opts ...grpc.CallOption,
		) (*pb.TabularDataBySQLResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			test.That(t, in.SqlQuery, test.ShouldResemble, sqlQuery)
			return &pb.TabularDataBySQLResponse{
				RawData: expectedRawDataPb,
			}, nil
		}
		response, err := client.TabularDataBySQL(context.Background(), organizationID, sqlQuery)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, response, test.ShouldResemble, rawData)
	})

	t.Run("TabularDataByMQL", func(t *testing.T) {
		// convert to BSON byte arrays
		matchQuery := bson.M{"$match": bson.M{"organization_id": "e76d1b3b-0468-4efd-bb7f-fb1d2b352fcb"}}
		matchBytes, _ := bson.Marshal(matchQuery)
		limitQuery := bson.M{"$limit": 1}
		limitBytes, _ := bson.Marshal(limitQuery)
		mqlQueries := []map[string]interface{}{matchQuery, limitQuery}
		mqlBinary := [][]byte{matchBytes, limitBytes}
		queryPrefixName := "prefix_name"

		// convert rawData to BSON
		var expectedRawDataPb [][]byte
		for _, byte := range rawData {
			bsonByte, err := bson.Marshal(byte)
			if err != nil {
				t.Fatalf("Failed to marshal expectedRawData: %v", err)
			}
			expectedRawDataPb = append(expectedRawDataPb, bsonByte)
		}
		grpcClient.TabularDataByMQLFunc = func(ctx context.Context, in *pb.TabularDataByMQLRequest,
			opts ...grpc.CallOption,
		) (*pb.TabularDataByMQLResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			test.That(t, in.MqlBinary, test.ShouldResemble, mqlBinary)
			if in.DataSource != nil {
				test.That(t, in.DataSource.Type, test.ShouldNotEqual, pb.TabularDataSourceType_TABULAR_DATA_SOURCE_TYPE_UNSPECIFIED)
			}
			if in.QueryPrefixName != nil {
				test.That(t, *in.QueryPrefixName, test.ShouldEqual, queryPrefixName)
			}
			return &pb.TabularDataByMQLResponse{
				RawData: expectedRawDataPb,
			}, nil
		}
		response, err := client.TabularDataByMQL(context.Background(), organizationID, mqlQueries, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, response, test.ShouldResemble, rawData)
		response, err = client.TabularDataByMQL(context.Background(), organizationID, mqlQueries, &TabularDataByMQLOptions{
			TabularDataSourceType: TabularDataSourceTypeStandard,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, response, test.ShouldResemble, rawData)
		response, err = client.TabularDataByMQL(context.Background(), organizationID, mqlQueries, &TabularDataByMQLOptions{
			TabularDataSourceType: TabularDataSourceTypeHotStorage,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, response, test.ShouldResemble, rawData)
		response, err = client.TabularDataByMQL(context.Background(), organizationID, mqlQueries, &TabularDataByMQLOptions{
			QueryPrefixName: queryPrefixName,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, response, test.ShouldResemble, rawData)
	})

	t.Run("GetLatestTabularData", func(t *testing.T) {
		dataStruct, _ := utils.StructToStructPb(data)
		latestTabularData := GetLatestTabularDataResponse{
			TimeCaptured: start,
			TimeSynced:   end,
			Payload:      dataStruct.AsMap(),
		}

		grpcClient.GetLatestTabularDataFunc = func(ctx context.Context, in *pb.GetLatestTabularDataRequest,
			opts ...grpc.CallOption,
		) (*pb.GetLatestTabularDataResponse, error) {
			test.That(t, in.PartId, test.ShouldEqual, partID)
			test.That(t, in.ResourceName, test.ShouldEqual, componentName)
			test.That(t, in.ResourceSubtype, test.ShouldEqual, componentType)
			test.That(t, in.MethodName, test.ShouldResemble, method)
			return &pb.GetLatestTabularDataResponse{
				TimeCaptured: timestamppb.New(start),
				TimeSynced:   timestamppb.New(end),
				Payload:      dataStruct,
			}, nil
		}

		resp, err := client.GetLatestTabularData(context.Background(), partID, componentName, componentType, method, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &latestTabularData)
	})

	t.Run("ExportTabularData", func(t *testing.T) {
		sentOnce := false
		mockStream := &inject.DataServiceExportTabularDataClient{
			RecvFunc: func() (*pb.ExportTabularDataResponse, error) {
				if sentOnce {
					return nil, io.EOF
				}

				sentOnce = true
				return exportTabularResponse, nil
			},
		}

		grpcClient.ExportTabularDataFunc = func(ctx context.Context, in *pb.ExportTabularDataRequest,
			opts ...grpc.CallOption,
		) (pb.DataService_ExportTabularDataClient, error) {
			test.That(t, in.PartId, test.ShouldEqual, partID)
			test.That(t, in.ResourceName, test.ShouldEqual, componentName)
			test.That(t, in.ResourceSubtype, test.ShouldEqual, componentType)
			test.That(t, in.MethodName, test.ShouldEqual, method)
			test.That(t, in.Interval, test.ShouldResemble, captureIntervalToProto(captureInterval))
			return mockStream, nil
		}

		responses, err := client.ExportTabularData(context.Background(), partID, componentName, componentType, method, captureInterval, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, responses[0], test.ShouldResemble, exportTabularDataResponseFromProto(exportTabularResponse))
	})

	t.Run("BinaryDataByFilter", func(t *testing.T) {
		includeBinary := true
		grpcClient.BinaryDataByFilterFunc = func(ctx context.Context, in *pb.BinaryDataByFilterRequest,
			opts ...grpc.CallOption,
		) (*pb.BinaryDataByFilterResponse, error) {
			expectedDataReq := dataRequestToProto(dataRequest)
			test.That(t, in.DataRequest, test.ShouldResemble, expectedDataReq)
			test.That(t, in.IncludeBinary, test.ShouldBeTrue)
			test.That(t, in.CountOnly, test.ShouldBeTrue)
			test.That(t, in.IncludeInternalData, test.ShouldBeTrue)
			return &pb.BinaryDataByFilterResponse{
				Data:  []*pb.BinaryData{binaryDataToProto(binaryData)},
				Count: pbCount,
				Last:  last,
			}, nil
		}
		resp, err := client.BinaryDataByFilter(
			context.Background(), includeBinary, &DataByFilterOptions{
				&filter, limit, last, dataRequest.SortOrder, countOnly, includeInternalData,
			})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.BinaryData[0], test.ShouldResemble, &binaryData)
		test.That(t, resp.Count, test.ShouldEqual, count)
		test.That(t, resp.Last, test.ShouldEqual, last)
	})

	t.Run("BinaryDataByIDs", func(t *testing.T) {
		t.Run("default behavior (backward compatible)", func(t *testing.T) {
			grpcClient.BinaryDataByIDsFunc = func(ctx context.Context, in *pb.BinaryDataByIDsRequest,
				opts ...grpc.CallOption,
			) (*pb.BinaryDataByIDsResponse, error) {
				test.That(t, in.IncludeBinary, test.ShouldBeTrue)
				test.That(t, in.BinaryDataIds, test.ShouldResemble, binaryDataIDs)
				expectedBinaryDataList := []*pb.BinaryData{binaryDataToProto(binaryData)}

				return &pb.BinaryDataByIDsResponse{Data: expectedBinaryDataList, Count: uint64(len(expectedBinaryDataList))}, nil
			}
			respBinaryData, err := client.BinaryDataByIDs(context.Background(), binaryDataIDs)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, respBinaryData[0], test.ShouldResemble, &binaryData)
		})

		t.Run("with IncludeBinary true", func(t *testing.T) {
			grpcClient.BinaryDataByIDsFunc = func(ctx context.Context, in *pb.BinaryDataByIDsRequest,
				opts ...grpc.CallOption,
			) (*pb.BinaryDataByIDsResponse, error) {
				test.That(t, in.IncludeBinary, test.ShouldBeTrue)
				test.That(t, in.BinaryDataIds, test.ShouldResemble, binaryDataIDs)
				expectedBinaryDataList := []*pb.BinaryData{binaryDataToProto(binaryData)}

				return &pb.BinaryDataByIDsResponse{Data: expectedBinaryDataList, Count: uint64(len(expectedBinaryDataList))}, nil
			}
			respBinaryData, err := client.BinaryDataByIDs(context.Background(), binaryDataIDs, &BinaryDataByIDsOptions{IncludeBinary: true})
			test.That(t, err, test.ShouldBeNil)
			test.That(t, respBinaryData[0], test.ShouldResemble, &binaryData)
		})

		t.Run("with IncludeBinary false", func(t *testing.T) {
			grpcClient.BinaryDataByIDsFunc = func(ctx context.Context, in *pb.BinaryDataByIDsRequest,
				opts ...grpc.CallOption,
			) (*pb.BinaryDataByIDsResponse, error) {
				test.That(t, in.IncludeBinary, test.ShouldBeFalse)
				test.That(t, in.BinaryDataIds, test.ShouldResemble, binaryDataIDs)
				expectedBinaryDataList := []*pb.BinaryData{binaryDataToProto(binaryData)}

				return &pb.BinaryDataByIDsResponse{Data: expectedBinaryDataList, Count: uint64(len(expectedBinaryDataList))}, nil
			}
			respBinaryData, err := client.BinaryDataByIDs(context.Background(), binaryDataIDs, &BinaryDataByIDsOptions{IncludeBinary: false})
			test.That(t, err, test.ShouldBeNil)
			test.That(t, respBinaryData[0], test.ShouldResemble, &binaryData)
		})
	})

	t.Run("CreateBinaryDataSignedURL", func(t *testing.T) {
		expectedSignedURL := "https://example.com/signed-url?token=abc123"
		expirationMinutes := uint32(60)
		grpcClient.CreateBinaryDataSignedURLFunc = func(ctx context.Context, in *pb.CreateBinaryDataSignedURLRequest,
			opts ...grpc.CallOption,
		) (*pb.CreateBinaryDataSignedURLResponse, error) {
			test.That(t, in.BinaryDataId, test.ShouldEqual, binaryDataID)
			test.That(t, in.ExpirationMinutes, test.ShouldNotBeNil)
			test.That(t, *in.ExpirationMinutes, test.ShouldEqual, expirationMinutes)
			return &pb.CreateBinaryDataSignedURLResponse{
				SignedUrl: expectedSignedURL,
			}, nil
		}
		resp, err := client.CreateBinaryDataSignedURL(context.Background(), binaryDataID, expirationMinutes)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldEqual, expectedSignedURL)
	})

	t.Run("DeleteTabularData", func(t *testing.T) {
		deleteOlderThanDays := 1
		pbDeleteOlderThanDays := uint32(deleteOlderThanDays)
		grpcClient.DeleteTabularDataFunc = func(ctx context.Context, in *pb.DeleteTabularDataRequest,
			opts ...grpc.CallOption,
		) (*pb.DeleteTabularDataResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			test.That(t, in.DeleteOlderThanDays, test.ShouldEqual, pbDeleteOlderThanDays)

			return &pb.DeleteTabularDataResponse{
				DeletedCount: pbCount,
			}, nil
		}
		resp, err := client.DeleteTabularData(context.Background(), organizationID, deleteOlderThanDays)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldEqual, count)
	})

	t.Run("DeleteBinaryDataByFilter", func(t *testing.T) {
		grpcClient.DeleteBinaryDataByFilterFunc = func(ctx context.Context, in *pb.DeleteBinaryDataByFilterRequest,
			opts ...grpc.CallOption,
		) (*pb.DeleteBinaryDataByFilterResponse, error) {
			test.That(t, in.Filter, test.ShouldResemble, pbFilter)
			test.That(t, in.IncludeInternalData, test.ShouldBeTrue)
			return &pb.DeleteBinaryDataByFilterResponse{
				DeletedCount: pbCount,
			}, nil
		}
		resp, err := client.DeleteBinaryDataByFilter(context.Background(), &filter)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldEqual, count)
	})

	t.Run("DeleteBinaryDataByIDs", func(t *testing.T) {
		grpcClient.DeleteBinaryDataByIDsFunc = func(ctx context.Context, in *pb.DeleteBinaryDataByIDsRequest,
			opts ...grpc.CallOption,
		) (*pb.DeleteBinaryDataByIDsResponse, error) {
			test.That(t, in.BinaryDataIds, test.ShouldResemble, binaryDataIDs)
			return &pb.DeleteBinaryDataByIDsResponse{
				DeletedCount: pbCount,
			}, nil
		}
		resp, err := client.DeleteBinaryDataByIDs(context.Background(), binaryDataIDs)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldEqual, count)
	})

	t.Run("AddTagsToBinaryDataByIDs", func(t *testing.T) {
		grpcClient.AddTagsToBinaryDataByIDsFunc = func(ctx context.Context, in *pb.AddTagsToBinaryDataByIDsRequest,
			opts ...grpc.CallOption,
		) (*pb.AddTagsToBinaryDataByIDsResponse, error) {
			test.That(t, in.BinaryDataIds, test.ShouldResemble, binaryDataIDs)
			test.That(t, in.Tags, test.ShouldResemble, tags)
			return &pb.AddTagsToBinaryDataByIDsResponse{}, nil
		}
		err := client.AddTagsToBinaryDataByIDs(context.Background(), tags, binaryDataIDs)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("AddTagsToBinaryDataByFilter", func(t *testing.T) {
		//nolint:staticcheck
		grpcClient.AddTagsToBinaryDataByFilterFunc = func(ctx context.Context, in *pb.AddTagsToBinaryDataByFilterRequest,
			opts ...grpc.CallOption,
			//nolint:staticcheck
		) (*pb.AddTagsToBinaryDataByFilterResponse, error) {
			test.That(t, in.Filter, test.ShouldResemble, pbFilter)
			test.That(t, in.Tags, test.ShouldResemble, tags)
			//nolint:staticcheck
			return &pb.AddTagsToBinaryDataByFilterResponse{}, nil
		}

		err := client.AddTagsToBinaryDataByFilter(context.Background(), tags, &filter)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("RemoveTagsFromBinaryDataByIDs", func(t *testing.T) {
		grpcClient.RemoveTagsFromBinaryDataByIDsFunc = func(ctx context.Context, in *pb.RemoveTagsFromBinaryDataByIDsRequest,
			opts ...grpc.CallOption,
		) (*pb.RemoveTagsFromBinaryDataByIDsResponse, error) {
			test.That(t, in.BinaryDataIds, test.ShouldResemble, binaryDataIDs)
			test.That(t, in.Tags, test.ShouldResemble, tags)
			return &pb.RemoveTagsFromBinaryDataByIDsResponse{
				DeletedCount: pbCount,
			}, nil
		}
		resp, err := client.RemoveTagsFromBinaryDataByIDs(context.Background(), tags, binaryDataIDs)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldEqual, count)
	})

	t.Run("RemoveTagsFromBinaryDataByFilter", func(t *testing.T) {
		//nolint:staticcheck
		grpcClient.RemoveTagsFromBinaryDataByFilterFunc = func(ctx context.Context, in *pb.RemoveTagsFromBinaryDataByFilterRequest,
			opts ...grpc.CallOption,
			//nolint:staticcheck
		) (*pb.RemoveTagsFromBinaryDataByFilterResponse, error) {
			test.That(t, in.Filter, test.ShouldResemble, pbFilter)
			test.That(t, in.Tags, test.ShouldResemble, tags)
			//nolint:staticcheck
			return &pb.RemoveTagsFromBinaryDataByFilterResponse{
				DeletedCount: pbCount,
			}, nil
		}

		resp, err := client.RemoveTagsFromBinaryDataByFilter(context.Background(), tags, &filter)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldEqual, count)
	})

	t.Run("AddBoundingBoxToImageByID", func(t *testing.T) {
		grpcClient.AddBoundingBoxToImageByIDFunc = func(ctx context.Context,
			in *pb.AddBoundingBoxToImageByIDRequest,
			opts ...grpc.CallOption,
		) (*pb.AddBoundingBoxToImageByIDResponse, error) {
			test.That(t, in.BinaryDataId, test.ShouldResemble, binaryDataID)
			test.That(t, in.Label, test.ShouldEqual, bboxLabel)
			test.That(t, in.XMinNormalized, test.ShouldEqual, annotations.Bboxes[0].XMinNormalized)
			test.That(t, in.YMinNormalized, test.ShouldEqual, annotations.Bboxes[0].YMinNormalized)
			test.That(t, in.XMaxNormalized, test.ShouldEqual, annotations.Bboxes[0].XMaxNormalized)
			test.That(t, in.YMaxNormalized, test.ShouldEqual, annotations.Bboxes[0].YMaxNormalized)

			return &pb.AddBoundingBoxToImageByIDResponse{
				BboxId: annotations.Bboxes[0].ID,
			}, nil
		}
		resp, err := client.AddBoundingBoxToImageByID(
			context.Background(), binaryDataID, bboxLabel, annotations.Bboxes[0].XMinNormalized,
			annotations.Bboxes[0].YMinNormalized, annotations.Bboxes[0].XMaxNormalized, annotations.Bboxes[0].YMaxNormalized)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, annotations.Bboxes[0].ID)
	})

	t.Run("RemoveBoundingBoxFromImageByID", func(t *testing.T) {
		grpcClient.RemoveBoundingBoxFromImageByIDFunc = func(ctx context.Context, in *pb.RemoveBoundingBoxFromImageByIDRequest,
			opts ...grpc.CallOption,
		) (*pb.RemoveBoundingBoxFromImageByIDResponse, error) {
			test.That(t, in.BinaryDataId, test.ShouldResemble, binaryDataID)
			test.That(t, in.BboxId, test.ShouldEqual, annotations.Bboxes[0].ID)

			return &pb.RemoveBoundingBoxFromImageByIDResponse{}, nil
		}
		err := client.RemoveBoundingBoxFromImageByID(context.Background(), annotations.Bboxes[0].ID, binaryDataID)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("BoundingBoxLabelsByFilter", func(t *testing.T) {
		expectedBBoxLabels := []string{
			annotations.Bboxes[0].Label,
			annotations.Bboxes[1].Label,
		}
		expectedBBoxLabelsPb := []string{
			annotationsToProto(&annotations).Bboxes[0].Label,
			annotationsToProto(&annotations).Bboxes[1].Label,
		}
		//nolint:staticcheck
		grpcClient.BoundingBoxLabelsByFilterFunc = func(ctx context.Context, in *pb.BoundingBoxLabelsByFilterRequest,
			opts ...grpc.CallOption,
			//nolint:staticcheck
		) (*pb.BoundingBoxLabelsByFilterResponse, error) {
			test.That(t, in.Filter, test.ShouldResemble, pbFilter)
			//nolint:staticcheck
			return &pb.BoundingBoxLabelsByFilterResponse{
				Labels: expectedBBoxLabelsPb,
			}, nil
		}
		resp, err := client.BoundingBoxLabelsByFilter(context.Background(), &filter)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, expectedBBoxLabels)
	})
	t.Run("UpdateBoundingBox", func(t *testing.T) {
		annotationsPb := annotationsToProto(&annotations)
		grpcClient.UpdateBoundingBoxFunc = func(ctx context.Context, in *pb.UpdateBoundingBoxRequest,
			opts ...grpc.CallOption,
		) (*pb.UpdateBoundingBoxResponse, error) {
			test.That(t, in.BinaryDataId, test.ShouldResemble, binaryDataID)
			test.That(t, in.BboxId, test.ShouldResemble, annotationsPb.Bboxes[0].Id)
			test.That(t, *in.Label, test.ShouldEqual, annotationsPb.Bboxes[0].Label)
			test.That(t, *in.XMinNormalized, test.ShouldEqual, annotationsPb.Bboxes[0].XMinNormalized)
			test.That(t, *in.YMinNormalized, test.ShouldEqual, annotationsPb.Bboxes[0].YMinNormalized)
			test.That(t, *in.XMaxNormalized, test.ShouldEqual, annotationsPb.Bboxes[0].XMaxNormalized)
			test.That(t, *in.YMaxNormalized, test.ShouldEqual, annotationsPb.Bboxes[0].YMaxNormalized)
			return &pb.UpdateBoundingBoxResponse{}, nil
		}
		err := client.UpdateBoundingBox(context.Background(), binaryDataID, annotations.Bboxes[0].ID, &UpdateBoundingBoxOptions{
			&annotationsPb.Bboxes[0].Label,
			&annotationsPb.Bboxes[0].XMinNormalized,
			&annotationsPb.Bboxes[0].YMinNormalized,
			&annotationsPb.Bboxes[0].XMaxNormalized,
			&annotationsPb.Bboxes[0].YMaxNormalized,
		})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("GetDatabaseConnection", func(t *testing.T) {
		grpcClient.GetDatabaseConnectionFunc = func(ctx context.Context, in *pb.GetDatabaseConnectionRequest,
			opts ...grpc.CallOption,
		) (*pb.GetDatabaseConnectionResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldResemble, organizationID)
			return &pb.GetDatabaseConnectionResponse{
				Hostname:        host,
				MongodbUri:      mongodbURI,
				HasDatabaseUser: true,
			}, nil
		}
		resp, err := client.GetDatabaseConnection(context.Background(), organizationID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Hostname, test.ShouldResemble, host)
		test.That(t, resp.MongodbURI, test.ShouldResemble, mongodbURI)
		test.That(t, resp.HasDatabaseUser, test.ShouldBeTrue)
	})

	t.Run("ConfigureDatabaseUser", func(t *testing.T) {
		grpcClient.ConfigureDatabaseUserFunc = func(ctx context.Context, in *pb.ConfigureDatabaseUserRequest,
			opts ...grpc.CallOption,
		) (*pb.ConfigureDatabaseUserResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldResemble, organizationID)
			test.That(t, in.Password, test.ShouldResemble, password)
			return &pb.ConfigureDatabaseUserResponse{}, nil
		}
		err := client.ConfigureDatabaseUser(context.Background(), organizationID, password)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("AddBinaryDataToDatasetByIDs", func(t *testing.T) {
		grpcClient.AddBinaryDataToDatasetByIDsFunc = func(ctx context.Context, in *pb.AddBinaryDataToDatasetByIDsRequest,
			opts ...grpc.CallOption,
		) (*pb.AddBinaryDataToDatasetByIDsResponse, error) {
			test.That(t, in.BinaryDataIds, test.ShouldResemble, binaryDataIDs)
			test.That(t, in.DatasetId, test.ShouldResemble, datasetID)
			return &pb.AddBinaryDataToDatasetByIDsResponse{}, nil
		}
		err := client.AddBinaryDataToDatasetByIDs(context.Background(), binaryDataIDs, datasetID)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("RemoveBinaryDataFromDatasetByIDs", func(t *testing.T) {
		grpcClient.RemoveBinaryDataFromDatasetByIDsFunc = func(ctx context.Context, in *pb.RemoveBinaryDataFromDatasetByIDsRequest,
			opts ...grpc.CallOption,
		) (*pb.RemoveBinaryDataFromDatasetByIDsResponse, error) {
			test.That(t, in.BinaryDataIds, test.ShouldResemble, binaryDataIDs)
			test.That(t, in.DatasetId, test.ShouldResemble, datasetID)
			return &pb.RemoveBinaryDataFromDatasetByIDsResponse{}, nil
		}
		err := client.RemoveBinaryDataFromDatasetByIDs(context.Background(), binaryDataIDs, datasetID)
		test.That(t, err, test.ShouldBeNil)
	})
}

func TestDataSyncClient(t *testing.T) {
	grpcClient := createDataSyncGrpcClient()
	client := DataClient{dataSyncClient: grpcClient}

	uploadMetadata := UploadMetadata{
		PartID:           partID,
		ComponentType:    componentType,
		ComponentName:    componentName,
		MethodName:       method,
		Type:             DataTypeBinarySensor,
		FileName:         fileName,
		MethodParameters: methodParameters,
		FileExtension:    fileExt,
		Tags:             tags,
	}

	t.Run("BinaryDataCaptureUpload", func(t *testing.T) {
		uploadMetadata.Type = DataTypeBinarySensor
		options := BinaryDataCaptureUploadOptions{
			Type:             &binaryDataType,
			FileName:         &fileName,
			MethodParameters: methodParameters,
			Tags:             tags,
			DataRequestTimes: &dataRequestTimes,
		}
		grpcClient.DataCaptureUploadFunc = func(ctx context.Context, in *syncPb.DataCaptureUploadRequest,
			opts ...grpc.CallOption,
		) (*syncPb.DataCaptureUploadResponse, error) {
			methodParams, _ := protoutils.ConvertMapToProtoAny(methodParameters)

			test.That(t, in.Metadata.PartId, test.ShouldEqual, partID)
			test.That(t, in.Metadata.ComponentType, test.ShouldEqual, componentType)
			test.That(t, in.Metadata.ComponentName, test.ShouldEqual, componentName)
			test.That(t, in.Metadata.MethodName, test.ShouldEqual, method)
			test.That(t, in.Metadata.Type, test.ShouldEqual, binaryDataType)
			test.That(t, in.Metadata.FileName, test.ShouldEqual, fileName)
			test.That(t, in.Metadata.MethodParameters, test.ShouldResemble, methodParams)
			test.That(t, in.Metadata.FileExtension, test.ShouldEqual, fileExt)
			test.That(t, in.Metadata.Tags, test.ShouldResemble, tags)

			test.That(t, in.SensorContents[0].Metadata.TimeRequested, test.ShouldResemble, timestamppb.New(start))
			test.That(t, in.SensorContents[0].Metadata.TimeReceived, test.ShouldResemble, timestamppb.New(end))
			dataField, ok := in.SensorContents[0].Data.(*syncPb.SensorData_Binary)
			test.That(t, ok, test.ShouldBeTrue)
			test.That(t, dataField.Binary, test.ShouldResemble, binaryDataByte)
			return &syncPb.DataCaptureUploadResponse{
				BinaryDataId: binaryDataID,
			}, nil
		}
		resp, err := client.BinaryDataCaptureUpload(context.Background(),
			binaryDataByte, partID, componentType, componentName,
			method, fileExt, &options)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, binaryDataID)
	})

	t.Run("TabularDataCaptureUpload", func(t *testing.T) {
		uploadMetadata.Type = DataTypeTabularSensor
		dataStruct, _ := utils.StructToStructPb(data)
		//nolint:staticcheck
		tabularDataPb := &pb.TabularData{
			Data:          dataStruct,
			MetadataIndex: 0,
			TimeRequested: timestamppb.New(start),
			TimeReceived:  timestamppb.New(end),
		}
		options := TabularDataCaptureUploadOptions{
			Type:             &binaryDataType,
			FileName:         &fileName,
			MethodParameters: methodParameters,
			FileExtension:    &fileExt,
			Tags:             tags,
		}
		grpcClient.DataCaptureUploadFunc = func(ctx context.Context, in *syncPb.DataCaptureUploadRequest,
			opts ...grpc.CallOption,
		) (*syncPb.DataCaptureUploadResponse, error) {
			methodParams, _ := protoutils.ConvertMapToProtoAny(methodParameters)

			test.That(t, in.Metadata.PartId, test.ShouldEqual, partID)
			test.That(t, in.Metadata.ComponentType, test.ShouldEqual, componentType)
			test.That(t, in.Metadata.ComponentName, test.ShouldEqual, componentName)
			test.That(t, in.Metadata.MethodName, test.ShouldEqual, method)
			test.That(t, in.Metadata.Type, test.ShouldEqual, tabularDataType)
			test.That(t, in.Metadata.FileName, test.ShouldEqual, fileName)
			test.That(t, in.Metadata.MethodParameters, test.ShouldResemble, methodParams)
			test.That(t, in.Metadata.FileExtension, test.ShouldEqual, fileExt)
			test.That(t, in.Metadata.Tags, test.ShouldResemble, tags)

			test.That(t, in.SensorContents[0].Metadata.TimeRequested, test.ShouldResemble, timestamppb.New(start))
			test.That(t, in.SensorContents[0].Metadata.TimeReceived, test.ShouldResemble, timestamppb.New(end))
			dataField, ok := in.SensorContents[0].Data.(*syncPb.SensorData_Struct)
			test.That(t, ok, test.ShouldBeTrue)
			test.That(t, dataField.Struct, test.ShouldResemble, tabularDataPb.Data)
			return &syncPb.DataCaptureUploadResponse{
				FileId: fileID,
			}, nil
		}
		tabularData := []map[string]interface{}{data}
		dataRequestTimes := [][2]time.Time{
			{start, end},
		}
		resp, err := client.TabularDataCaptureUpload(context.Background(),
			tabularData, partID, componentType, componentName, method,
			dataRequestTimes, &options)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, fileID)
	})

	t.Run("StreamingDataCaptureUpload", func(t *testing.T) {
		options := StreamingDataCaptureUploadOptions{
			ComponentType:    &componentType,
			ComponentName:    &componentName,
			MethodName:       &method,
			Type:             &binaryDataType,
			FileName:         &fileName,
			MethodParameters: methodParameters,
			Tags:             tags,
			DataRequestTimes: &dataRequestTimes,
		}
		// Mock implementation of the streaming client.
		mockStream := &inject.DataSyncServiceStreamingDataCaptureUploadClient{
			SendFunc: func(req *syncPb.StreamingDataCaptureUploadRequest) error {
				switch packet := req.UploadPacket.(type) {
				case *syncPb.StreamingDataCaptureUploadRequest_Metadata:
					meta := packet.Metadata
					test.That(t, meta.UploadMetadata.PartId, test.ShouldEqual, partID)
					test.That(t, meta.UploadMetadata.FileExtension, test.ShouldEqual, fileExt)
					test.That(t, meta.UploadMetadata.ComponentType, test.ShouldEqual, componentType)
					test.That(t, meta.UploadMetadata.ComponentName, test.ShouldEqual, componentName)
					test.That(t, meta.UploadMetadata.MethodName, test.ShouldEqual, method)
					test.That(t, meta.UploadMetadata.Tags, test.ShouldResemble, tags)
					test.That(t, meta.SensorMetadata.TimeRequested, test.ShouldResemble, timestamppb.New(start))
					test.That(t, meta.SensorMetadata.TimeReceived, test.ShouldResemble, timestamppb.New(end))
				case *syncPb.StreamingDataCaptureUploadRequest_Data:
					test.That(t, packet.Data, test.ShouldResemble, binaryDataByte)
				default:
					t.Errorf("unexpected packet type: %T", packet)
				}
				return nil
			},
			CloseAndRecvFunc: func() (*syncPb.StreamingDataCaptureUploadResponse, error) {
				return &syncPb.StreamingDataCaptureUploadResponse{
					BinaryDataId: binaryDataID,
				}, nil
			},
		}
		grpcClient.StreamingDataCaptureUploadFunc = func(ctx context.Context,
			opts ...grpc.CallOption,
		) (syncPb.DataSyncService_StreamingDataCaptureUploadClient, error) {
			return mockStream, nil
		}
		resp, err := client.StreamingDataCaptureUpload(context.Background(), binaryDataByte, partID, fileExt, &options)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldEqual, binaryDataID)
	})
	t.Run("FileUploadFromBytes", func(t *testing.T) {
		options := FileUploadOptions{
			ComponentType:    &componentType,
			ComponentName:    &componentName,
			MethodName:       &method,
			FileName:         &fileName,
			MethodParameters: methodParameters,
			FileExtension:    &fileExt,
			Tags:             tags,
		}
		// Mock implementation of the streaming client.
		//nolint:dupl
		mockStream := &inject.DataSyncServiceFileUploadClient{
			SendFunc: func(req *syncPb.FileUploadRequest) error {
				switch packet := req.UploadPacket.(type) {
				case *syncPb.FileUploadRequest_Metadata:
					methodParams, _ := protoutils.ConvertMapToProtoAny(methodParameters)
					meta := packet.Metadata
					test.That(t, meta.PartId, test.ShouldEqual, partID)
					test.That(t, meta.ComponentType, test.ShouldEqual, componentType)
					test.That(t, meta.ComponentName, test.ShouldEqual, componentName)
					test.That(t, meta.MethodName, test.ShouldEqual, method)
					test.That(t, meta.Type, test.ShouldEqual, DataTypeFile)
					test.That(t, meta.FileName, test.ShouldEqual, fileName)
					test.That(t, meta.MethodParameters, test.ShouldResemble, methodParams)
					test.That(t, meta.FileExtension, test.ShouldEqual, fileExt)
					test.That(t, meta.Tags, test.ShouldResemble, tags)
				case *syncPb.FileUploadRequest_FileContents:
					test.That(t, packet.FileContents.Data, test.ShouldResemble, binaryDataByte)
				default:
					t.Errorf("unexpected packet type: %T", packet)
				}
				return nil
			},
			CloseAndRecvFunc: func() (*syncPb.FileUploadResponse, error) {
				return &syncPb.FileUploadResponse{
					BinaryDataId: binaryDataID,
				}, nil
			},
		}
		grpcClient.FileUploadFunc = func(ctx context.Context,
			opts ...grpc.CallOption,
		) (syncPb.DataSyncService_FileUploadClient, error) {
			return mockStream, nil
		}
		resp, err := client.FileUploadFromBytes(context.Background(), partID, binaryDataByte, &options)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldEqual, binaryDataID)
	})

	t.Run("FileUploadFromPath", func(t *testing.T) {
		options := FileUploadOptions{
			ComponentType:    &componentType,
			ComponentName:    &componentName,
			MethodName:       &method,
			FileName:         &fileName,
			MethodParameters: methodParameters,
			FileExtension:    &fileExt,
			Tags:             tags,
		}
		// Create a temporary file for testing
		tempContent := []byte("test file content")
		tempFile, err := os.CreateTemp("", "test-upload-*.txt")
		test.That(t, err, test.ShouldBeNil)
		defer os.Remove(tempFile.Name())
		// Mock implementation of the streaming client.
		//nolint:dupl
		mockStream := &inject.DataSyncServiceFileUploadClient{
			SendFunc: func(req *syncPb.FileUploadRequest) error {
				switch packet := req.UploadPacket.(type) {
				case *syncPb.FileUploadRequest_Metadata:
					methodParams, _ := protoutils.ConvertMapToProtoAny(methodParameters)
					meta := packet.Metadata
					test.That(t, meta.PartId, test.ShouldEqual, partID)
					test.That(t, meta.ComponentType, test.ShouldEqual, componentType)
					test.That(t, meta.ComponentName, test.ShouldEqual, componentName)
					test.That(t, meta.MethodName, test.ShouldEqual, method)
					test.That(t, meta.Type, test.ShouldEqual, DataTypeFile)
					test.That(t, meta.FileName, test.ShouldEqual, fileName)
					test.That(t, meta.MethodParameters, test.ShouldResemble, methodParams)
					test.That(t, meta.FileExtension, test.ShouldEqual, fileExt)
					test.That(t, meta.Tags, test.ShouldResemble, tags)
				case *syncPb.FileUploadRequest_FileContents:
					test.That(t, packet.FileContents.Data, test.ShouldResemble, tempContent)
				default:
					t.Errorf("unexpected packet type: %T", packet)
				}
				return nil
			},
			CloseAndRecvFunc: func() (*syncPb.FileUploadResponse, error) {
				return &syncPb.FileUploadResponse{
					BinaryDataId: binaryDataID,
				}, nil
			},
		}
		grpcClient.FileUploadFunc = func(ctx context.Context,
			opts ...grpc.CallOption,
		) (syncPb.DataSyncService_FileUploadClient, error) {
			return mockStream, nil
		}
		resp, err := client.FileUploadFromPath(context.Background(), partID, tempFile.Name(), &options)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldEqual, binaryDataID)
	})
}

func TestDatasetClient(t *testing.T) {
	grpcClient := createDatasetGrpcClient()
	client := DataClient{datasetClient: grpcClient}

	t.Run("CreateDataset", func(t *testing.T) {
		grpcClient.CreateDatasetFunc = func(
			ctx context.Context, in *setPb.CreateDatasetRequest, opts ...grpc.CallOption,
		) (*setPb.CreateDatasetResponse, error) {
			test.That(t, in.Name, test.ShouldEqual, name)
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			return &setPb.CreateDatasetResponse{
				Id: datasetID,
			}, nil
		}
		resp, err := client.CreateDataset(context.Background(), name, organizationID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldEqual, datasetID)
	})

	t.Run("DeleteDataset", func(t *testing.T) {
		grpcClient.DeleteDatasetFunc = func(
			ctx context.Context, in *setPb.DeleteDatasetRequest, opts ...grpc.CallOption,
		) (*setPb.DeleteDatasetResponse, error) {
			test.That(t, in.Id, test.ShouldEqual, datasetID)
			return &setPb.DeleteDatasetResponse{}, nil
		}
		err := client.DeleteDataset(context.Background(), datasetID)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("RenameDataset", func(t *testing.T) {
		grpcClient.RenameDatasetFunc = func(
			ctx context.Context, in *setPb.RenameDatasetRequest, opts ...grpc.CallOption,
		) (*setPb.RenameDatasetResponse, error) {
			test.That(t, in.Id, test.ShouldEqual, datasetID)
			test.That(t, in.Name, test.ShouldEqual, name)
			return &setPb.RenameDatasetResponse{}, nil
		}
		err := client.RenameDataset(context.Background(), datasetID, name)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("ListDatasetsByOrganizationID", func(t *testing.T) {
		grpcClient.ListDatasetsByOrganizationIDFunc = func(
			ctx context.Context, in *setPb.ListDatasetsByOrganizationIDRequest, opts ...grpc.CallOption,
		) (*setPb.ListDatasetsByOrganizationIDResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			return &setPb.ListDatasetsByOrganizationIDResponse{
				Datasets: pbDatasets,
			}, nil
		}
		resp, err := client.ListDatasetsByOrganizationID(context.Background(), organizationID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, &resp, test.ShouldResemble, &datasets)
	})

	t.Run("ListDatasetsByIDs", func(t *testing.T) {
		grpcClient.ListDatasetsByIDsFunc = func(
			ctx context.Context, in *setPb.ListDatasetsByIDsRequest, opts ...grpc.CallOption,
		) (*setPb.ListDatasetsByIDsResponse, error) {
			test.That(t, in.Ids, test.ShouldResemble, datasetIDs)
			return &setPb.ListDatasetsByIDsResponse{
				Datasets: pbDatasets,
			}, nil
		}
		resp, err := client.ListDatasetsByIDs(context.Background(), datasetIDs)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, &resp, test.ShouldResemble, &datasets)
	})
}

func TestDataPipelineClient(t *testing.T) {
	grpcClient := createDataPipelineGrpcClient()
	client := DataClient{datapipelinesClient: grpcClient}

	matchQuery := bson.M{"$match": bson.M{"organization_id": "e76d1b3b-0468-4efd-bb7f-fb1d2b352fcb"}}
	matchBytes, _ := bson.Marshal(matchQuery)
	limitQuery := bson.M{"$limit": 1}
	limitBytes, _ := bson.Marshal(limitQuery)
	mqlQueries := []map[string]interface{}{matchQuery, limitQuery}
	mqlBinary := [][]byte{matchBytes, limitBytes}

	t.Run("ListDataPipelines", func(t *testing.T) {
		grpcClient.ListDataPipelinesFunc = func(
			ctx context.Context, in *datapipelinesPb.ListDataPipelinesRequest, opts ...grpc.CallOption,
		) (*datapipelinesPb.ListDataPipelinesResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			return &datapipelinesPb.ListDataPipelinesResponse{
				DataPipelines: pbDataPipelines,
			}, nil
		}

		resp, err := client.ListDataPipelines(context.Background(), organizationID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, &resp, test.ShouldResemble, &dataPipelines)
	})

	t.Run("GetDataPipeline", func(t *testing.T) {
		grpcClient.GetDataPipelineFunc = func(
			ctx context.Context, in *datapipelinesPb.GetDataPipelineRequest, opts ...grpc.CallOption,
		) (*datapipelinesPb.GetDataPipelineResponse, error) {
			test.That(t, in.Id, test.ShouldEqual, dataPipelineID)
			return &datapipelinesPb.GetDataPipelineResponse{
				DataPipeline: pbDataPipeline,
			}, nil
		}
		resp, err := client.GetDataPipeline(context.Background(), dataPipelineID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &dataPipeline)
	})

	t.Run("CreateDataPipeline", func(t *testing.T) {
		grpcClient.CreateDataPipelineFunc = func(
			ctx context.Context, in *datapipelinesPb.CreateDataPipelineRequest, opts ...grpc.CallOption,
		) (*datapipelinesPb.CreateDataPipelineResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			test.That(t, in.Name, test.ShouldEqual, name)
			test.That(t, in.MqlBinary, test.ShouldResemble, mqlBinary)
			test.That(t, in.Schedule, test.ShouldEqual, "0 9 * * *")
			test.That(t, *in.EnableBackfill, test.ShouldBeTrue)
			test.That(t, *in.DataSourceType, test.ShouldEqual, pb.TabularDataSourceType_TABULAR_DATA_SOURCE_TYPE_STANDARD)
			return &datapipelinesPb.CreateDataPipelineResponse{
				Id: "new-data-pipeline-id",
			}, nil
		}
		options := &CreateDataPipelineOptions{
			TabularDataSourceType: TabularDataSourceTypeStandard,
		}
		resp, err := client.CreateDataPipeline(context.Background(), organizationID, name, mqlQueries, "0 9 * * *", true, options)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldEqual, "new-data-pipeline-id")
	})

	t.Run("RenameDataPipeline", func(t *testing.T) {
		grpcClient.RenameDataPipelineFunc = func(
			ctx context.Context, in *datapipelinesPb.RenameDataPipelineRequest, opts ...grpc.CallOption,
		) (*datapipelinesPb.RenameDataPipelineResponse, error) {
			test.That(t, in.Id, test.ShouldEqual, dataPipelineID)
			test.That(t, in.Name, test.ShouldEqual, name)
			return &datapipelinesPb.RenameDataPipelineResponse{}, nil
		}
		err := client.RenameDataPipeline(context.Background(), dataPipelineID, name)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("DeleteDataPipeline", func(t *testing.T) {
		grpcClient.DeleteDataPipelineFunc = func(
			ctx context.Context, in *datapipelinesPb.DeleteDataPipelineRequest, opts ...grpc.CallOption,
		) (*datapipelinesPb.DeleteDataPipelineResponse, error) {
			test.That(t, in.Id, test.ShouldEqual, dataPipelineID)
			return &datapipelinesPb.DeleteDataPipelineResponse{}, nil
		}
		err := client.DeleteDataPipeline(context.Background(), dataPipelineID)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("EnableDataPipeline", func(t *testing.T) {
		grpcClient.EnableDataPipelineFunc = func(
			ctx context.Context, in *datapipelinesPb.EnableDataPipelineRequest, opts ...grpc.CallOption,
		) (*datapipelinesPb.EnableDataPipelineResponse, error) {
			test.That(t, in.Id, test.ShouldEqual, dataPipelineID)
			return &datapipelinesPb.EnableDataPipelineResponse{}, nil
		}
		err := client.EnableDataPipeline(context.Background(), dataPipelineID)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("DisableDataPipeline", func(t *testing.T) {
		grpcClient.DisableDataPipelineFunc = func(
			ctx context.Context, in *datapipelinesPb.DisableDataPipelineRequest, opts ...grpc.CallOption,
		) (*datapipelinesPb.DisableDataPipelineResponse, error) {
			test.That(t, in.Id, test.ShouldEqual, dataPipelineID)
			return &datapipelinesPb.DisableDataPipelineResponse{}, nil
		}
		err := client.DisableDataPipeline(context.Background(), dataPipelineID)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("ListDataPipelineRuns", func(t *testing.T) {
		grpcClient.ListDataPipelineRunsFunc = func(
			ctx context.Context, in *datapipelinesPb.ListDataPipelineRunsRequest, opts ...grpc.CallOption,
		) (*datapipelinesPb.ListDataPipelineRunsResponse, error) {
			test.That(t, in.Id, test.ShouldEqual, dataPipelineID)
			test.That(t, in.PageSize, test.ShouldEqual, limit)
			return &datapipelinesPb.ListDataPipelineRunsResponse{
				Runs:          pbDataPipelineRuns,
				NextPageToken: "next1",
			}, nil
		}

		resp, err := client.ListDataPipelineRuns(context.Background(), dataPipelineID, uint32(limit))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, &resp.Runs, test.ShouldResemble, &dataPipelineRuns)

		grpcClient.ListDataPipelineRunsFunc = func(
			ctx context.Context, in *datapipelinesPb.ListDataPipelineRunsRequest, opts ...grpc.CallOption,
		) (*datapipelinesPb.ListDataPipelineRunsResponse, error) {
			test.That(t, in.Id, test.ShouldEqual, dataPipelineID)
			test.That(t, in.PageSize, test.ShouldEqual, limit)
			test.That(t, in.PageToken, test.ShouldEqual, "next1")
			return &datapipelinesPb.ListDataPipelineRunsResponse{
				Runs:          pbDataPipelineRuns,
				NextPageToken: "next2",
			}, nil
		}
		resp, err = resp.NextPage(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, &resp.Runs, test.ShouldResemble, &dataPipelineRuns)
	})
}
