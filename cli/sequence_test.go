package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"
	datapb "go.viam.com/api/app/data/v1"
	datasetpb "go.viam.com/api/app/dataset/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/testutils/inject"
)

func TestParseSequenceResources(t *testing.T) {
	tests := []struct {
		name    string
		values  []string
		want    []*datapb.SequenceResourceFilter
		wantErr bool
	}{
		{
			name:   "single",
			values: []string{"cam:ReadImage"},
			want:   []*datapb.SequenceResourceFilter{{ResourceName: "cam", MethodName: "ReadImage"}},
		},
		{
			name:   "same resource with multiple methods",
			values: []string{"cam:ReadImage", "cam:GetPointCloud", "arm:JointPositions"},
			want: []*datapb.SequenceResourceFilter{
				{ResourceName: "cam", MethodName: "ReadImage"},
				{ResourceName: "cam", MethodName: "GetPointCloud"},
				{ResourceName: "arm", MethodName: "JointPositions"},
			},
		},
		{name: "missing colon", values: []string{"cam"}, wantErr: true},
		{name: "empty name", values: []string{":ReadImage"}, wantErr: true},
		{name: "empty method", values: []string{"cam:"}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseSequenceResources(tc.values)
			if tc.wantErr {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, "<resource-name>:<method>")
				return
			}
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(got), test.ShouldEqual, len(tc.want))
			for i := range got {
				test.That(t, got[i].GetResourceName(), test.ShouldEqual, tc.want[i].GetResourceName())
				test.That(t, got[i].GetMethodName(), test.ShouldEqual, tc.want[i].GetMethodName())
			}
		})
	}
}

func TestDataSequenceCreate(t *testing.T) {
	t.Run("sends request fields", func(t *testing.T) {
		var gotReq *datapb.CreateSequenceRequest
		dsc := &inject.DataServiceClient{
			CreateSequenceFunc: func(_ context.Context, in *datapb.CreateSequenceRequest, _ ...grpc.CallOption,
			) (*datapb.CreateSequenceResponse, error) {
				gotReq = in
				return &datapb.CreateSequenceResponse{Id: "seq-1"}, nil
			},
		}
		_, ac, out, _ := setup(&inject.AppServiceClient{}, dsc, nil, nil, "token")

		err := ac.createSequence(context.Background(), dataSequenceCreateArgs{
			PartID:    "part-1",
			Resources: []string{"cam:ReadImage", "arm:JointPositions"},
			Tags:      []string{"pick"},
			Start:     "2026-10-01T00:00:00Z",
			End:       "2026-10-01T00:05:00Z",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gotReq.GetPartId(), test.ShouldEqual, "part-1")
		test.That(t, formatSequenceResources(gotReq.GetResources()), test.ShouldEqual, "cam:ReadImage,arm:JointPositions")
		test.That(t, gotReq.GetSequenceTags(), test.ShouldResemble, []string{"pick"})
		test.That(t, gotReq.GetStartTime().AsTime().Format("15:04"), test.ShouldEqual, "00:00")
		test.That(t, gotReq.GetEndTime().AsTime().Format("15:04"), test.ShouldEqual, "00:05")
		test.That(t, strings.Join(out.messages, ""), test.ShouldContainSubstring, "seq-1")
	})

	t.Run("missing end is an error", func(t *testing.T) {
		ac := &viamClient{dataClient: &inject.DataServiceClient{}}
		err := ac.createSequence(context.Background(), dataSequenceCreateArgs{
			PartID:    "part-1",
			Resources: []string{"cam:ReadImage"},
			Start:     "2026-10-01T00:00:00Z",
		})
		test.That(t, err, test.ShouldNotBeNil)
	})
}

func TestDataSequenceGet(t *testing.T) {
	dsc := &inject.DataServiceClient{
		GetSequenceFunc: func(_ context.Context, in *datapb.GetSequenceRequest, _ ...grpc.CallOption,
		) (*datapb.GetSequenceResponse, error) {
			return &datapb.GetSequenceResponse{Sequence: &datapb.Sequence{
				Id:        in.GetId(),
				PartId:    "part-1",
				Resources: []*datapb.SequenceResourceFilter{{ResourceName: "cam", MethodName: "ReadImage"}},
				StartTime: timestamppb.Now(),
				EndTime:   timestamppb.Now(),
			}}, nil
		},
	}
	_, ac, out, _ := setup(&inject.AppServiceClient{}, dsc, nil, nil, "token")

	test.That(t, ac.getSequence(context.Background(), "seq-1"), test.ShouldBeNil)
	output := strings.Join(out.messages, "")
	test.That(t, output, test.ShouldContainSubstring, "ID: seq-1")
	test.That(t, output, test.ShouldContainSubstring, "Resources: cam:ReadImage")
}

func TestDataSequenceUpdate(t *testing.T) {
	tests := []struct {
		name      string
		flags     map[string]any
		wantPaths []string
		wantErr   bool
	}{
		{
			name:      "tags only",
			flags:     map[string]any{generalFlagTags: []string{"a", "b"}},
			wantPaths: []string{"sequence_tags"},
		},
		{
			name: "resources and times",
			flags: map[string]any{
				sequenceFlagResources: []string{"cam:ReadImage"},
				generalFlagStart:      "2026-10-01T00:00:00Z",
				generalFlagEnd:        "2026-10-01T00:05:00Z",
			},
			wantPaths: []string{"resources", "start_time", "end_time"},
		},
		{
			name:    "no fields set",
			flags:   map[string]any{},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotReq *datapb.UpdateSequenceRequest
			dsc := &inject.DataServiceClient{
				UpdateSequenceFunc: func(_ context.Context, in *datapb.UpdateSequenceRequest, _ ...grpc.CallOption,
				) (*datapb.UpdateSequenceResponse, error) {
					gotReq = in
					return &datapb.UpdateSequenceResponse{}, nil
				},
			}
			flags := map[string]any{sequenceFlagSequenceID: "seq-1"}
			for k, v := range tc.flags {
				flags[k] = v
			}
			cCtx, ac, out, _ := setup(&inject.AppServiceClient{}, dsc, nil, flags, "token")

			err := ac.updateSequence(context.Background(), parseStructFromCtx[dataSequenceUpdateArgs](cCtx))
			if tc.wantErr {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, gotReq, test.ShouldBeNil)
				test.That(t, out.messages, test.ShouldBeEmpty)
				return
			}
			test.That(t, err, test.ShouldBeNil)
			test.That(t, gotReq.GetId(), test.ShouldEqual, "seq-1")
			test.That(t, gotReq.GetFieldMask().GetPaths(), test.ShouldResemble, tc.wantPaths)
		})
	}
}

