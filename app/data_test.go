package app

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	datapb "go.viam.com/api/app/data/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"

	"go.viam.com/rdk/testutils/inject"
)

const (
	component_name  = "component_name"
	component_type  = "component_type"
	method          = "method"
	robot_name      = "robot_name"
	robot_id        = "robot_id"
	part_name       = "part_name"
	part_id         = "part_id"
	location_id     = "location_id"
	organization_id = "organization_id"
	// password       = "password"
	mime_type  = "mime_type"
	uri        = "some.robot.uri"
	bbox_label = "bbox_label"
	dataset_id = "dataset_id"
	tag        = "tag"
)

var (
	location_IDs      = []string{location_id}
	org_IDs           = []string{organization_id}
	mime_Types        = []string{mime_type}
	Bbox_Labels       = []string{bbox_label}
	method_parameters = map[string]interface{}{}
	tags              = []string{tag}
)

// // Helper function to create BSON documents for MongoDB queries
// func createMQLBSON() [][]byte {
// 	// create BSON documents for mongodb queries
// 	matchStage := bson.M{"$match": bson.M{"organization_id": "e76d1b3b-0468-4efd-bb7f-fb1d2b352fcb"}}
// 	limitStage := bson.M{"$limit": 1}
// 	// convert to BSON byte arrays
// 	matchBytes, _ := bson.Marshal(matchStage)
// 	limitBytes, _ := bson.Marshal(limitStage)
// 	mqlbinary := [][]byte{matchBytes, limitBytes}

// 	return mqlbinary

// }

// set up gRPC client??
func createGrpclient() datapb.DataServiceClient {
	return &inject.DataServiceClient{}
}

