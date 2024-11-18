package app

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	pb "go.viam.com/api/app/data/v1"
	syncPb "go.viam.com/api/app/datasync/v1"

	"go.viam.com/test"
	utils "go.viam.com/utils/protoutils"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/protoutils"
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
	last           = "last"
)

var (
	locationIDs         = []string{locationID}
	orgIDs              = []string{organizationID}
	mimeTypes           = []string{mimeType}
	bboxLabels          = []string{bboxLabel}
	methodParameters    = map[string]interface{}{}
	tags                = []string{tag}
	startTime           = time.Now().UTC().Round(time.Millisecond)
	endTime             = time.Now().UTC().Round(time.Millisecond)
	count               = uint64(5)
	limit               = uint64(5)
	countOnly           = true
	includeInternalData = true
	data                = map[string]interface{}{
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
	rawData        = []map[string]interface{}{
		{
			"key1": startTime,
			"key2": "2",
			"key3": []interface{}{1, 2, 3},
			"key4": map[string]interface{}{
				"key4sub1": endTime,
			},
			"key5": 4.05,
			"key6": []interface{}{true, false, true},
			"key7": []interface{}{
				map[string]interface{}{
					"nestedKey1": "simpleValue",
				},
				map[string]interface{}{
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

func annotationsToProto(annotations Annotations) *pb.Annotations {
	var protoBboxes []*pb.BoundingBox
	for _, bbox := range annotations.Bboxes {
		protoBboxes = append(protoBboxes, &pb.BoundingBox{
			Id:             bbox.ID,
			Label:          bbox.Label,
			XMinNormalized: bbox.XMinNormalized,
			YMinNormalized: bbox.YMinNormalized,
			XMaxNormalized: bbox.XMaxNormalized,
			YMaxNormalized: bbox.YMaxNormalized,
		})
	}
	return &pb.Annotations{
		Bboxes: protoBboxes,
	}
}

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

func binaryMetadataToProto(binaryMetadata BinaryMetadata) *pb.BinaryMetadata {
	return &pb.BinaryMetadata{
		Id:              binaryMetadata.ID,
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
		Filter:    filterToProto(dataRequest.Filter),
		Limit:     dataRequest.Limit,
		Last:      dataRequest.Last,
		SortOrder: orderToProto(dataRequest.SortOrder),
	}
}

func createGrpcClient() *inject.DataServiceClient {
	return &inject.DataServiceClient{}
}
func createGrpcDataSyncClient() *inject.DataSyncServiceClient {
	return &inject.DataSyncServiceClient{}
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

	dataRequest := DataRequest{
		Filter:    filter,
		Limit:     count,
		Last:      last,
		SortOrder: Unspecified,
	}

	binaryData := BinaryData{
		Binary:   binaryDataByte,
		Metadata: binaryMetadata,
	}

	t.Run("TabularDataByFilter", func(t *testing.T) {
		tabularData := TabularData{
			Data:          data,
			MetadataIndex: 0,
			Metadata:      tabularMetadata,
			TimeRequested: startTime,
			TimeReceived:  endTime,
		}
		dataStruct, _ := utils.StructToStructPb(data)
		tabularDataPb := &pb.TabularData{
			Data:          dataStruct,
			MetadataIndex: 0,
			TimeRequested: timestamppb.New(startTime),
			TimeReceived:  timestamppb.New(endTime),
		}
		grpcClient.TabularDataByFilterFunc = func(ctx context.Context, in *pb.TabularDataByFilterRequest,
			opts ...grpc.CallOption,
		) (*pb.TabularDataByFilterResponse, error) {
			test.That(t, in.DataRequest, test.ShouldResemble, dataRequestToProto(dataRequest))
			test.That(t, in.CountOnly, test.ShouldBeTrue)
			test.That(t, in.IncludeInternalData, test.ShouldBeTrue)
			return &pb.TabularDataByFilterResponse{
				Data:     []*pb.TabularData{tabularDataPb},
				Count:    count,
				Last:     last,
				Metadata: []*pb.CaptureMetadata{captureMetadataToProto(tabularMetadata)},
			}, nil
		}
		resp, _ := client.TabularDataByFilter(
			context.Background(), filter, limit, last,
			dataRequest.SortOrder, countOnly, includeInternalData)
		test.That(t, resp.TabularData[0], test.ShouldResemble, tabularData)
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
		response, _ := client.TabularDataBySQL(context.Background(), organizationID, sqlQuery)
		test.That(t, response, test.ShouldResemble, rawData)
	})

	t.Run("TabularDataByMQL", func(t *testing.T) {
		// convert to BSON byte arrays
		matchBytes, _ := bson.Marshal(bson.M{"$match": bson.M{"organization_id": "e76d1b3b-0468-4efd-bb7f-fb1d2b352fcb"}})
		limitBytes, _ := bson.Marshal(bson.M{"$limit": 1})
		mqlbinary := [][]byte{matchBytes, limitBytes}

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
			test.That(t, in.MqlBinary, test.ShouldResemble, mqlbinary)
			return &pb.TabularDataByMQLResponse{
				RawData: expectedRawDataPb,
			}, nil
		}
		response, _ := client.TabularDataByMQL(context.Background(), organizationID, mqlbinary)
		test.That(t, response, test.ShouldResemble, rawData)
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
				Count: count,
				Last:  last,
			}, nil
		}
		resp, _ := client.BinaryDataByFilter(
			context.Background(), filter, count, dataRequest.SortOrder,
			last, includeBinary, countOnly, includeInternalData)
		test.That(t, resp.BinaryData[0], test.ShouldResemble, binaryData)
		test.That(t, resp.Count, test.ShouldEqual, count)
		test.That(t, resp.Last, test.ShouldEqual, last)
	})
	t.Run("BinaryDataByIDs", func(t *testing.T) {
		grpcClient.BinaryDataByIDsFunc = func(ctx context.Context, in *pb.BinaryDataByIDsRequest,
			opts ...grpc.CallOption,
		) (*pb.BinaryDataByIDsResponse, error) {
			test.That(t, in.IncludeBinary, test.ShouldBeTrue)
			test.That(t, in.BinaryIds, test.ShouldResemble, binaryIDsToProto(binaryIDs))
			expectedBinaryDataList := []*pb.BinaryData{binaryDataToProto(binaryData)}

			return &pb.BinaryDataByIDsResponse{Data: expectedBinaryDataList, Count: uint64(len(expectedBinaryDataList))}, nil
		}
		respBinaryData, _ := client.BinaryDataByIDs(context.Background(), binaryIDs)
		test.That(t, respBinaryData[0], test.ShouldResemble, binaryData)
	})

	t.Run("DeleteTabularData", func(t *testing.T) {
		deleteOlderThanDays := uint32(1)
		grpcClient.DeleteTabularDataFunc = func(ctx context.Context, in *pb.DeleteTabularDataRequest,
			opts ...grpc.CallOption,
		) (*pb.DeleteTabularDataResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			test.That(t, in.DeleteOlderThanDays, test.ShouldEqual, deleteOlderThanDays)

			return &pb.DeleteTabularDataResponse{
				DeletedCount: count,
			}, nil
		}
		resp, _ := client.DeleteTabularData(context.Background(), organizationID, deleteOlderThanDays)
		test.That(t, resp, test.ShouldEqual, count)
	})

	t.Run("DeleteBinaryDataByFilter", func(t *testing.T) {
		grpcClient.DeleteBinaryDataByFilterFunc = func(ctx context.Context, in *pb.DeleteBinaryDataByFilterRequest,
			opts ...grpc.CallOption,
		) (*pb.DeleteBinaryDataByFilterResponse, error) {
			test.That(t, in.Filter, test.ShouldResemble, filterToProto(filter))
			test.That(t, in.IncludeInternalData, test.ShouldBeTrue)
			return &pb.DeleteBinaryDataByFilterResponse{
				DeletedCount: count,
			}, nil
		}
		resp, _ := client.DeleteBinaryDataByFilter(context.Background(), filter)
		test.That(t, resp, test.ShouldEqual, count)
	})

	t.Run("DeleteBinaryDataByIDs", func(t *testing.T) {
		grpcClient.DeleteBinaryDataByIDsFunc = func(ctx context.Context, in *pb.DeleteBinaryDataByIDsRequest,
			opts ...grpc.CallOption,
		) (*pb.DeleteBinaryDataByIDsResponse, error) {
			test.That(t, in.BinaryIds, test.ShouldResemble, binaryIDsToProto(binaryIDs))
			return &pb.DeleteBinaryDataByIDsResponse{
				DeletedCount: count,
			}, nil
		}
		resp, _ := client.DeleteBinaryDataByIDs(context.Background(), binaryIDs)
		test.That(t, resp, test.ShouldEqual, count)
	})

	t.Run("AddTagsToBinaryDataByIDs", func(t *testing.T) {
		grpcClient.AddTagsToBinaryDataByIDsFunc = func(ctx context.Context, in *pb.AddTagsToBinaryDataByIDsRequest,
			opts ...grpc.CallOption,
		) (*pb.AddTagsToBinaryDataByIDsResponse, error) {
			test.That(t, in.BinaryIds, test.ShouldResemble, binaryIDsToProto(binaryIDs))
			test.That(t, in.Tags, test.ShouldResemble, tags)
			return &pb.AddTagsToBinaryDataByIDsResponse{}, nil
		}
		client.AddTagsToBinaryDataByIDs(context.Background(), tags, binaryIDs)
	})

	t.Run("AddTagsToBinaryDataByFilter", func(t *testing.T) {
		grpcClient.AddTagsToBinaryDataByFilterFunc = func(ctx context.Context, in *pb.AddTagsToBinaryDataByFilterRequest,
			opts ...grpc.CallOption,
		) (*pb.AddTagsToBinaryDataByFilterResponse, error) {
			test.That(t, in.Filter, test.ShouldResemble, filterToProto(filter))
			test.That(t, in.Tags, test.ShouldResemble, tags)
			return &pb.AddTagsToBinaryDataByFilterResponse{}, nil
		}
		client.AddTagsToBinaryDataByFilter(context.Background(), tags, filter)
	})

	t.Run("RemoveTagsFromBinaryDataByIDs", func(t *testing.T) {
		grpcClient.RemoveTagsFromBinaryDataByIDsFunc = func(ctx context.Context, in *pb.RemoveTagsFromBinaryDataByIDsRequest,
			opts ...grpc.CallOption,
		) (*pb.RemoveTagsFromBinaryDataByIDsResponse, error) {
			test.That(t, in.BinaryIds, test.ShouldResemble, binaryIDsToProto(binaryIDs))
			test.That(t, in.Tags, test.ShouldResemble, tags)
			return &pb.RemoveTagsFromBinaryDataByIDsResponse{
				DeletedCount: count,
			}, nil
		}
		resp, _ := client.RemoveTagsFromBinaryDataByIDs(context.Background(), tags, binaryIDs)
		test.That(t, resp, test.ShouldEqual, count)
	})

	t.Run("RemoveTagsFromBinaryDataByFilter", func(t *testing.T) {
		grpcClient.RemoveTagsFromBinaryDataByFilterFunc = func(ctx context.Context, in *pb.RemoveTagsFromBinaryDataByFilterRequest,
			opts ...grpc.CallOption,
		) (*pb.RemoveTagsFromBinaryDataByFilterResponse, error) {
			test.That(t, in.Filter, test.ShouldResemble, filterToProto(filter))
			test.That(t, in.Tags, test.ShouldResemble, tags)
			return &pb.RemoveTagsFromBinaryDataByFilterResponse{
				DeletedCount: count,
			}, nil
		}
		resp, _ := client.RemoveTagsFromBinaryDataByFilter(context.Background(), tags, filter)
		test.That(t, resp, test.ShouldEqual, count)
	})

	t.Run("TagsByFilter", func(t *testing.T) {
		grpcClient.TagsByFilterFunc = func(ctx context.Context, in *pb.TagsByFilterRequest,
			opts ...grpc.CallOption,
		) (*pb.TagsByFilterResponse, error) {
			test.That(t, in.Filter, test.ShouldResemble, filterToProto(filter))
			return &pb.TagsByFilterResponse{
				Tags: tags,
			}, nil
		}
		resp, _ := client.TagsByFilter(context.Background(), filter)
		test.That(t, resp, test.ShouldResemble, tags)
	})

	t.Run("AddBoundingBoxToImageByID", func(t *testing.T) {
		grpcClient.AddBoundingBoxToImageByIDFunc = func(ctx context.Context,
			in *pb.AddBoundingBoxToImageByIDRequest,
			opts ...grpc.CallOption,
		) (*pb.AddBoundingBoxToImageByIDResponse, error) {
			test.That(t, in.BinaryId, test.ShouldResemble, binaryIDToProto(binaryID))
			test.That(t, in.Label, test.ShouldEqual, bboxLabel)
			test.That(t, in.XMinNormalized, test.ShouldEqual, annotations.Bboxes[0].XMinNormalized)
			test.That(t, in.YMinNormalized, test.ShouldEqual, annotations.Bboxes[0].YMinNormalized)
			test.That(t, in.XMaxNormalized, test.ShouldEqual, annotations.Bboxes[0].XMaxNormalized)
			test.That(t, in.YMaxNormalized, test.ShouldEqual, annotations.Bboxes[0].YMaxNormalized)

			return &pb.AddBoundingBoxToImageByIDResponse{
				BboxId: annotations.Bboxes[0].ID,
			}, nil
		}
		resp, _ := client.AddBoundingBoxToImageByID(
			context.Background(), binaryID, bboxLabel, annotations.Bboxes[0].XMinNormalized,
			annotations.Bboxes[0].YMinNormalized, annotations.Bboxes[0].XMaxNormalized, annotations.Bboxes[0].YMaxNormalized)
		test.That(t, resp, test.ShouldResemble, annotations.Bboxes[0].ID)
	})

	t.Run("RemoveBoundingBoxFromImageByID", func(t *testing.T) {
		grpcClient.RemoveBoundingBoxFromImageByIDFunc = func(ctx context.Context, in *pb.RemoveBoundingBoxFromImageByIDRequest,
			opts ...grpc.CallOption,
		) (*pb.RemoveBoundingBoxFromImageByIDResponse, error) {
			test.That(t, in.BinaryId, test.ShouldResemble, binaryIDToProto(binaryID))
			test.That(t, in.BboxId, test.ShouldEqual, annotations.Bboxes[0].ID)

			return &pb.RemoveBoundingBoxFromImageByIDResponse{}, nil
		}
		client.RemoveBoundingBoxFromImageByID(context.Background(), annotations.Bboxes[0].ID, binaryID)
	})

	t.Run("BoundingBoxLabelsByFilter", func(t *testing.T) {
		expectedBBoxLabels := []string{
			annotations.Bboxes[0].Label,
			annotations.Bboxes[1].Label,
		}
		expectedBBoxLabelsPb := []string{
			annotationsToProto(annotations).Bboxes[0].Label,
			annotationsToProto(annotations).Bboxes[1].Label,
		}
		grpcClient.BoundingBoxLabelsByFilterFunc = func(ctx context.Context, in *pb.BoundingBoxLabelsByFilterRequest,
			opts ...grpc.CallOption,
		) (*pb.BoundingBoxLabelsByFilterResponse, error) {
			test.That(t, in.Filter, test.ShouldResemble, filterToProto(filter))
			return &pb.BoundingBoxLabelsByFilterResponse{
				Labels: expectedBBoxLabelsPb,
			}, nil
		}
		resp, _ := client.BoundingBoxLabelsByFilter(context.Background(), filter)
		test.That(t, resp, test.ShouldResemble, expectedBBoxLabels)
	})
	t.Run("UpdateBoundingBox", func(t *testing.T) {
		annotationsPb := annotationsToProto(annotations)
		grpcClient.UpdateBoundingBoxFunc = func(ctx context.Context, in *pb.UpdateBoundingBoxRequest,
			opts ...grpc.CallOption,
		) (*pb.UpdateBoundingBoxResponse, error) {
			test.That(t, in.BinaryId, test.ShouldResemble, binaryIDToProto(binaryID))
			test.That(t, in.BboxId, test.ShouldResemble, annotationsPb.Bboxes[0].Id)
			test.That(t, *in.Label, test.ShouldEqual, annotationsPb.Bboxes[0].Label)
			test.That(t, *in.XMinNormalized, test.ShouldEqual, annotationsPb.Bboxes[0].XMinNormalized)
			test.That(t, *in.YMinNormalized, test.ShouldEqual, annotationsPb.Bboxes[0].YMinNormalized)
			test.That(t, *in.XMaxNormalized, test.ShouldEqual, annotationsPb.Bboxes[0].XMaxNormalized)
			test.That(t, *in.YMaxNormalized, test.ShouldEqual, annotationsPb.Bboxes[0].YMaxNormalized)
			return &pb.UpdateBoundingBoxResponse{}, nil
		}
		client.UpdateBoundingBox(context.Background(), binaryID, annotations.Bboxes[0].ID, &annotationsPb.Bboxes[0].Label,
			&annotationsPb.Bboxes[0].XMinNormalized, &annotationsPb.Bboxes[0].YMinNormalized,
			&annotationsPb.Bboxes[0].XMaxNormalized, &annotationsPb.Bboxes[0].YMaxNormalized)
	})

	t.Run("GetDatabaseConnection", func(t *testing.T) {
		grpcClient.GetDatabaseConnectionFunc = func(ctx context.Context, in *pb.GetDatabaseConnectionRequest,
			opts ...grpc.CallOption,
		) (*pb.GetDatabaseConnectionResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldResemble, organizationID)
			return &pb.GetDatabaseConnectionResponse{
				Hostname:        hostName,
				MongodbUri:      mongodbURI,
				HasDatabaseUser: true,
			}, nil
		}
		resp, _ := client.GetDatabaseConnection(context.Background(), organizationID)
		test.That(t, resp.Hostname, test.ShouldResemble, hostName)
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
		client.ConfigureDatabaseUser(context.Background(), organizationID, password)
	})

	t.Run("AddBinaryDataToDatasetByIDs", func(t *testing.T) {
		grpcClient.AddBinaryDataToDatasetByIDsFunc = func(ctx context.Context, in *pb.AddBinaryDataToDatasetByIDsRequest,
			opts ...grpc.CallOption,
		) (*pb.AddBinaryDataToDatasetByIDsResponse, error) {
			test.That(t, in.BinaryIds, test.ShouldResemble, binaryIDsToProto(binaryIDs))
			test.That(t, in.DatasetId, test.ShouldResemble, datasetID)
			return &pb.AddBinaryDataToDatasetByIDsResponse{}, nil
		}
		client.AddBinaryDataToDatasetByIDs(context.Background(), binaryIDs, datasetID)
	})

	t.Run("RemoveBinaryDataFromDatasetByIDs", func(t *testing.T) {
		grpcClient.RemoveBinaryDataFromDatasetByIDsFunc = func(ctx context.Context, in *pb.RemoveBinaryDataFromDatasetByIDsRequest,
			opts ...grpc.CallOption,
		) (*pb.RemoveBinaryDataFromDatasetByIDsResponse, error) {
			test.That(t, in.BinaryIds, test.ShouldResemble, binaryIDsToProto(binaryIDs))
			test.That(t, in.DatasetId, test.ShouldResemble, datasetID)
			return &pb.RemoveBinaryDataFromDatasetByIDsResponse{}, nil
		}
		client.RemoveBinaryDataFromDatasetByIDs(context.Background(), binaryIDs, datasetID)
	})
}

// ***********Added this below for new dataSync !!!******
func TestDataSyncClient(t *testing.T) {
	grpcClient := createGrpcDataSyncClient()
	client := DataClient{dataSyncClient: grpcClient}

	t.Run("DataCaptureUpload", func(t *testing.T) {
		grpcClient.DataCaptureUploadFunc = func(ctx context.Context, in *syncPb.DataCaptureUploadRequest,
			opts ...grpc.CallOption,
		) (*syncPb.DataCaptureUploadResponse, error) {
			//test.That(t, in._, test.ShouldResemble, toProto(something)) //toProto
			return &syncPb.DataCaptureUploadResponse{
				//fill all variables w prototype-types

			}, nil
		}
		resp, _ := client.DataCaptureUpload(context.Background()) //not proto-types, regular types u expect to recieve in the function
		//test.That(t, resp._, test.ShouldResemble, fromProto(something if needed)) //compare response with regular expected types
	})

}
