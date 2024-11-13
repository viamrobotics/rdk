package data

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	datapb "go.viam.com/api/app/data/v1"
	"go.viam.com/test"
	utils "go.viam.com/utils/protoutils"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/testutils/inject"
)

const (
	componentName  = "component_name"
	componentType  = "component_type"
	method         = "method"
	robotName      = "robot_name"
	robotID        = "robot_id"
	partName       = "part_name"
	partID         = "part_id"
	locationID     = "location_id"
	organizationID = "organization_id"
	password       = "password"
	mimeType       = "mime_type"
	uri            = "some.robot.uri"
	bboxLabel      = "bbox_label"
	tag            = "tag"
	fileName       = "file_name"
	fileExt        = "file_ext.ext"
	datasetID      = "dataset_id"
	binaryMetaID   = "binary_id"
	mongodbURI     = "mongo_uri"
	hostName       = "host_name"
)

var (
	locationIDs      = []string{locationID}
	orgIDs           = []string{organizationID}
	mimeTypes        = []string{mimeType}
	bboxLabels       = []string{bboxLabel}
	methodParameters = map[string]string{}
	tags             = []string{tag}
	startTime        = time.Now().UTC().Round(time.Millisecond)
	endTime          = time.Now().UTC().Round(time.Millisecond)
	data             = map[string]interface{}{
		"key": "value",
	}
	binaryID = BinaryID{
		FileID:         "file1",
		OrganizationID: organizationID,
		LocationID:     locationID,
	}
	binaryIDs      = []BinaryID{binaryID}
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
	datasetIDs  = []string{datasetID}
	annotations = Annotations{
		Bboxes: []BoundingBox{
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
	}
)

func createGrpcClient() *inject.DataServiceClient {
	return &inject.DataServiceClient{}
}

func TestDataClient(t *testing.T) {
	grpcClient := createGrpcClient()
	client := Client{client: grpcClient}

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
		TagsFilter:      tagsFilter, // asterix or no??
		BboxLabels:      bboxLabels,
		DatasetID:       datasetID,
	}
	tabularMetadata := CaptureMetadata{
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
	binaryMetadata := BinaryMetadata{
		ID:              binaryMetaID,
		CaptureMetadata: tabularMetadata,
		TimeRequested:   startTime,
		TimeReceived:    endTime,
		FileName:        fileName,
		FileExt:         fileExt,
		URI:             uri,
		Annotations:     annotations,
		DatasetIDs:      datasetIDs,
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

		grpcClient.TabularDataByFilterFunc = func(ctx context.Context, in *datapb.TabularDataByFilterRequest,
			opts ...grpc.CallOption,
		) (*datapb.TabularDataByFilterResponse, error) {
			expectedDataReqProto := dataRequestToProto(dataRequest)
			test.That(t, in.DataRequest, test.ShouldResemble, expectedDataReqProto)
			test.That(t, in.CountOnly, test.ShouldBeTrue)
			test.That(t, in.IncludeInternalData, test.ShouldBeTrue)
			return &datapb.TabularDataByFilterResponse{
				Data:     []*datapb.TabularData{tabularDataPb},
				Count:    expectedCount,
				Last:     expectedLast,
				Metadata: []*datapb.CaptureMetadata{captureMetadataToProto(tabularMetadata)},
			}, nil
		}
		respTabularData, respCount, respLast, _ := client.TabularDataByFilter(
			context.Background(), filter, expectedLimit, expectedLast,
			dataRequest.SortOrder, countOnly, includeInternalData)
		test.That(t, respTabularData[0], test.ShouldResemble, expectedTabularData)
		test.That(t, respCount, test.ShouldEqual, expectedCount)
		test.That(t, respLast, test.ShouldEqual, expectedLast)
	})

	t.Run("TabularDataBySQL", func(t *testing.T) {
		expectedOrgID := organizationID
		expectedSQLQuery := sqlQuery
		expectedRawData := rawData

		// convert rawData to BSON
		var expectedRawDataPb [][]byte
		for _, byte := range expectedRawData {
			bsonByte, err := bson.Marshal(byte)
			if err != nil {
				t.Fatalf("Failed to marshal expectedRawData: %v", err)
			}
			expectedRawDataPb = append(expectedRawDataPb, bsonByte)
		}
		grpcClient.TabularDataBySQLFunc = func(ctx context.Context, in *datapb.TabularDataBySQLRequest,
			opts ...grpc.CallOption,
		) (*datapb.TabularDataBySQLResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, expectedOrgID)
			test.That(t, in.SqlQuery, test.ShouldResemble, expectedSQLQuery)
			return &datapb.TabularDataBySQLResponse{
				RawData: expectedRawDataPb,
			}, nil
		}
		response, _ := client.TabularDataBySQL(context.Background(), expectedOrgID, expectedSQLQuery)
		test.That(t, response, test.ShouldResemble, expectedRawData)
	})

	t.Run("TabularDataByMQL", func(t *testing.T) {
		expectedOrgID := organizationID
		matchStage := bson.M{"$match": bson.M{"organization_id": "e76d1b3b-0468-4efd-bb7f-fb1d2b352fcb"}}
		limitStage := bson.M{"$limit": 1}

		// convert to BSON byte arrays
		matchBytes, _ := bson.Marshal(matchStage)
		limitBytes, _ := bson.Marshal(limitStage)
		mqlbinary := [][]byte{matchBytes, limitBytes}
		expectedMqlBinary := mqlbinary
		expectedRawData := rawData

		// convert rawData to BSON
		var expectedRawDataPb [][]byte
		for _, byte := range expectedRawData {
			bsonByte, err := bson.Marshal(byte)
			if err != nil {
				t.Fatalf("Failed to marshal expectedRawData: %v", err)
			}
			expectedRawDataPb = append(expectedRawDataPb, bsonByte)
		}
		grpcClient.TabularDataByMQLFunc = func(ctx context.Context, in *datapb.TabularDataByMQLRequest,
			opts ...grpc.CallOption,
		) (*datapb.TabularDataByMQLResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, expectedOrgID)
			test.That(t, in.MqlBinary, test.ShouldResemble, expectedMqlBinary)
			return &datapb.TabularDataByMQLResponse{
				RawData: expectedRawDataPb,
			}, nil
		}
		response, _ := client.TabularDataByMQL(context.Background(), expectedOrgID, expectedMqlBinary)
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
		expectedBinaryDataPb := binaryDataToProto(expectedBinaryData)

		grpcClient.BinaryDataByFilterFunc = func(ctx context.Context, in *datapb.BinaryDataByFilterRequest,
			opts ...grpc.CallOption,
		) (*datapb.BinaryDataByFilterResponse, error) {
			expectedDataReq := dataRequestToProto(dataRequest)
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
		respBinaryData, respCount, respLast, _ := client.BinaryDataByFilter(
			context.Background(), filter, expectedCount, dataRequest.SortOrder,
			expectedLast, includeBinary, countOnly, includeInternalData)
		test.That(t, respBinaryData[0], test.ShouldResemble, expectedBinaryData)
		test.That(t, respCount, test.ShouldEqual, expectedCount)
		test.That(t, respLast, test.ShouldEqual, expectedLast)
	})
	t.Run("BinaryDataByIDs", func(t *testing.T) {
		expectedBinaryData := BinaryData{
			Binary:   binaryDataByte,
			Metadata: binaryMetadata,
		}
		grpcClient.BinaryDataByIDsFunc = func(ctx context.Context, in *datapb.BinaryDataByIDsRequest,
			opts ...grpc.CallOption,
		) (*datapb.BinaryDataByIDsResponse, error) {
			test.That(t, in.IncludeBinary, test.ShouldBeTrue)
			test.That(t, in.BinaryIds, test.ShouldResemble, binaryIDsToProto(binaryIDs))
			expectedBinaryDataList := []*datapb.BinaryData{binaryDataToProto(expectedBinaryData)}

			return &datapb.BinaryDataByIDsResponse{Data: expectedBinaryDataList, Count: uint64(len(expectedBinaryDataList))}, nil
		}
		respBinaryData, _ := client.BinaryDataByIDs(context.Background(), binaryIDs)
		test.That(t, respBinaryData[0], test.ShouldResemble, expectedBinaryData)
	})

	t.Run("DeleteTabularData", func(t *testing.T) {
		deleteOlderThanDays := uint32(1)
		expectedOrgID := organizationID
		expectedDeleteOlderThanDays := deleteOlderThanDays
		expectedDeletedCount := uint64(5)

		grpcClient.DeleteTabularDataFunc = func(ctx context.Context, in *datapb.DeleteTabularDataRequest,
			opts ...grpc.CallOption,
		) (*datapb.DeleteTabularDataResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, expectedOrgID)
			test.That(t, in.DeleteOlderThanDays, test.ShouldEqual, expectedDeleteOlderThanDays)

			return &datapb.DeleteTabularDataResponse{
				DeletedCount: expectedDeletedCount,
			}, nil
		}
		resp, _ := client.DeleteTabularData(context.Background(), organizationID, deleteOlderThanDays)
		test.That(t, resp, test.ShouldEqual, expectedDeletedCount)
	})

	t.Run("DeleteBinaryDataByFilter", func(t *testing.T) {
		expectedFilterPb := filterToProto(filter)
		expectedDeletedCount := uint64(5)

		grpcClient.DeleteBinaryDataByFilterFunc = func(ctx context.Context, in *datapb.DeleteBinaryDataByFilterRequest,
			opts ...grpc.CallOption,
		) (*datapb.DeleteBinaryDataByFilterResponse, error) {
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
		expectedBinaryIDs := binaryIDsToProto(binaryIDs)
		grpcClient.DeleteBinaryDataByIDsFunc = func(ctx context.Context, in *datapb.DeleteBinaryDataByIDsRequest,
			opts ...grpc.CallOption,
		) (*datapb.DeleteBinaryDataByIDsResponse, error) {
			test.That(t, in.BinaryIds, test.ShouldResemble, expectedBinaryIDs)
			return &datapb.DeleteBinaryDataByIDsResponse{
				DeletedCount: expectedDeletedCount,
			}, nil
		}
		resp, _ := client.DeleteBinaryDataByIDs(context.Background(), binaryIDs)
		test.That(t, resp, test.ShouldEqual, expectedDeletedCount)
	})

	t.Run("AddTagsToBinaryDataByIDs", func(t *testing.T) {
		expectedTags := tags
		expectedBinaryIDs := binaryIDsToProto(binaryIDs)
		grpcClient.AddTagsToBinaryDataByIDsFunc = func(ctx context.Context, in *datapb.AddTagsToBinaryDataByIDsRequest,
			opts ...grpc.CallOption,
		) (*datapb.AddTagsToBinaryDataByIDsResponse, error) {
			test.That(t, in.BinaryIds, test.ShouldResemble, expectedBinaryIDs)
			test.That(t, in.Tags, test.ShouldResemble, expectedTags)
			return &datapb.AddTagsToBinaryDataByIDsResponse{}, nil
		}
		client.AddTagsToBinaryDataByIDs(context.Background(), tags, binaryIDs)
	})

	t.Run("AddTagsToBinaryDataByFilter", func(t *testing.T) {
		expectedTags := tags
		expectedFilterPb := filterToProto(filter)
		grpcClient.AddTagsToBinaryDataByFilterFunc = func(ctx context.Context, in *datapb.AddTagsToBinaryDataByFilterRequest,
			opts ...grpc.CallOption,
		) (*datapb.AddTagsToBinaryDataByFilterResponse, error) {
			test.That(t, in.Filter, test.ShouldResemble, expectedFilterPb)
			test.That(t, in.Tags, test.ShouldResemble, expectedTags)
			return &datapb.AddTagsToBinaryDataByFilterResponse{}, nil
		}
		client.AddTagsToBinaryDataByFilter(context.Background(), tags, filter)
	})

	t.Run("RemoveTagsFromBinaryDataByIDs", func(t *testing.T) {
		expectedTags := tags
		expectedBinaryIDs := binaryIDsToProto(binaryIDs)
		expectedDeletedCount := uint64(5)
		grpcClient.RemoveTagsFromBinaryDataByIDsFunc = func(ctx context.Context, in *datapb.RemoveTagsFromBinaryDataByIDsRequest,
			opts ...grpc.CallOption,
		) (*datapb.RemoveTagsFromBinaryDataByIDsResponse, error) {
			test.That(t, in.BinaryIds, test.ShouldResemble, expectedBinaryIDs)
			test.That(t, in.Tags, test.ShouldResemble, expectedTags)
			return &datapb.RemoveTagsFromBinaryDataByIDsResponse{
				DeletedCount: expectedDeletedCount,
			}, nil
		}
		resp, _ := client.RemoveTagsFromBinaryDataByIDs(context.Background(), tags, binaryIDs)
		test.That(t, resp, test.ShouldEqual, expectedDeletedCount)
	})

	t.Run("RemoveTagsFromBinaryDataByFilter", func(t *testing.T) {
		expectedTags := tags
		expectedFilterPb := filterToProto(filter)
		expectedDeletedCount := uint64(5)

		grpcClient.RemoveTagsFromBinaryDataByFilterFunc = func(ctx context.Context, in *datapb.RemoveTagsFromBinaryDataByFilterRequest,
			opts ...grpc.CallOption,
		) (*datapb.RemoveTagsFromBinaryDataByFilterResponse, error) {
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
		expectedFilterPb := filterToProto(filter)

		grpcClient.TagsByFilterFunc = func(ctx context.Context, in *datapb.TagsByFilterRequest,
			opts ...grpc.CallOption,
		) (*datapb.TagsByFilterResponse, error) {
			test.That(t, in.Filter, test.ShouldResemble, expectedFilterPb)
			return &datapb.TagsByFilterResponse{
				Tags: tags,
			}, nil
		}
		resp, _ := client.TagsByFilter(context.Background(), filter)
		test.That(t, resp, test.ShouldResemble, expectedTags)
	})

	t.Run("AddBoundingBoxToImageByID", func(t *testing.T) {
		expectedBinaryIDPb := binaryIDToProto(binaryID)
		expectedLabel := bboxLabel
		expectedXMin := annotations.Bboxes[0].XMinNormalized
		expectedYMin := annotations.Bboxes[0].YMinNormalized
		expectedXMax := annotations.Bboxes[0].XMaxNormalized
		expectedYMax := annotations.Bboxes[0].YMaxNormalized
		expectedBBoxID := annotations.Bboxes[0].ID

		grpcClient.AddBoundingBoxToImageByIDFunc = func(ctx context.Context,
			in *datapb.AddBoundingBoxToImageByIDRequest,
			opts ...grpc.CallOption,
		) (*datapb.AddBoundingBoxToImageByIDResponse, error) {
			test.That(t, in.BinaryId, test.ShouldResemble, expectedBinaryIDPb)
			test.That(t, in.Label, test.ShouldEqual, expectedLabel)
			test.That(t, in.XMinNormalized, test.ShouldEqual, expectedXMin)
			test.That(t, in.YMinNormalized, test.ShouldEqual, expectedYMin)
			test.That(t, in.XMaxNormalized, test.ShouldEqual, expectedXMax)
			test.That(t, in.YMaxNormalized, test.ShouldEqual, expectedYMax)

			return &datapb.AddBoundingBoxToImageByIDResponse{
				BboxId: expectedBBoxID,
			}, nil
		}
		resp, _ := client.AddBoundingBoxToImageByID(
			context.Background(), binaryID, bboxLabel, expectedXMin,
			expectedYMin, expectedXMax, expectedYMax)
		test.That(t, resp, test.ShouldResemble, expectedBBoxID)
	})

	t.Run("RemoveBoundingBoxFromImageByID", func(t *testing.T) {
		expectedBinaryIDPb := binaryIDToProto(binaryID)
		expectedBBoxID := annotations.Bboxes[0].ID

		grpcClient.RemoveBoundingBoxFromImageByIDFunc = func(ctx context.Context, in *datapb.RemoveBoundingBoxFromImageByIDRequest,
			opts ...grpc.CallOption,
		) (*datapb.RemoveBoundingBoxFromImageByIDResponse, error) {
			test.That(t, in.BinaryId, test.ShouldResemble, expectedBinaryIDPb)
			test.That(t, in.BboxId, test.ShouldEqual, expectedBBoxID)

			return &datapb.RemoveBoundingBoxFromImageByIDResponse{}, nil
		}
		client.RemoveBoundingBoxFromImageByID(context.Background(), expectedBBoxID, binaryID)
	})

	t.Run("BoundingBoxLabelsByFilter", func(t *testing.T) {
		expectedFilterPb := filterToProto(filter)
		expectedBBoxLabels := []string{
			annotations.Bboxes[0].Label,
			annotations.Bboxes[1].Label,
		}
		annotationsPb := annotationsToProto(annotations)
		expectedBBoxLabelsPb := []string{
			annotationsPb.Bboxes[0].Label,
			annotationsPb.Bboxes[1].Label,
		}

		grpcClient.BoundingBoxLabelsByFilterFunc = func(ctx context.Context, in *datapb.BoundingBoxLabelsByFilterRequest,
			opts ...grpc.CallOption,
		) (*datapb.BoundingBoxLabelsByFilterResponse, error) {
			test.That(t, in.Filter, test.ShouldResemble, expectedFilterPb)
			return &datapb.BoundingBoxLabelsByFilterResponse{
				Labels: expectedBBoxLabelsPb,
			}, nil
		}
		resp, _ := client.BoundingBoxLabelsByFilter(context.Background(), filter)
		test.That(t, resp, test.ShouldResemble, expectedBBoxLabels)
	})
	t.Run("UpdateBoundingBox", func(t *testing.T) {
		bBoxID := annotations.Bboxes[0].ID
		expectedBinaryIDPb := binaryIDToProto(binaryID)

		annotationsPb := annotationsToProto(annotations)
		expectedLabel := annotationsPb.Bboxes[0].Label
		expectedBBoxIDPb := annotationsPb.Bboxes[0].Id
		expectedXMin := annotationsPb.Bboxes[0].XMinNormalized
		expectedYMin := annotationsPb.Bboxes[0].YMinNormalized
		expectedXMax := annotationsPb.Bboxes[0].XMaxNormalized
		expectedYMax := annotationsPb.Bboxes[0].YMaxNormalized

		grpcClient.UpdateBoundingBoxFunc = func(ctx context.Context, in *datapb.UpdateBoundingBoxRequest,
			opts ...grpc.CallOption,
		) (*datapb.UpdateBoundingBoxResponse, error) {
			test.That(t, in.BinaryId, test.ShouldResemble, expectedBinaryIDPb)
			test.That(t, in.BboxId, test.ShouldResemble, expectedBBoxIDPb)
			test.That(t, *in.Label, test.ShouldEqual, expectedLabel)
			test.That(t, *in.XMinNormalized, test.ShouldEqual, expectedXMin)
			test.That(t, *in.YMinNormalized, test.ShouldEqual, expectedYMin)
			test.That(t, *in.XMaxNormalized, test.ShouldEqual, expectedXMax)
			test.That(t, *in.YMaxNormalized, test.ShouldEqual, expectedYMax)
			return &datapb.UpdateBoundingBoxResponse{}, nil
		}
		client.UpdateBoundingBox(context.Background(), binaryID, bBoxID, &expectedLabel,
			&expectedXMin, &expectedYMin, &expectedXMax, &expectedYMax)
	})

	t.Run("GetDatabaseConnection", func(t *testing.T) {
		expectedOrgID := organizationID
		expectedHostName := hostName
		expectedMongodbURI := mongodbURI
		expectedDBUser := true

		grpcClient.GetDatabaseConnectionFunc = func(ctx context.Context, in *datapb.GetDatabaseConnectionRequest,
			opts ...grpc.CallOption,
		) (*datapb.GetDatabaseConnectionResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldResemble, expectedOrgID)
			return &datapb.GetDatabaseConnectionResponse{
				Hostname:        expectedHostName,
				MongodbUri:      expectedMongodbURI,
				HasDatabaseUser: expectedDBUser,
			}, nil
		}
		resp, _ := client.GetDatabaseConnection(context.Background(), organizationID)
		test.That(t, resp, test.ShouldResemble, expectedHostName)
	})

	t.Run("ConfigureDatabaseUser", func(t *testing.T) {
		expectedOrgID := organizationID
		expectedPassword := password

		grpcClient.ConfigureDatabaseUserFunc = func(ctx context.Context, in *datapb.ConfigureDatabaseUserRequest,
			opts ...grpc.CallOption,
		) (*datapb.ConfigureDatabaseUserResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldResemble, expectedOrgID)
			test.That(t, in.Password, test.ShouldResemble, expectedPassword)
			return &datapb.ConfigureDatabaseUserResponse{}, nil
		}
		client.ConfigureDatabaseUser(context.Background(), organizationID, password)
	})

	t.Run("AddBinaryDataToDatasetByIDs", func(t *testing.T) {
		expectedBinaryIDs := binaryIDsToProto(binaryIDs)
		expectedDataSetID := datasetID

		grpcClient.AddBinaryDataToDatasetByIDsFunc = func(ctx context.Context, in *datapb.AddBinaryDataToDatasetByIDsRequest,
			opts ...grpc.CallOption,
		) (*datapb.AddBinaryDataToDatasetByIDsResponse, error) {
			test.That(t, in.BinaryIds, test.ShouldResemble, expectedBinaryIDs)
			test.That(t, in.DatasetId, test.ShouldResemble, expectedDataSetID)
			return &datapb.AddBinaryDataToDatasetByIDsResponse{}, nil
		}
		client.AddBinaryDataToDatasetByIDs(context.Background(), binaryIDs, datasetID)
	})

	t.Run("RemoveBinaryDataFromDatasetByIDs", func(t *testing.T) {
		expectedBinaryIDs := binaryIDsToProto(binaryIDs)
		expectedDataSetID := datasetID

		grpcClient.RemoveBinaryDataFromDatasetByIDsFunc = func(ctx context.Context, in *datapb.RemoveBinaryDataFromDatasetByIDsRequest,
			opts ...grpc.CallOption,
		) (*datapb.RemoveBinaryDataFromDatasetByIDsResponse, error) {
			test.That(t, in.BinaryIds, test.ShouldResemble, expectedBinaryIDs)
			test.That(t, in.DatasetId, test.ShouldResemble, expectedDataSetID)
			return &datapb.RemoveBinaryDataFromDatasetByIDsResponse{}, nil
		}
		client.RemoveBinaryDataFromDatasetByIDs(context.Background(), binaryIDs, datasetID)
	})
}
