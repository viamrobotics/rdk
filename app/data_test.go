package app

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	datapb "go.viam.com/api/app/data/v1"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/test"
	utils "go.viam.com/utils/protoutils"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	componentName  = "component_name"
	componentType  = "component_type"
	method         = "method"
	robotName      = "robot_name"
	robotId        = "robot_id"
	partName       = "part_name"
	partId         = "part_id"
	locationId     = "location_id"
	organizationId = "organization_id"
	password       = "password"
	mimeType       = "mime_type"
	uri            = "some.robot.uri"
	bboxLabel      = "bbox_label"
	tag            = "tag"
	fileName       = "file_name"
	fileExt        = "file_ext.ext"
	datasetId      = "dataset_id"
	binaryMetaId   = "binary_id"
	mongodbUri     = "mongo_uri"
	hostName       = "host_name"
)

var (
	locationIds      = []string{locationId}
	orgIds           = []string{organizationId}
	mimeTypes        = []string{mimeType}
	bboxLabels       = []string{bboxLabel}
	methodParameters = map[string]string{}
	tags             = []string{tag}
	startTime        = time.Now().UTC().Round(time.Millisecond)
	endTime          = time.Now().UTC().Round(time.Millisecond)
	data             = map[string]interface{}{
		"key": "value",
	}
	binaryId = BinaryID{
		FileId:         "file1",
		OrganizationId: organizationId,
		LocationId:     locationId,
	}
	binaryIds      = []BinaryID{binaryId}
	binaryDataByte = []byte("BYTE")
	sqlQuery       = "SELECT * FROM readings WHERE organization_id='e76d1b3b-0468-4efd-bb7f-fb1d2b352fcb' LIMIT 1"
	rawData        = []map[string]any{
		{
			"key1": startTime,
			"key2": "2",
			"key3": []any{1, 2, 3},
			"key4": map[string]any{
				"key4sub1": endTime,
			},
			"key5": 4.05,
			"key6": []any{true, false, true},
			"key7": []any{
				map[string]any{
					"nestedKey1": "simpleValue",
				},
				map[string]any{
					"nestedKey2": startTime,
				},
			},
		},
	}
	datasetIds  = []string{datasetId}
	annotations = Annotations{
		Bboxes: []BoundingBox{
			{
				Id:             "bbox1",
				Label:          "label1",
				XMinNormalized: 0.1,
				YMinNormalized: 0.2,
				XMaxNormalized: 0.8,
				YMaxNormalized: 0.9,
			},
			{
				Id:             "bbox2",
				Label:          "label2",
				XMinNormalized: 0.15,
				YMinNormalized: 0.25,
				XMaxNormalized: 0.75,
				YMaxNormalized: 0.85,
			},
		},
	}
)

