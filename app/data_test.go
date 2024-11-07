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
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/testutils/inject"
)

// var (
// 	orgId = "e76d1b3b-0468-4efd-bb7f-fb1d2b352fcb"
// )

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
	t.Run("TabularDataByFilter", func(t *testing.T) {
		// expected := {}
		// expectedData := "" //change - not done
		// expectedCount := 5 //change - not done
		// expectedLast := "" //change - not done
		// filter := &datapb.Filter{}
		// limit := uint64(5)
		// last := "last"
		// sortOrder := datapb.Order_ORDER_DESCENDING  //i think this is correct
		// countOnly := true
		// includeInternalData := true
		// myData := {
		// 	Data: map[string]interface{}
		// 	Metadata: CaptureMetadata
		// }
		// myLast := nil

		// returns: List[TabularData]: The tabular data, int: The count (number of entries), str: The last-returned page ID.

		grpcClient.TabularDataByFilterFunc = func(ctx context.Context, in *datapb.TabularDataByFilterRequest, opts ...grpc.CallOption) (*datapb.TabularDataByFilterResponse, error) {
			test.That(t, in.DataRequest, test.ShouldEqual)
			test.That(t, in.CountOnly, test.ShouldBeTrue)
			test.That(t, in.IncludeInternalData, test.ShouldBeTrue)
			return &datapb.TabularDataByFilterResponse{Data: _, Count: _, Last: _}, nil
		}

		respTabularData, respCount, respLast, _ := client.TabularDataByFilter(context.Background(), filter, limit, last, sortOrder, countOnly, includeInternalData)
		test.That(t, respData, test.ShouldEqual, expectedData)
		test.That(t, respCount, test.ShouldEqual, expectedCount)
		test.That(t, respLast, test.ShouldEqual, expectedLast)

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
		