func TestDataClient(t *testing.T) {
	grpcClient := &inject.DataServiceClient{}
	client := DataClient{client: grpcClient}

	capture_interval := CaptureInterval{
		Start: time.Now(),
		End:   time.Now(),
	}
	tags_filter := TagsFilter{
		Type: 2,
		Tags: []string{"tag1", "tag2"},
	}

	myFilter := Filter{
		ComponentName:   component_name,
		ComponentType:   component_type,
		Method:          method,
		RobotName:       robot_name,
		RobotId:         robot_id,
		PartName:        part_name,
		PartId:          part_id,
		LocationIds:     location_IDs,
		OrganizationIds: org_IDs,
		MimeType:        mime_Types,
		Interval:        capture_interval,
		TagsFilter:      tags_filter, //asterix or no??
		BboxLabels:      Bbox_Labels,
		DatasetId:       dataset_id,
	}
	myTabularMetadata := CaptureMetadata{
		OrganizationId:   organization_id,
		LocationId:       location_id,
		RobotName:        robot_name,
		RobotId:          robot_id,
		PartName:         part_name,
		PartId:           part_id,
		ComponentType:    component_type,
		ComponentName:    component_name,
		MethodName:       method,
		MethodParameters: method_parameters,
		Tags:             tags,
		MimeType:         mime_type,
	}
	t.Run("TabularDataByFilter", func(t *testing.T) {
		myDataRequest := DataRequest{
			Filter:    myFilter,
			Limit:     5,
			Last:      "last",
			SortOrder: Unspecified,
		}
		myTabularData := TabularData{
			Data: map[string]interface{}{
				"key": "value",
			},
			MetadataIndex: 0,
			Metadata:      myTabularMetadata, //not sure if i need to pass this
			TimeRequested: time.Now(),
			TimeReceived:  time.Now(),
		}
		myTabularDatas := []TabularData{myTabularData}
		myCount := uint64(5)
		myLast := "last"
		myLimit := uint64(100)
		countOnly := true
		includeInternalData := true

		grpcClient.TabularDataByFilterFunc = func(ctx context.Context, in *datapb.TabularDataByFilterRequest, opts ...grpc.CallOption) (*datapb.TabularDataByFilterResponse, error) {
			test.That(t, in.DataRequest, test.ShouldEqual, myDataRequest)
			test.That(t, in.CountOnly, test.ShouldBeTrue)
			test.That(t, in.IncludeInternalData, test.ShouldBeTrue)
			return &datapb.TabularDataByFilterResponse{Data: TabularDataToProtoList(myTabularDatas), Count: myCount, Last: myLast}, nil
			//the only returns we care about are TabularData, int coint, and str last rewturned pg id
		}

		respTabularData, respCount, respLast, _ := client.TabularDataByFilter(context.Background(), myFilter, myLimit, myLast, myDataRequest.SortOrder, countOnly, includeInternalData)
		test.That(t, respTabularData, test.ShouldEqual, myTabularData)
		test.That(t, respCount, test.ShouldEqual, myCount)
		test.That(t, respLast, test.ShouldEqual, myLast)
	})

	t.Run("TabularDataBySQL", func(t *testing.T) {

	})
	t.Run("TabularDataByMQL", func(t *testing.T) {
		org_id := "MY_ORG"
		input := map[string]interface{}{
			"foo":  "bar",
			"one":  1,
			"list": []string{"a", "b", "c"},
		}
		//serialize input into list of bytearrays, aka bson
		bsonData, err := bson.Marshal(input)
		if err != nil {
			fmt.Printf("trying something out")
		}
		mql_binary := [][]byte{bsonData} //bson data type

		expected := []map[string]interface{}{
			{"foo": "bar"},
			{"one": 1},
			{"list": []string{"a", "b", "c"}},
		}

		grpcClient.TabularDataByMQLFunc = func(ctx context.Context, in *datapb.TabularDataByMQLRequest, opts ...grpc.CallOption) (*datapb.TabularDataByMQLResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, org_id)
			test.That(t, in.MqlBinary, test.ShouldResemble, mql_binary)
			return &datapb.TabularDataByMQLResponse{RawData: mql_binary}, nil //MQlResponse is created with BSON data
		}

		response, _ := client.TabularDataByMQL(context.Background(), org_id, mql_binary)
		// test.That(t, response, test.ShouldEqual, expected)
		fmt.Printf("Expected: %#v\n", expected)
		fmt.Printf("Actual  : %#v\n", response)
		test.That(t, response, test.ShouldResemble, expected) //*this test not working as expected yet!
	})
	t.Run("BinaryDataByFilter", func(t *testing.T) {

	})
	t.Run("BinaryDataByIDs", func(t *testing.T) {
		binaryID1 := BinaryID{ //can this just be of type BinaryID??? why proto type?
			FileId:         "file1",
			OrganizationId: "org1",
			LocationId:     "loc1",
		}
		//  ==> why would it have to be of type datapb??? cant it jsut be of BinaryID
		binaryID2 := BinaryID{
			FileId:         "file2",
			OrganizationId: "org1",
			LocationId:     "loc1",
		}
		binaryIDs := []BinaryID{binaryID1, binaryID2}

		// binaryMetadata that acts as input!
		metadata := BinaryMetadata{
			ID:              "id1",
			CaptureMetadata: CaptureMetadata{ /* I probs need to fill these fields in???*/ },
			TimeRequested:   time.Now(),
			TimeReceived:    time.Now(),
			FileName:        "file.txt",
			FileExt:         ".txt",
			URI:             "http://ex/file.txt", //can i just make this up??
			Annotations:     Annotations{ /* this too???*/ },
			DatasetIDs:      []string{"dataset1", "dataset2"},
		}

		binary := []byte("something")
		// binaryData that acts as input
		binaryData := BinaryData{
			Binary:   binary,
			Metadata: metadata,
		}
		binaryDataList := []BinaryData{binaryData}

		grpcClient.BinaryDataByIDsFunc = func(ctx context.Context, in *datapb.BinaryDataByIDsRequest, opts ...grpc.CallOption) (*datapb.BinaryDataByIDsResponse, error) {
			test.That(t, in.BinaryIds, test.ShouldEqual, binaryIDs)
			return &datapb.BinaryDataByIDsResponse{Data: binaryDataList, Count: uint64(len(binaryDataList))}, nil
		}
		response, _ := client.BinaryDataByIDs(context.Background(), binaryIDs)

		// expectedData := BinaryData{binary: binary, Metadata: metadata}
		// var expectedData []BinaryData{BinaryDataFromProto(binaryDataList)}
		var expectedData []BinaryData
		for _, binaryDataItem := range binaryDataList {
			convertedData := BinaryDataFromProto(binaryDataItem)
			expectedData = append(expectedData, convertedData)
		}
		//loop thru binary dataDataList and convert binaryDataFromproto each one---> and then add that to a list...!
		test.That(t, response, test.ShouldResemble, expectedData)
	})

}

// func TestTabularDataByFilter(t *testing.T) {
// 	// Set up test logger
// 	logger := logging.NewTestLogger(t)
// 	// need this listener probs
// 	listener, err := net.Listen("tcp", "localhost:0")
// 	test.That(t, err, test.ShouldBeNil)
// 	client, conn, err := createGrpclient(t, logger, listener) // create client conn w helper
// 	test.That(t, err, test.ShouldBeNil)

// 	dataRequest := &datapb.DataRequest{} //this doesn't seem right???

// 	//grpc requeset??
// 	req := &datapb.TabularDataByFilterRequest{
// 		DataRequest:         dataRequest, //this doesn't seem right???
// 		CountOnly:           true,
// 		IncludeInternalData: true,
// 	}

