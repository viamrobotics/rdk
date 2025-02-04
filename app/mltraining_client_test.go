package app

import (
	"context"
	"testing"

	pb "go.viam.com/api/app/mltraining/v1"
	"go.viam.com/test"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/testutils/inject"
)

const (
	jobID          = "job_id"
	isCustomJob    = true
	trainingStatus = TrainingStatusCompleted
	modelID        = "model_id"
	code           = 0
)

var (
	arguments      = map[string]string{"arg1": "one", "arg2": "two"}
	errorDetail, _ = anypb.New(&errdetails.ErrorInfo{
		Reason: "API_DISABLED",
		Domain: "googleapis.com",
		Metadata: map[string]string{
			"resource": "projects/123",
			"service":  "pubsub.googleapis.com",
		},
	})
	jobMetadata = TrainingJobMetadata{
		ID:                  jobID,
		DatasetID:           datasetID,
		OrganizationID:      organizationID,
		ModelName:           name,
		ModelVersion:        version,
		ModelType:           modelType,
		ModelFramework:      modelFramework,
		IsCustomJob:         isCustomJob,
		RegistryItemID:      itemID,
		RegistryItemVersion: version,
		Status:              trainingStatus,
		ErrorStatus: &status.Status{
			Code:    int32(3),
			Message: message,
			Details: []*anypb.Any{errorDetail},
		},
		CreatedOn:       &createdOn,
		LastModified:    &lastUpdated,
		TrainingStarted: &start,
		TrainingEnded:   &end,
		SyncedModelID:   modelID,
		Tags:            tags,
	}
	pbJobMetadata = &pb.TrainingJobMetadata{
		Id:                  jobMetadata.ID,
		DatasetId:           jobMetadata.DatasetID,
		OrganizationId:      jobMetadata.OrganizationID,
		ModelName:           jobMetadata.ModelName,
		ModelVersion:        jobMetadata.ModelVersion,
		ModelType:           modelTypeToProto(jobMetadata.ModelType),
		ModelFramework:      modelFrameworkToProto(jobMetadata.ModelFramework),
		IsCustomJob:         jobMetadata.IsCustomJob,
		RegistryItemId:      jobMetadata.RegistryItemID,
		RegistryItemVersion: jobMetadata.RegistryItemVersion,
		Status:              trainingStatusToProto(jobMetadata.Status),
		ErrorStatus:         jobMetadata.ErrorStatus,
		CreatedOn:           pbCreatedOn,
		LastModified:        timestamppb.New(lastUpdated),
		TrainingStarted:     pbStart,
		TrainingEnded:       pbEnd,
		SyncedModelId:       jobMetadata.SyncedModelID,
		Tags:                jobMetadata.Tags,
	}
	trainingLogEntry = TrainingJobLogEntry{
		Level:   level,
		Time:    &timestamp,
		Message: message,
	}
)

func createMLTrainingGrpcClient() *inject.MLTrainingServiceClient {
	return &inject.MLTrainingServiceClient{}
}

