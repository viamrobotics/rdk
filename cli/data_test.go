package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	datapb "go.viam.com/api/app/data/v1"
	datapipelinespb "go.viam.com/api/app/datapipelines/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"

	"go.viam.com/rdk/testutils/inject"
)

func TestFilenameForDownload(t *testing.T) {
	const expectedUTC = "1970-01-01T00_00_00Z"
	noFilename := filenameForDownload(&datapb.BinaryMetadata{Id: "my-id"})
	test.That(t, noFilename, test.ShouldEqual, expectedUTC+"_my-id")

	normalExt := filenameForDownload(&datapb.BinaryMetadata{FileName: "whatever.txt"})
	test.That(t, normalExt, test.ShouldEqual, expectedUTC+"_whatever.txt")

	inFolder := filenameForDownload(&datapb.BinaryMetadata{FileName: "dir/whatever.txt"})
	test.That(t, inFolder, test.ShouldEqual, "dir/whatever.txt")

	inViamCaptureFolder := filenameForDownload(&datapb.BinaryMetadata{FileName: "/.viam/capture/2024-01-30Twhatever.jpg"})
	test.That(t, inViamCaptureFolder, test.ShouldEqual, "2024-01-30Twhatever.jpg")

	nestedViamCaptureFolder := filenameForDownload(&datapb.BinaryMetadata{FileName: "Users/hi/.viam/capture/2024-01-30Twhatever.jpg"})
	test.That(t, nestedViamCaptureFolder, test.ShouldEqual, "2024-01-30Twhatever.jpg")

	nestedDirViamCaptureFolder := filenameForDownload(&datapb.BinaryMetadata{FileName: "Users/hi/.viam/capture/dir/2024-01-30Twhatever.jpg"})
	test.That(t, nestedDirViamCaptureFolder, test.ShouldEqual, "dir/2024-01-30Twhatever.jpg")

	gzAtRoot := filenameForDownload(&datapb.BinaryMetadata{FileName: "whatever.gz"})
	test.That(t, gzAtRoot, test.ShouldEqual, expectedUTC+"_whatever")

	gzInFolder := filenameForDownload(&datapb.BinaryMetadata{FileName: "dir/whatever.gz"})
	test.That(t, gzInFolder, test.ShouldEqual, "dir/whatever")
}