// 	// call the real method
// 	resp, err := client.TabularDataByFilter(context.Background(), req)
// 	test.That(t, err, test.ShouldBeNil)
// 	// check that the parameters being passed match the expected data??
// 	test.That(t, req.DataRequest, test.ShouldResemble, dataRequest)

// 	// check that the response matches expected data
// 	expectedData := &datapb.TabularDataByFilterResponse{
// 		Data:    resp.Data,    //idk if it makes sense to be passing the resp.Data??
// 		RawData: resp.RawData, //bc what are we actually testing?
// 	}
// 	// test.That(t, expectedData, test.ShouldResembleProto, &datapb.TabularDataByMQLResponse{})
// 	test.That(t, expectedData, test.ShouldResemble, resp)
// 	// Close the connection
// 	require.NoError(t, conn.Close())

// }

// func TestTabularDataBySQL(t *testing.T) {}
// func TestTabularDataByMQL(t *testing.T) {
// 	client := createGrpclient()

// 	return
// 	// Set up test logger
// 	logger := logging.NewTestLogger(t)
// 	// need this listener probs
// 	listener, err := net.Listen("tcp", "localhost:0")
// 	test.That(t, err, test.ShouldBeNil)
// 	client, conn, err := createGrpclient(t, logger, listener) // create client conn w helper
// 	test.That(t, err, test.ShouldBeNil)

// 	//call helper
// 	mqlbinary := createMQLBSON()
// 	// orgId := "e76d1b3b-0468-4efd-bb7f-fb1d2b352fcb"
// 	// make the actual call to the grpc dataserive function??
// 	req := &datapb.TabularDataByMQLRequest{
// 		OrganizationId: orgId,
// 		MqlBinary:      mqlbinary,
// 	}
// 	// call the real method
// 	resp, err := client.TabularDataByMQL(context.Background(), req)
// 	test.That(t, err, test.ShouldBeNil)
// 	// check that the parameters being passed match the expected data??
// 	test.That(t, req.OrganizationId, test.ShouldResemble, orgId)

// 	// check that the response matches expected data
// 	expectedData := &datapb.TabularDataByMQLResponse{
// 		Data:    resp.Data,    //idk if it makes sense to be passing the resp.Data??
// 		RawData: resp.RawData, //bc what are we actually testing?
// 	}
// 	// test.That(t, expectedData, test.ShouldResembleProto, &datapb.TabularDataByMQLResponse{})
// 	test.That(t, expectedData, test.ShouldResemble, resp)
// 	// Close the connection
// 	require.NoError(t, conn.Close())
// }

// func TestBinaryDataByFilter(t *testing.T)               {}
// func TestBinaryDataByIDs(t *testing.T)                  {}
// func TestDeleteTabularData(t *testing.T)                {}
// func TestDeleteBinaryDataByFilter(t *testing.T)         {}
// func TestDeleteBinaryDataByIDs(t *testing.T)            {}
// func TestAddTagsToBinaryDataByIDs(t *testing.T)         {}
// func TestAddTagsToBinaryDataByFilter(t *testing.T)      {}
// func TestRemoveTagsFromBinaryDataByIDs(t *testing.T)    {}
// func TestRemoveTagsFromBinaryDataByFilter(t *testing.T) {}
// func TestTagsByFilter(t *testing.T)                     {}

//notes:
// Set up gRPC server (should it be grpc.NewServer()??)
// logger := logging.NewTestLogger(t)
// listener, err := net.Listen("tcp", "localhost:0")
// test.That(t, err, test.ShouldBeNil)
// rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
// test.That(t, err, test.ShouldBeNil)

//****need to somehow register the DataService server here??
// datapb.RegisterDataServiceHandlerFromEndpoint(context.Background(),runtime.NewServeMux(),)
// datapb.RegisterDataServiceServer(rpcServer,)
//  datapb.RegisterDataServiceServer(rpcServer, &datapb.UnimplementedDataServiceServer{})
// data = datapb.DataServiceServer{}

// Start serving requests??
// go rpcServer.Serve(listener)
// defer rpcServer.Stop()

// Make client connection
// conn, err := viamgrpc.Dial(context.Background(), listener.Addr().String(), logger)
// test.That(t, err, test.ShouldBeNil)
// client := datapb.NewDataServiceClient(conn)

//notes on the param chekcing stuff
// var request *datapb.TabularDataByMQLRequest

// grpcClient.TabularDataByMQLFunc = func(ctx context.Context, in *datapb.TabularDataByMQLRequest, opts ...grpc.CallOption) (*datapb.TabularDataByMQLResponse, error) {
// 	request = in
// 	return &datapb.TabularDataByMQLResponse{RawData: mql_binary}, nil //MQlResponse is created with BSON data
// }

// assert request.org