func TestMLTrainingClient(t *testing.T) {
	grpcClient := createMLTrainingGrpcClient()
	client := MLTrainingClient{client: grpcClient}

	t.Run("SubmitTrainingJob", func(t *testing.T) {
		grpcClient.SubmitTrainingJobFunc = func(
			ctx context.Context, in *pb.SubmitTrainingJobRequest, opts ...grpc.CallOption,
		) (*pb.SubmitTrainingJobResponse, error) {
			test.That(t, in.DatasetId, test.ShouldEqual, datasetID)
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			test.That(t, in.ModelName, test.ShouldEqual, name)
			test.That(t, in.ModelVersion, test.ShouldEqual, version)
			test.That(t, in.ModelType, test.ShouldEqual, pbModelType)
			test.That(t, in.Tags, test.ShouldResemble, tags)
			return &pb.SubmitTrainingJobResponse{
				Id: jobID,
			}, nil
		}
		resp, err := client.SubmitTrainingJob(
			context.Background(), SubmitTrainingJobArgs{datasetID, organizationID, name, version}, modelType, tags,
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldEqual, jobID)
	})

	t.Run("SubmitCustomTrainingJob", func(t *testing.T) {
		grpcClient.SubmitCustomTrainingJobFunc = func(
			ctx context.Context, in *pb.SubmitCustomTrainingJobRequest, opts ...grpc.CallOption,
		) (*pb.SubmitCustomTrainingJobResponse, error) {
			test.That(t, in.DatasetId, test.ShouldEqual, datasetID)
			test.That(t, in.RegistryItemId, test.ShouldEqual, itemID)
			test.That(t, in.RegistryItemVersion, test.ShouldEqual, version)
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			test.That(t, in.ModelName, test.ShouldEqual, name)
			test.That(t, in.ModelVersion, test.ShouldEqual, version)
			test.That(t, in.Arguments, test.ShouldEqual, arguments)
			return &pb.SubmitCustomTrainingJobResponse{
				Id: jobID,
			}, nil
		}
		resp, err := client.SubmitCustomTrainingJob(
			context.Background(), SubmitTrainingJobArgs{datasetID, organizationID, name, version}, itemID, version, arguments,
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldEqual, jobID)
	})

	t.Run("GetTrainingJob", func(t *testing.T) {
		grpcClient.GetTrainingJobFunc = func(
			ctx context.Context, in *pb.GetTrainingJobRequest, opts ...grpc.CallOption,
		) (*pb.GetTrainingJobResponse, error) {
			test.That(t, in.Id, test.ShouldEqual, jobID)
			return &pb.GetTrainingJobResponse{
				Metadata: pbJobMetadata,
			}, nil
		}
		resp, err := client.GetTrainingJob(context.Background(), jobID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &jobMetadata)
	})

	t.Run("ListTrainingJobs", func(t *testing.T) {
		expectedJobs := []*TrainingJobMetadata{&jobMetadata}
		grpcClient.ListTrainingJobsFunc = func(
			ctx context.Context, in *pb.ListTrainingJobsRequest, opts ...grpc.CallOption,
		) (*pb.ListTrainingJobsResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			test.That(t, in.Status, test.ShouldEqual, trainingStatusToProto(trainingStatus))
			return &pb.ListTrainingJobsResponse{
				Jobs: []*pb.TrainingJobMetadata{pbJobMetadata},
			}, nil
		}
		resp, err := client.ListTrainingJobs(context.Background(), organizationID, trainingStatus)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, expectedJobs)
	})

	t.Run("CancelTrainingJob", func(t *testing.T) {
		grpcClient.CancelTrainingJobFunc = func(
			ctx context.Context, in *pb.CancelTrainingJobRequest, opts ...grpc.CallOption,
		) (*pb.CancelTrainingJobResponse, error) {
			test.That(t, in.Id, test.ShouldEqual, jobID)
			return &pb.CancelTrainingJobResponse{}, nil
		}
		err := client.CancelTrainingJob(context.Background(), jobID)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("DeleteCompletedTrainingJob", func(t *testing.T) {
		grpcClient.DeleteCompletedTrainingJobFunc = func(
			ctx context.Context, in *pb.DeleteCompletedTrainingJobRequest, opts ...grpc.CallOption,
		) (*pb.DeleteCompletedTrainingJobResponse, error) {
			test.That(t, in.Id, test.ShouldEqual, jobID)
			return &pb.DeleteCompletedTrainingJobResponse{}, nil
		}
		err := client.DeleteCompletedTrainingJob(context.Background(), jobID)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("GetTrainingJobLogs", func(t *testing.T) {
		expectedLogs := []*TrainingJobLogEntry{&trainingLogEntry}
		grpcClient.GetTrainingJobLogsFunc = func(
			ctx context.Context, in *pb.GetTrainingJobLogsRequest, opts ...grpc.CallOption,
		) (*pb.GetTrainingJobLogsResponse, error) {
			test.That(t, in.Id, test.ShouldEqual, jobID)
			test.That(t, in.PageToken, test.ShouldEqual, &pageToken)
			return &pb.GetTrainingJobLogsResponse{
				Logs: []*pb.TrainingJobLogEntry{
					{
						Level:   trainingLogEntry.Level,
						Time:    timestamppb.New(*trainingLogEntry.Time),
						Message: trainingLogEntry.Message,
					},
				},
				NextPageToken: pageToken,
			}, nil
		}
		logs, token, err := client.GetTrainingJobLogs(context.Background(), jobID, &GetTrainingJobLogsOptions{&pageToken})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, logs, test.ShouldResemble, expectedLogs)
		test.That(t, token, test.ShouldEqual, pageToken)
	})
}