// Empty values are passed as raw CLI args because buildTestCmd drops empty flag values.
func TestDataSequenceUpdateEmptyValues(t *testing.T) {
	tests := []struct {
		name     string
		cliArgs  []string
		wantTags []string
		wantErr  string
	}{
		{
			name:     "empty tags clears tags",
			cliArgs:  []string{"--" + generalFlagTags, ""},
			wantTags: []string{},
		},
		{
			name:     "blank entries are dropped from tags",
			cliArgs:  []string{"--" + generalFlagTags, "a,,b"},
			wantTags: []string{"a", "b"},
		},
		{
			name:    "empty start is rejected before calling the server",
			cliArgs: []string{"--" + generalFlagStart, ""},
			wantErr: "--" + generalFlagStart,
		},
		{
			name:    "empty end is rejected before calling the server",
			cliArgs: []string{"--" + generalFlagEnd, ""},
			wantErr: "--" + generalFlagEnd,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotReq *datapb.UpdateSequenceRequest
			dsc := &inject.DataServiceClient{
				UpdateSequenceFunc: func(_ context.Context, in *datapb.UpdateSequenceRequest, _ ...grpc.CallOption,
				) (*datapb.UpdateSequenceResponse, error) {
					gotReq = in
					return &datapb.UpdateSequenceResponse{}, nil
				},
			}
			flags := map[string]any{
				sequenceFlagSequenceID: "seq-1",
				generalFlagTags:        []string{},
				generalFlagStart:       "",
				generalFlagEnd:         "",
			}
			cCtx, ac, _, _ := setup(&inject.AppServiceClient{}, dsc, nil, flags, "token", tc.cliArgs...)

			err := ac.updateSequence(context.Background(), parseStructFromCtx[dataSequenceUpdateArgs](cCtx))
			if tc.wantErr != "" {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.wantErr)
				test.That(t, gotReq, test.ShouldBeNil)
				return
			}
			test.That(t, err, test.ShouldBeNil)
			test.That(t, gotReq.GetFieldMask().GetPaths(), test.ShouldResemble, []string{"sequence_tags"})
			test.That(t, gotReq.GetSequenceTags(), test.ShouldResemble, tc.wantTags)
		})
	}
}

func TestDataSequenceDelete(t *testing.T) {
	var deleted []string
	dsc := &inject.DataServiceClient{
		DeleteSequenceFunc: func(_ context.Context, in *datapb.DeleteSequenceRequest, _ ...grpc.CallOption,
		) (*datapb.DeleteSequenceResponse, error) {
			if in.GetId() == "bad" {
				return nil, errors.New("not found")
			}
			deleted = append(deleted, in.GetId())
			return &datapb.DeleteSequenceResponse{}, nil
		},
	}
	_, ac, out, _ := setup(&inject.AppServiceClient{}, dsc, nil, nil, "token")

	err := ac.deleteSequences(context.Background(), []string{"seq-1", "bad", "seq-2"})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "bad")
	test.That(t, deleted, test.ShouldResemble, []string{"seq-1"})
	test.That(t, strings.Join(out.messages, ""), test.ShouldEqual, "Deleted sequence seq-1\n")
}