func createGrpcClient() *inject.DataServiceClient {
	return &inject.DataServiceClient{}
}
func TestDataClient(t *testing.T) {
	grpcClient := createGrpcClient()
	client := DataClient{client: grpcClient}

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
		RobotId:         robotId,
		PartName:        partName,
		PartId:          partId,
		LocationIds:     locationIds,
		OrganizationIds: orgIds,
		MimeType:        mimeTypes,
		Interval:        captureInterval,
		TagsFilter:      tagsFilter, //asterix or no??
		BboxLabels:      bboxLabels,
		DatasetId:       datasetId,
	}
	tabularMetadata := CaptureMetadata{
		OrganizationId:   organizationId,
		LocationId:       locationId,
		RobotName:        robotName,
		RobotId:          robotId,
		PartName:         partName,
		PartId:           partId,
		ComponentType:    componentType,
		ComponentName:    componentName,
		MethodName:       method,
		MethodParameters: methodParameters,
		Tags:             tags,
		MimeType:         mimeType,
	}
	binaryMetadata := BinaryMetadata{
		ID:              binaryMetaId,
		CaptureMetadata: tabularMetadata,
		TimeRequested:   startTime,
		TimeReceived:    endTime,
		FileName:        fileName,
		FileExt:         fileExt,
		URI:             uri,
		Annotations:     annotations,
		DatasetIDs:      datasetIds,
	}
	t.Run("TabularDataByFilter", func(t *testing.T) {
		countOnly := true
		includeInternalData := true
		dataRequest := DataRequest{
			Filter:    filter,
			Limit:     5,
			Last:      "last",
			SortOrder: Unspecified,
		}
		expectedTabularData := TabularData{
			Data:          data,
			MetadataIndex: 0,
			Metadata:      tabularMetadata,
			TimeRequested: startTime,
			TimeReceived:  endTime,
		}
		expectedCount := uint64(5)
		expectedLast := "last"
		expectedLimit := uint64(5)

		dataStruct, _ := utils.StructToStructPb(data)
		tabularDataPb := &datapb.TabularData{
			Data:          dataStruct,
			MetadataIndex: 0,
			TimeRequested: timestamppb.New(startTime),
			TimeReceived:  timestamppb.New(endTime),
		}

		grpcClient.TabularDataByFilterFunc = func(ctx context.Context, in *datapb.TabularDataByFilterRequest, opts ...grpc.CallOption) (*datapb.TabularDataByFilterResponse, error) {
			expectedDataReqProto, _ := DataRequestToProto(dataRequest)
			test.That(t, in.DataRequest, test.ShouldResemble, expectedDataReqProto)
			test.That(t, in.CountOnly, test.ShouldBeTrue)
			test.That(t, in.IncludeInternalData, test.ShouldBeTrue)
			return &datapb.TabularDataByFilterResponse{
				Data:     []*datapb.TabularData{tabularDataPb},
				Count:    expectedCount,
				Last:     expectedLast,
				Metadata: []*datapb.CaptureMetadata{CaptureMetadataToProto(tabularMetadata)}}, nil
		}
		respTabularData, respCount, respLast, _ := client.TabularDataByFilter(context.Background(), filter, expectedLimit, expectedLast, dataRequest.SortOrder, countOnly, includeInternalData)
		test.That(t, respTabularData[0], test.ShouldResemble, expectedTabularData)
		test.That(t, respCount, test.ShouldEqual, expectedCount)
		test.That(t, respLast, test.ShouldEqual, expectedLast)
	})

	t.Run("TabularDataBySQL", func(t *testing.T) {
		expectedOrgId := organizationId
		expectedSqlQuery := sqlQuery
		expectedRawData := rawData

		//convert rawData to BSON
		var expectedRawDataPb [][]byte
		for _, byte := range expectedRawData {
			bsonByte, err := bson.Marshal(byte)
			if err != nil {
				t.Fatalf("Failed to marshal expectedRawData: %v", err)
			}
			expectedRawDataPb = append(expectedRawDataPb, bsonByte)
		}
		grpcClient.TabularDataBySQLFunc = func(ctx context.Context, in *datapb.TabularDataBySQLRequest, opts ...grpc.CallOption) (*datapb.TabularDataBySQLResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, expectedOrgId)
			test.That(t, in.SqlQuery, test.ShouldResemble, expectedSqlQuery)
			return &datapb.TabularDataBySQLResponse{
				RawData: expectedRawDataPb,
			}, nil
		}
		response, _ := client.TabularDataBySQL(context.Background(), expectedOrgId, expectedSqlQuery)
		test.That(t, response, test.ShouldResemble, expectedRawData)
	})

	t.Run("TabularDataByMQL", func(t *testing.T) {
		expectedOrgId := organizationId
		matchStage := bson.M{"$match": bson.M{"organization_id": "e76d1b3b-0468-4efd-bb7f-fb1d2b352fcb"}}
		limitStage := bson.M{"$limit": 1}

		// convert to BSON byte arrays
		matchBytes, _ := bson.Marshal(matchStage)
		limitBytes, _ := bson.Marshal(limitStage)
		mqlbinary := [][]byte{matchBytes, limitBytes}
		expectedMqlBinary := mqlbinary
		expectedRawData := rawData

		//convert rawData to BSON
		var expectedRawDataPb [][]byte
		for _, byte := range expectedRawData {
			bsonByte, err := bson.Marshal(byte)
			if err != nil {
				t.Fatalf("Failed to marshal expectedRawData: %v", err)
			}
			expectedRawDataPb = append(expectedRawDataPb, bsonByte)
		}
		grpcClient.TabularDataByMQLFunc = func(ctx context.Context, in *datapb.TabularDataByMQLRequest, opts ...grpc.CallOption) (*datapb.TabularDataByMQLResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, expectedOrgId)
			test.That(t, in.MqlBinary, test.ShouldResemble, expectedMqlBinary)
			return &datapb.TabularDataByMQLResponse{
				RawData: expectedRawDataPb,
			}, nil
		}
		response, _ := client.TabularDataByMQL(context.Background(), expectedOrgId, expectedMqlBinary)
		test.That(t, response, test.ShouldResemble, expectedRawData)
	})

	t.Run("BinaryDataByFilter", func(t *testing.T) {
		includeBinary := true
		countOnly := true
		includeInternalData := true
		dataRequest := DataRequest{
			Filter:    filter,
			Limit:     5,
			Last:      "last",
			SortOrder: Unspecified,
		}
		expectedCount := uint64(5)
		expectedLast := "last"
		expectedBinaryData := BinaryData{
			Binary:   binaryDataByte,
			Metadata: binaryMetadata,
		}
		expectedBinaryDataPb := BinaryDataToProto(expectedBinaryData)

		grpcClient.BinaryDataByFilterFunc = func(ctx context.Context, in *datapb.BinaryDataByFilterRequest, opts ...grpc.CallOption) (*datapb.BinaryDataByFilterResponse, error) {
			expectedDataReq, _ := DataRequestToProto(dataRequest)
			test.That(t, in.DataRequest, test.ShouldResemble, expectedDataReq)
			test.That(t, in.IncludeBinary, test.ShouldBeTrue)
			test.That(t, in.CountOnly, test.ShouldBeTrue)
			test.That(t, in.IncludeInternalData, test.ShouldBeTrue)
			return &datapb.BinaryDataByFilterResponse{
				Data:  []*datapb.BinaryData{expectedBinaryDataPb},
				Count: expectedCount,
				Last:  expectedLast,
			}, nil
		}
		respBinaryData, respCount, respLast, _ := client.BinaryDataByFilter(context.Background(), filter, expectedCount, dataRequest.SortOrder, expectedLast, includeBinary, countOnly, includeInternalData)
		test.That(t, respBinaryData[0], test.ShouldResemble, expectedBinaryData)
		test.That(t, respCount, test.ShouldEqual, expectedCount)
		test.That(t, respLast, test.ShouldEqual, expectedLast)

	})
	t.Run("BinaryDataByIDs", func(t *testing.T) {
		expectedBinaryData := BinaryData{
			Binary:   binaryDataByte,
			Metadata: binaryMetadata,
		}
		grpcClient.BinaryDataByIDsFunc = func(ctx context.Context, in *datapb.BinaryDataByIDsRequest, opts ...grpc.CallOption) (*datapb.BinaryDataByIDsResponse, error) {
			test.That(t, in.IncludeBinary, test.ShouldBeTrue)
			test.That(t, in.BinaryIds, test.ShouldResemble, BinaryIdsToProto(binaryIds))
			expectedBinaryDataList := []*datapb.BinaryData{BinaryDataToProto(expectedBinaryData)}

			return &datapb.BinaryDataByIDsResponse{Data: expectedBinaryDataList, Count: uint64(len(expectedBinaryDataList))}, nil
		}
		respBinaryData, _ := client.BinaryDataByIDs(context.Background(), binaryIds)
		test.That(t, respBinaryData[0], test.ShouldResemble, expectedBinaryData)
	})

	t.Run("DeleteTabularData", func(t *testing.T) {
		deleteOlderThanDays := uint32(1)
		expectedOrgId := organizationId
		expectedDeleteOlderThanDays := deleteOlderThanDays
		expectedDeletedCount := uint64(5)

		grpcClient.DeleteTabularDataFunc = func(ctx context.Context, in *datapb.DeleteTabularDataRequest, opts ...grpc.CallOption) (*datapb.DeleteTabularDataResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, expectedOrgId)
			test.That(t, in.DeleteOlderThanDays, test.ShouldEqual, expectedDeleteOlderThanDays)

			return &datapb.DeleteTabularDataResponse{
				DeletedCount: expectedDeletedCount,
			}, nil
		}
		resp, _ := client.DeleteTabularData(context.Background(), organizationId, deleteOlderThanDays)
		test.That(t, resp, test.ShouldEqual, expectedDeletedCount)
	})

	t.Run("DeleteBinaryDataByFilter", func(t *testing.T) {
		expectedFilterPb := FilterToProto(filter)
		expectedDeletedCount := uint64(5)

		grpcClient.DeleteBinaryDataByFilterFunc = func(ctx context.Context, in *datapb.DeleteBinaryDataByFilterRequest, opts ...grpc.CallOption) (*datapb.DeleteBinaryDataByFilterResponse, error) {
			test.That(t, in.Filter, test.ShouldResemble, expectedFilterPb)
			test.That(t, in.IncludeInternalData, test.ShouldBeTrue)
			return &datapb.DeleteBinaryDataByFilterResponse{
				DeletedCount: expectedDeletedCount,
			}, nil
		}
		resp, _ := client.DeleteBinaryDataByFilter(context.Background(), filter)
		test.That(t, resp, test.ShouldEqual, expectedDeletedCount)
	})

	t.Run("DeleteBinaryDataByIDs", func(t *testing.T) {
		expectedDeletedCount := uint64(5)
		expectedBinaryIds := BinaryIdsToProto(binaryIds)
		grpcClient.DeleteBinaryDataByIDsFunc = func(ctx context.Context, in *datapb.DeleteBinaryDataByIDsRequest, opts ...grpc.CallOption) (*datapb.DeleteBinaryDataByIDsResponse, error) {
			test.That(t, in.BinaryIds, test.ShouldResemble, expectedBinaryIds)
			return &datapb.DeleteBinaryDataByIDsResponse{
				DeletedCount: expectedDeletedCount,
			}, nil
		}
		resp, _ := client.DeleteBinaryDataByIDs(context.Background(), binaryIds)
		test.That(t, resp, test.ShouldEqual, expectedDeletedCount)
	})

	t.Run("AddTagsToBinaryDataByIDs", func(t *testing.T) {
		expectedTags := tags
		expectedBinaryIds := BinaryIdsToProto(binaryIds)
		grpcClient.AddTagsToBinaryDataByIDsFunc = func(ctx context.Context, in *datapb.AddTagsToBinaryDataByIDsRequest, opts ...grpc.CallOption) (*datapb.AddTagsToBinaryDataByIDsResponse, error) {
			test.That(t, in.BinaryIds, test.ShouldResemble, expectedBinaryIds)
			test.That(t, in.Tags, test.ShouldResemble, expectedTags)
			return &datapb.AddTagsToBinaryDataByIDsResponse{}, nil
		}
		client.AddTagsToBinaryDataByIDs(context.Background(), tags, binaryIds)

	})

	t.Run("AddTagsToBinaryDataByFilter", func(t *testing.T) {
		expectedTags := tags
		expectedFilterPb := FilterToProto(filter)
		grpcClient.AddTagsToBinaryDataByFilterFunc = func(ctx context.Context, in *datapb.AddTagsToBinaryDataByFilterRequest, opts ...grpc.CallOption) (*datapb.AddTagsToBinaryDataByFilterResponse, error) {
			test.That(t, in.Filter, test.ShouldResemble, expectedFilterPb)
			test.That(t, in.Tags, test.ShouldResemble, expectedTags)
			return &datapb.AddTagsToBinaryDataByFilterResponse{}, nil
		}
		client.AddTagsToBinaryDataByFilter(context.Background(), tags, filter)
	})

	t.Run("RemoveTagsFromBinaryDataByIDs", func(t *testing.T) {
		expectedTags := tags
		expectedBinaryIds := BinaryIdsToProto(binaryIds)
		expectedDeletedCount := uint64(5)
		grpcClient.RemoveTagsFromBinaryDataByIDsFunc = func(ctx context.Context, in *datapb.RemoveTagsFromBinaryDataByIDsRequest, opts ...grpc.CallOption) (*datapb.RemoveTagsFromBinaryDataByIDsResponse, error) {
			test.That(t, in.BinaryIds, test.ShouldResemble, expectedBinaryIds)
			test.That(t, in.Tags, test.ShouldResemble, expectedTags)
			return &datapb.RemoveTagsFromBinaryDataByIDsResponse{
				DeletedCount: expectedDeletedCount,
			}, nil
		}
		resp, _ := client.RemoveTagsFromBinaryDataByIDs(context.Background(), tags, binaryIds)
		test.That(t, resp, test.ShouldEqual, expectedDeletedCount)
	})

	t.Run("RemoveTagsFromBinaryDataByFilter", func(t *testing.T) {
		expectedTags := tags
		expectedFilterPb := FilterToProto(filter)
		expectedDeletedCount := uint64(5)

		grpcClient.RemoveTagsFromBinaryDataByFilterFunc = func(ctx context.Context, in *datapb.RemoveTagsFromBinaryDataByFilterRequest, opts ...grpc.CallOption) (*datapb.RemoveTagsFromBinaryDataByFilterResponse, error) {
			test.That(t, in.Filter, test.ShouldResemble, expectedFilterPb)
			test.That(t, in.Tags, test.ShouldResemble, expectedTags)
			return &datapb.RemoveTagsFromBinaryDataByFilterResponse{
				DeletedCount: expectedDeletedCount,
			}, nil
		}
		resp, _ := client.RemoveTagsFromBinaryDataByFilter(context.Background(), tags, filter)
		test.That(t, resp, test.ShouldEqual, expectedDeletedCount)
	})

	t.Run("TagsByFilter", func(t *testing.T) {
		expectedTags := tags
		expectedFilterPb := FilterToProto(filter)

		grpcClient.TagsByFilterFunc = func(ctx context.Context, in *datapb.TagsByFilterRequest, opts ...grpc.CallOption) (*datapb.TagsByFilterResponse, error) {
			test.That(t, in.Filter, test.ShouldResemble, expectedFilterPb)
			return &datapb.TagsByFilterResponse{
				Tags: tags,
			}, nil
		}
		resp, _ := client.TagsByFilter(context.Background(), filter)
		test.That(t, resp, test.ShouldResemble, expectedTags)
	})

	t.Run("AddBoundingBoxToImageByID", func(t *testing.T) {
		expectedBinaryIdPb := BinaryIdToProto(binaryId)
		expectedLabel := bboxLabel
		expectedXMin := annotations.Bboxes[0].XMinNormalized
		expectedYMin := annotations.Bboxes[0].YMinNormalized
		expectedXMax := annotations.Bboxes[0].XMaxNormalized
		expectedYMax := annotations.Bboxes[0].YMaxNormalized
		expectedBBoxId := annotations.Bboxes[0].Id

		grpcClient.AddBoundingBoxToImageByIDFunc = func(ctx context.Context, in *datapb.AddBoundingBoxToImageByIDRequest, opts ...grpc.CallOption) (*datapb.AddBoundingBoxToImageByIDResponse, error) {
			test.That(t, in.BinaryId, test.ShouldResemble, expectedBinaryIdPb)
			test.That(t, in.Label, test.ShouldEqual, expectedLabel)
			test.That(t, in.XMinNormalized, test.ShouldEqual, expectedXMin)
			test.That(t, in.YMinNormalized, test.ShouldEqual, expectedYMin)
			test.That(t, in.XMaxNormalized, test.ShouldEqual, expectedXMax)
			test.That(t, in.YMaxNormalized, test.ShouldEqual, expectedYMax)

			return &datapb.AddBoundingBoxToImageByIDResponse{
				BboxId: expectedBBoxId,
			}, nil
		}
		resp, _ := client.AddBoundingBoxToImageByID(context.Background(), binaryId, bboxLabel, expectedXMin, expectedYMin, expectedXMax, expectedYMax)
		test.That(t, resp, test.ShouldResemble, expectedBBoxId)
	})

	t.Run("RemoveBoundingBoxFromImageByID", func(t *testing.T) {
		expectedBinaryIdPb := BinaryIdToProto(binaryId)
		expectedBBoxId := annotations.Bboxes[0].Id

		grpcClient.RemoveBoundingBoxFromImageByIDFunc = func(ctx context.Context, in *datapb.RemoveBoundingBoxFromImageByIDRequest, opts ...grpc.CallOption) (*datapb.RemoveBoundingBoxFromImageByIDResponse, error) {
			test.That(t, in.BinaryId, test.ShouldResemble, expectedBinaryIdPb)
			test.That(t, in.BboxId, test.ShouldEqual, expectedBBoxId)

			return &datapb.RemoveBoundingBoxFromImageByIDResponse{}, nil
		}
		client.RemoveBoundingBoxFromImageByID(context.Background(), expectedBBoxId, binaryId)

	})

	t.Run("BoundingBoxLabelsByFilter", func(t *testing.T) {
		expectedFilterPb := FilterToProto(filter)
		expectedBBoxLabels := []string{
			annotations.Bboxes[0].Label,
			annotations.Bboxes[1].Label,
		}
		annotationsPb := AnnotationsToProto(annotations)
		expectedBBoxLabelsPb := []string{
			annotationsPb.Bboxes[0].Label,
			annotationsPb.Bboxes[1].Label,
		}

		grpcClient.BoundingBoxLabelsByFilterFunc = func(ctx context.Context, in *datapb.BoundingBoxLabelsByFilterRequest, opts ...grpc.CallOption) (*datapb.BoundingBoxLabelsByFilterResponse, error) {
			test.That(t, in.Filter, test.ShouldResemble, expectedFilterPb)
			return &datapb.BoundingBoxLabelsByFilterResponse{
				Labels: expectedBBoxLabelsPb,
			}, nil
		}
		resp, _ := client.BoundingBoxLabelsByFilter(context.Background(), filter)
		test.That(t, resp, test.ShouldResemble, expectedBBoxLabels)
	})
	t.Run("UpdateBoundingBox", func(t *testing.T) {
		bBoxId := annotations.Bboxes[0].Id
		expectedBinaryIdPb := BinaryIdToProto(binaryId)

		annotationsPb := AnnotationsToProto(annotations)
		expectedLabel := annotationsPb.Bboxes[0].Label
		expectedBBoxIdPb := annotationsPb.Bboxes[0].Id
		expectedXMin := annotationsPb.Bboxes[0].XMinNormalized
		expectedYMin := annotationsPb.Bboxes[0].YMinNormalized
		expectedXMax := annotationsPb.Bboxes[0].XMaxNormalized
		expectedYMax := annotationsPb.Bboxes[0].YMaxNormalized

		grpcClient.UpdateBoundingBoxFunc = func(ctx context.Context, in *datapb.UpdateBoundingBoxRequest, opts ...grpc.CallOption) (*datapb.UpdateBoundingBoxResponse, error) {
			test.That(t, in.BinaryId, test.ShouldResemble, expectedBinaryIdPb)
			test.That(t, in.BboxId, test.ShouldResemble, expectedBBoxIdPb)
			test.That(t, *in.Label, test.ShouldEqual, expectedLabel)
			test.That(t, *in.XMinNormalized, test.ShouldEqual, expectedXMin)
			test.That(t, *in.YMinNormalized, test.ShouldEqual, expectedYMin)
			test.That(t, *in.XMaxNormalized, test.ShouldEqual, expectedXMax)
			test.That(t, *in.YMaxNormalized, test.ShouldEqual, expectedYMax)
			return &datapb.UpdateBoundingBoxResponse{}, nil
		}
		client.UpdateBoundingBox(context.Background(), binaryId, bBoxId, &expectedLabel, &expectedXMin, &expectedYMin, &expectedXMax, &expectedYMax)
	})

	t.Run("GetDatabaseConnection", func(t *testing.T) {
		expectedOrgId := organizationId
		expectedHostName := hostName
		expectedMongodbUri := mongodbUri
		expectedDbUser := true

		grpcClient.GetDatabaseConnectionFunc = func(ctx context.Context, in *datapb.GetDatabaseConnectionRequest, opts ...grpc.CallOption) (*datapb.GetDatabaseConnectionResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldResemble, expectedOrgId)
			return &datapb.GetDatabaseConnectionResponse{
				Hostname:        expectedHostName,
				MongodbUri:      expectedMongodbUri,
				HasDatabaseUser: expectedDbUser,
			}, nil
		}
		resp, _ := client.GetDatabaseConnection(context.Background(), organizationId)
		test.That(t, resp, test.ShouldResemble, expectedHostName)
	})

	t.Run("ConfigureDatabaseUser", func(t *testing.T) {
		expectedOrgId := organizationId
		expectedPassword := password

		grpcClient.ConfigureDatabaseUserFunc = func(ctx context.Context, in *datapb.ConfigureDatabaseUserRequest, opts ...grpc.CallOption) (*datapb.ConfigureDatabaseUserResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldResemble, expectedOrgId)
			test.That(t, in.Password, test.ShouldResemble, expectedPassword)
			return &datapb.ConfigureDatabaseUserResponse{}, nil
		}
		client.ConfigureDatabaseUser(context.Background(), organizationId, password)
	})

	t.Run("AddBinaryDataToDatasetByIDs", func(t *testing.T) {
		expectedBinaryIds := BinaryIdsToProto(binaryIds)
		expectedDataSetId := datasetId

		grpcClient.AddBinaryDataToDatasetByIDsFunc = func(ctx context.Context, in *datapb.AddBinaryDataToDatasetByIDsRequest, opts ...grpc.CallOption) (*datapb.AddBinaryDataToDatasetByIDsResponse, error) {
			test.That(t, in.BinaryIds, test.ShouldResemble, expectedBinaryIds)
			test.That(t, in.DatasetId, test.ShouldResemble, expectedDataSetId)
			return &datapb.AddBinaryDataToDatasetByIDsResponse{}, nil
		}
		client.AddBinaryDataToDatasetByIDs(context.Background(), binaryIds, datasetId)
	})

	t.Run("RemoveBinaryDataFromDatasetByIDs", func(t *testing.T) {
		expectedBinaryIds := BinaryIdsToProto(binaryIds)
		expectedDataSetId := datasetId

		grpcClient.RemoveBinaryDataFromDatasetByIDsFunc = func(ctx context.Context, in *datapb.RemoveBinaryDataFromDatasetByIDsRequest, opts ...grpc.CallOption) (*datapb.RemoveBinaryDataFromDatasetByIDsResponse, error) {
			test.That(t, in.BinaryIds, test.ShouldResemble, expectedBinaryIds)
			test.That(t, in.DatasetId, test.ShouldResemble, expectedDataSetId)
			return &datapb.RemoveBinaryDataFromDatasetByIDsResponse{}, nil
		}
		client.RemoveBinaryDataFromDatasetByIDs(context.Background(), binaryIds, datasetId)
	})
}
