package app

import (
	"context"
	"fmt"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	datapb "go.viam.com/api/app/data/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"

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
	t.Run("TabularDataByFilter", func(t *testing.T) {

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