func TestDataQuerySQLAction(t *testing.T) {
	row1, err := bson.Marshal(bson.M{"part_id": "p1", "value": float64(42)})
	test.That(t, err, test.ShouldBeNil)
	row2, err := bson.Marshal(bson.M{"part_id": "p2", "value": float64(7)})
	test.That(t, err, test.ShouldBeNil)

	t.Run("prints rows to stdout when no destination is set", func(t *testing.T) {
		var capturedReq *datapb.TabularDataBySQLRequest
		dsc := &inject.DataServiceClient{
			TabularDataBySQLFunc: func(ctx context.Context, in *datapb.TabularDataBySQLRequest, opts ...grpc.CallOption,
			) (*datapb.TabularDataBySQLResponse, error) {
				capturedReq = in
				return &datapb.TabularDataBySQLResponse{RawData: [][]byte{row1, row2}}, nil
			},
		}

		_, ac, out, errOut := setup(&inject.AppServiceClient{}, dsc, nil, nil, "token")
		err := ac.dataQuerySQLAction(context.Background(), dataQuerySQLArgs{
			OrgID: "org-1",
			SQL:   "SELECT * FROM readings",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(errOut.messages), test.ShouldEqual, 0)
		test.That(t, capturedReq.GetOrganizationId(), test.ShouldEqual, "org-1")
		test.That(t, capturedReq.GetSqlQuery(), test.ShouldEqual, "SELECT * FROM readings")

		// Parse NDJSON written to stdout back into maps before comparing to expected.
		var actual []map[string]interface{}
		decoder := json.NewDecoder(strings.NewReader(strings.Join(out.messages, "")))
		for decoder.More() {
			var row map[string]interface{}
			test.That(t, decoder.Decode(&row), test.ShouldBeNil)
			actual = append(actual, row)
		}
		test.That(t, actual, test.ShouldResemble, []map[string]interface{}{
			{"part_id": "p1", "value": float64(42)},
			{"part_id": "p2", "value": float64(7)},
		})
	})

	t.Run("requires an org id", func(t *testing.T) {
		_, ac, _, _ := setup(&inject.AppServiceClient{}, &inject.DataServiceClient{}, nil, nil, "token")
		err := ac.dataQuerySQLAction(context.Background(), dataQuerySQLArgs{SQL: "SELECT 1"})
		test.That(t, err, test.ShouldBeError, errors.New("must provide an organization ID"))
	})

	t.Run("requires a sql query", func(t *testing.T) {
		_, ac, _, _ := setup(&inject.AppServiceClient{}, &inject.DataServiceClient{}, nil, nil, "token")
		err := ac.dataQuerySQLAction(context.Background(), dataQuerySQLArgs{OrgID: "org-1"})
		test.That(t, err, test.ShouldBeError, errors.New("must provide a SQL query"))
	})
}

func TestDataQueryMQLAction(t *testing.T) {
	row, err := bson.Marshal(bson.M{"part_id": "p1", "n": float64(1)})
	test.That(t, err, test.ShouldBeNil)

	mql := `[{"$match": {"part_id": "p1"}}]`

	t.Run("forwards data source and pipeline id", func(t *testing.T) {
		var capturedReq *datapb.TabularDataByMQLRequest
		dsc := &inject.DataServiceClient{
			TabularDataByMQLFunc: func(ctx context.Context, in *datapb.TabularDataByMQLRequest, opts ...grpc.CallOption,
			) (*datapb.TabularDataByMQLResponse, error) {
				capturedReq = in
				return &datapb.TabularDataByMQLResponse{RawData: [][]byte{row}}, nil
			},
		}

		_, ac, _, errOut := setup(&inject.AppServiceClient{}, dsc, nil, nil, "token")
		err := ac.dataQueryMQLAction(context.Background(), dataQueryMQLArgs{
			OrgID:          "org-1",
			MQL:            mql,
			DataSourceType: pipelineSinkDataSourceType,
			PipelineID:     "pipe-1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(errOut.messages), test.ShouldEqual, 0)
		test.That(t, capturedReq.GetOrganizationId(), test.ShouldEqual, "org-1")
		test.That(t, capturedReq.GetDataSource().GetType(),
			test.ShouldEqual, datapb.TabularDataSourceType_TABULAR_DATA_SOURCE_TYPE_PIPELINE_SINK)
		test.That(t, capturedReq.GetDataSource().GetPipelineId(), test.ShouldEqual, "pipe-1")
		test.That(t, len(capturedReq.GetMqlBinary()), test.ShouldEqual, 1)
	})

	t.Run("resolves pipeline name to id", func(t *testing.T) {
		var capturedReq *datapb.TabularDataByMQLRequest
		dsc := &inject.DataServiceClient{
			TabularDataByMQLFunc: func(ctx context.Context, in *datapb.TabularDataByMQLRequest, opts ...grpc.CallOption,
			) (*datapb.TabularDataByMQLResponse, error) {
				capturedReq = in
				return &datapb.TabularDataByMQLResponse{RawData: [][]byte{row}}, nil
			},
		}
		dpc := &inject.DataPipelinesServiceClient{
			ListDataPipelinesFunc: func(ctx context.Context, in *datapipelinespb.ListDataPipelinesRequest, opts ...grpc.CallOption,
			) (*datapipelinespb.ListDataPipelinesResponse, error) {
				return &datapipelinespb.ListDataPipelinesResponse{
					DataPipelines: []*datapipelinespb.DataPipeline{
						{Id: "pipe-other-id", Name: "other"},
						{Id: "pipe-matched-id", Name: "my-pipeline"},
					},
				}, nil
			},
		}

		_, ac, _, _ := setup(&inject.AppServiceClient{}, dsc, nil, nil, "token")
		ac.datapipelinesClient = dpc
		err := ac.dataQueryMQLAction(context.Background(), dataQueryMQLArgs{
			OrgID:          "org-1",
			MQL:            mql,
			DataSourceType: pipelineSinkDataSourceType,
			PipelineName:   "my-pipeline",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capturedReq.GetDataSource().GetPipelineId(), test.ShouldEqual, "pipe-matched-id")
	})

	t.Run("errors when pipeline name has no match", func(t *testing.T) {
		dpc := &inject.DataPipelinesServiceClient{
			ListDataPipelinesFunc: func(ctx context.Context, in *datapipelinespb.ListDataPipelinesRequest, opts ...grpc.CallOption,
			) (*datapipelinespb.ListDataPipelinesResponse, error) {
				return &datapipelinespb.ListDataPipelinesResponse{
					DataPipelines: []*datapipelinespb.DataPipeline{{Id: "x", Name: "something-else"}},
				}, nil
			},
		}
		_, ac, _, _ := setup(&inject.AppServiceClient{}, &inject.DataServiceClient{}, nil, nil, "token")
		ac.datapipelinesClient = dpc
		err := ac.dataQueryMQLAction(context.Background(), dataQueryMQLArgs{
			OrgID:          "org-1",
			MQL:            mql,
			DataSourceType: pipelineSinkDataSourceType,
			PipelineName:   "missing",
		})
		test.That(t, err, test.ShouldBeError, fmt.Errorf("no data pipeline found with name %q", "missing"))
	})

	t.Run("omits data source when not provided", func(t *testing.T) {
		var capturedReq *datapb.TabularDataByMQLRequest
		dsc := &inject.DataServiceClient{
			TabularDataByMQLFunc: func(ctx context.Context, in *datapb.TabularDataByMQLRequest, opts ...grpc.CallOption,
			) (*datapb.TabularDataByMQLResponse, error) {
				capturedReq = in
				return &datapb.TabularDataByMQLResponse{RawData: [][]byte{row}}, nil
			},
		}

		_, ac, _, errOut := setup(&inject.AppServiceClient{}, dsc, nil, nil, "token")
		err := ac.dataQueryMQLAction(context.Background(), dataQueryMQLArgs{OrgID: "org-1", MQL: mql})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(errOut.messages), test.ShouldEqual, 0)
		test.That(t, capturedReq.GetDataSource(), test.ShouldBeNil)
	})

	testCases := map[string]struct {
		args        dataQueryMQLArgs
		expectedErr error
	}{
		"hot-storage data source rejects a pipeline id": {
			args: dataQueryMQLArgs{
				OrgID:          "org-1",
				MQL:            mql,
				DataSourceType: hotStorageDataSourceType,
				PipelineID:     "pipe-1",
			},
			expectedErr: fmt.Errorf("--%s and --%s are only valid when --%s=%s",
				dataFlagPipelineID, dataFlagPipelineName, dataFlagDataSourceType, pipelineSinkDataSourceType),
		},
		"pipeline-sink requires a pipeline id or name": {
			args: dataQueryMQLArgs{
				OrgID:          "org-1",
				MQL:            mql,
				DataSourceType: pipelineSinkDataSourceType,
			},
			expectedErr: fmt.Errorf("--%s or --%s is required when --%s=%s",
				dataFlagPipelineID, dataFlagPipelineName, dataFlagDataSourceType, pipelineSinkDataSourceType),
		},
		"pipeline-id without a source type": {
			args: dataQueryMQLArgs{
				OrgID:      "org-1",
				MQL:        mql,
				PipelineID: "pipe-1",
			},
			expectedErr: fmt.Errorf("--%s is required when --%s or --%s is provided",
				dataFlagDataSourceType, dataFlagPipelineID, dataFlagPipelineName),
		},
		"pipeline-id and pipeline-name are both provided": {
			args: dataQueryMQLArgs{
				OrgID:          "org-1",
				MQL:            mql,
				DataSourceType: pipelineSinkDataSourceType,
				PipelineID:     "pipe-1",
				PipelineName:   "my-pipeline",
			},
			expectedErr: fmt.Errorf("--%s and --%s cannot both be provided",
				dataFlagPipelineID, dataFlagPipelineName),
		},
		"missing mql query": {
			args:        dataQueryMQLArgs{OrgID: "org-1"},
			expectedErr: errors.New("missing MQL query"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			_, ac, _, _ := setup(&inject.AppServiceClient{}, &inject.DataServiceClient{}, nil, nil, "token")
			err := ac.dataQueryMQLAction(context.Background(), tc.args)
			test.That(t, err, test.ShouldBeError, tc.expectedErr)
		})
	}
}

func TestDataSourceTypeToProto(t *testing.T) {
	t.Run("maps each name to its proto enum when in the allowed set", func(t *testing.T) {
		all := []string{standardDataSourceType, hotStorageDataSourceType, pipelineSinkDataSourceType}
		cases := map[string]datapb.TabularDataSourceType{
			standardDataSourceType:     datapb.TabularDataSourceType_TABULAR_DATA_SOURCE_TYPE_STANDARD,
			hotStorageDataSourceType:   datapb.TabularDataSourceType_TABULAR_DATA_SOURCE_TYPE_HOT_STORAGE,
			pipelineSinkDataSourceType: datapb.TabularDataSourceType_TABULAR_DATA_SOURCE_TYPE_PIPELINE_SINK,
		}
		for name, want := range cases {
			got, err := dataSourceTypeToProto(name, all)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, got, test.ShouldEqual, want)
		}
	})

	t.Run("rejects names not in the caller's allowed set", func(t *testing.T) {
		// pipelinesink is a known proto value, but not a valid source for creating a pipeline.
		_, err := dataSourceTypeToProto(pipelineSinkDataSourceType, pipelineDataSourceTypes)
		test.That(t, err, test.ShouldBeError, fmt.Errorf(
			"invalid data source type: %q. Supported values: %v",
			pipelineSinkDataSourceType, pipelineDataSourceTypes))
	})

	t.Run("rejects unknown and empty names", func(t *testing.T) {
		for _, name := range []string{"unknown", ""} {
			_, err := dataSourceTypeToProto(name, tabularDataByMQLDataSourceTypes)
			test.That(t, err, test.ShouldBeError, fmt.Errorf(
				"invalid data source type: %q. Supported values: %v",
				name, tabularDataByMQLDataSourceTypes))
		}
	})
}

func TestDataQueryBinaryAction(t *testing.T) {
	page := &datapb.BinaryDataByFilterResponse{
		Data: []*datapb.BinaryData{
			{Metadata: &datapb.BinaryMetadata{BinaryDataId: "bin-1", FileName: "a.jpg"}},
		},
	}

	t.Run("writes metadata to stdout as NDJSON and pages until exhausted", func(t *testing.T) {
		var capturedReq *datapb.BinaryDataByFilterRequest
		calls := 0
		dsc := &inject.DataServiceClient{
			BinaryDataByFilterFunc: func(ctx context.Context, in *datapb.BinaryDataByFilterRequest, opts ...grpc.CallOption,
			) (*datapb.BinaryDataByFilterResponse, error) {
				capturedReq = in
				calls++
				if calls == 1 {
					return page, nil
				}
				return &datapb.BinaryDataByFilterResponse{}, nil
			},
		}

		_, ac, out, errOut := setup(&inject.AppServiceClient{}, dsc, nil, nil, "token")
		err := ac.dataQueryBinaryAction(context.Background(), dataQueryBinaryArgs{}, &datapb.Filter{PartId: "p1"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(errOut.messages), test.ShouldEqual, 0)
		test.That(t, calls, test.ShouldEqual, 2)
		test.That(t, capturedReq.GetDataRequest().GetFilter().GetPartId(), test.ShouldEqual, "p1")

		// Parse NDJSON back into maps since protojson's spacing isn't stable.
		var actual []map[string]interface{}
		decoder := json.NewDecoder(strings.NewReader(strings.Join(out.messages, "")))
		for decoder.More() {
			var row map[string]interface{}
			test.That(t, decoder.Decode(&row), test.ShouldBeNil)
			actual = append(actual, row)
		}
		test.That(t, len(actual), test.ShouldEqual, 1)
	})

	t.Run("stops once --limit results have been written", func(t *testing.T) {
		var capturedReq *datapb.BinaryDataByFilterRequest
		calls := 0
		// Return a full page every call so the loop can only end by honoring --limit.
		dsc := &inject.DataServiceClient{
			BinaryDataByFilterFunc: func(ctx context.Context, in *datapb.BinaryDataByFilterRequest, opts ...grpc.CallOption,
			) (*datapb.BinaryDataByFilterResponse, error) {
				capturedReq = in
				calls++
				return page, nil
			},
		}

		_, ac, _, _ := setup(&inject.AppServiceClient{}, dsc, nil, nil, "token")
		err := ac.dataQueryBinaryAction(context.Background(), dataQueryBinaryArgs{Limit: 1}, &datapb.Filter{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capturedReq.GetDataRequest().GetLimit(), test.ShouldEqual, uint64(1))
		test.That(t, calls, test.ShouldEqual, 1)
	})
}