func TestDatasetSequenceAddRemove(t *testing.T) {
	var added *datapb.AddSequencesToDatasetRequest
	var removed *datapb.RemoveSequencesFromDatasetRequest
	dsc := &inject.DataServiceClient{
		AddSequencesToDatasetFunc: func(_ context.Context, in *datapb.AddSequencesToDatasetRequest, _ ...grpc.CallOption,
		) (*datapb.AddSequencesToDatasetResponse, error) {
			added = in
			return &datapb.AddSequencesToDatasetResponse{}, nil
		},
		RemoveSequencesFromDatasetFunc: func(_ context.Context, in *datapb.RemoveSequencesFromDatasetRequest, _ ...grpc.CallOption,
		) (*datapb.RemoveSequencesFromDatasetResponse, error) {
			removed = in
			return &datapb.RemoveSequencesFromDatasetResponse{}, nil
		},
	}
	_, ac, out, _ := setup(&inject.AppServiceClient{}, dsc, nil, nil, "token")

	ids := []string{"seq-1", "seq-2"}
	test.That(t, ac.addSequencesToDataset(context.Background(), "ds-1", ids), test.ShouldBeNil)
	test.That(t, added.GetDatasetId(), test.ShouldEqual, "ds-1")
	test.That(t, added.GetSequenceIds(), test.ShouldResemble, ids)

	test.That(t, ac.removeSequencesFromDataset(context.Background(), "ds-1", ids[:1]), test.ShouldBeNil)
	test.That(t, removed.GetDatasetId(), test.ShouldEqual, "ds-1")
	test.That(t, removed.GetSequenceIds(), test.ShouldResemble, ids[:1])

	output := strings.Join(out.messages, "")
	test.That(t, output, test.ShouldContainSubstring, "Added 2 sequences to dataset ds-1")
	test.That(t, output, test.ShouldContainSubstring, "Removed 1 sequence from dataset ds-1")
}

func TestDatasetCreateType(t *testing.T) {
	t.Run("parse", func(t *testing.T) {
		got, err := parseDatasetType(datasetTypeBinary)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, got, test.ShouldEqual, datasetpb.DatasetType_DATASET_TYPE_BINARY_DATA)

		got, err = parseDatasetType(datasetTypeSequence)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, got, test.ShouldEqual, datasetpb.DatasetType_DATASET_TYPE_SEQUENCE_DATA)

		_, err = parseDatasetType("tabular")
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("invalid type fails before connecting", func(t *testing.T) {
		err := DatasetCreateAction(context.Background(), &cli.Command{}, datasetCreateArgs{OrgID: "org-1", Name: "ds", Type: "tabular"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "invalid dataset type")
	})

	t.Run("create sends type", func(t *testing.T) {
		var gotReq *datasetpb.CreateDatasetRequest
		_, ac, out, _ := setup(&inject.AppServiceClient{}, nil, nil, nil, "token")
		ac.datasetClient = &inject.DatasetServiceClient{
			CreateDatasetFunc: func(_ context.Context, in *datasetpb.CreateDatasetRequest, _ ...grpc.CallOption,
			) (*datasetpb.CreateDatasetResponse, error) {
				gotReq = in
				return &datasetpb.CreateDatasetResponse{Id: "ds-1"}, nil
			},
		}
		test.That(t, ac.createDataset("org-1", "ds", datasetpb.DatasetType_DATASET_TYPE_SEQUENCE_DATA), test.ShouldBeNil)
		test.That(t, gotReq.GetType(), test.ShouldEqual, datasetpb.DatasetType_DATASET_TYPE_SEQUENCE_DATA)
		test.That(t, strings.Join(out.messages, ""), test.ShouldContainSubstring, "Created sequence dataset ds")
	})

	t.Run("list prints type", func(t *testing.T) {
		_, ac, out, _ := setup(&inject.AppServiceClient{}, nil, nil, nil, "token")
		ac.datasetClient = &inject.DatasetServiceClient{
			ListDatasetsByOrganizationIDFunc: func(_ context.Context, _ *datasetpb.ListDatasetsByOrganizationIDRequest,
				_ ...grpc.CallOption,
			) (*datasetpb.ListDatasetsByOrganizationIDResponse, error) {
				return &datasetpb.ListDatasetsByOrganizationIDResponse{Datasets: []*datasetpb.Dataset{
					{Id: "ds-1", Name: "imgs", Type: datasetpb.DatasetType_DATASET_TYPE_BINARY_DATA},
					{Id: "ds-2", Name: "seqs", Type: datasetpb.DatasetType_DATASET_TYPE_SEQUENCE_DATA},
				}}, nil
			},
		}
		test.That(t, ac.listDatasetByOrg("org-1"), test.ShouldBeNil)
		output := strings.Join(out.messages, "")
		test.That(t, output, test.ShouldContainSubstring, "imgs (ID: ds-1, Organization ID: , Type: binary)")
		test.That(t, output, test.ShouldContainSubstring, "seqs (ID: ds-2, Organization ID: , Type: sequence)")
	})
}
