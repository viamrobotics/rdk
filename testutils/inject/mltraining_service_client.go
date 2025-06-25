package inject

import (
	"context"

	mltrainingpb "go.viam.com/api/app/mltraining/v1"
	"google.golang.org/grpc"
)

// MLTrainingServiceClient represents a fake instance of a ML training service client.
type MLTrainingServiceClient struct {
	mltrainingpb.MLTrainingServiceClient
	SubmitTrainingJobFunc func(ctx context.Context, in *mltrainingpb.SubmitTrainingJobRequest,
		opts ...grpc.CallOption) (*mltrainingpb.SubmitTrainingJobResponse, error)
	SubmitCustomTrainingJobFunc func(ctx context.Context, in *mltrainingpb.SubmitCustomTrainingJobRequest,
		opts ...grpc.CallOption) (*mltrainingpb.SubmitCustomTrainingJobResponse, error)
	GetTrainingJobFunc func(ctx context.Context, in *mltrainingpb.GetTrainingJobRequest,
		opts ...grpc.CallOption) (*mltrainingpb.GetTrainingJobResponse, error)
	ListTrainingJobsFunc func(ctx context.Context, in *mltrainingpb.ListTrainingJobsRequest,
		opts ...grpc.CallOption) (*mltrainingpb.ListTrainingJobsResponse, error)
	CancelTrainingJobFunc func(ctx context.Context, in *mltrainingpb.CancelTrainingJobRequest,
		opts ...grpc.CallOption) (*mltrainingpb.CancelTrainingJobResponse, error)
	DeleteCompletedTrainingJobFunc func(ctx context.Context, in *mltrainingpb.DeleteCompletedTrainingJobRequest,
		opts ...grpc.CallOption) (*mltrainingpb.DeleteCompletedTrainingJobResponse, error)
	GetTrainingJobLogsFunc func(ctx context.Context, in *mltrainingpb.GetTrainingJobLogsRequest,
		opts ...grpc.CallOption) (*mltrainingpb.GetTrainingJobLogsResponse, error)
}

// SubmitTrainingJob calls the injected SubmitTrainingJob or the real version.
func (client *MLTrainingServiceClient) SubmitTrainingJob(ctx context.Context, in *mltrainingpb.SubmitTrainingJobRequest,
	opts ...grpc.CallOption,
) (*mltrainingpb.SubmitTrainingJobResponse, error) {
	if client.SubmitTrainingJobFunc == nil {
		return client.MLTrainingServiceClient.SubmitTrainingJob(ctx, in, opts...)
	}
	return client.SubmitTrainingJobFunc(ctx, in, opts...)
}

// SubmitCustomTrainingJob calls the injected SubmitCustomTrainingJob or the real version.
func (client *MLTrainingServiceClient) SubmitCustomTrainingJob(ctx context.Context, in *mltrainingpb.SubmitCustomTrainingJobRequest,
	opts ...grpc.CallOption,
) (*mltrainingpb.SubmitCustomTrainingJobResponse, error) {
	if client.SubmitCustomTrainingJobFunc == nil {
		return client.MLTrainingServiceClient.SubmitCustomTrainingJob(ctx, in, opts...)
	}
	return client.SubmitCustomTrainingJobFunc(ctx, in, opts...)
}

// GetTrainingJob calls the injected GetTrainingJob or the real version.
func (client *MLTrainingServiceClient) GetTrainingJob(ctx context.Context, in *mltrainingpb.GetTrainingJobRequest,
	opts ...grpc.CallOption,
) (*mltrainingpb.GetTrainingJobResponse, error) {
	if client.GetTrainingJobFunc == nil {
		return client.MLTrainingServiceClient.GetTrainingJob(ctx, in, opts...)
	}
	return client.GetTrainingJobFunc(ctx, in, opts...)
}

// ListTrainingJobs calls the injected ListTrainingJobs or the real version.
func (client *MLTrainingServiceClient) ListTrainingJobs(ctx context.Context, in *mltrainingpb.ListTrainingJobsRequest,
	opts ...grpc.CallOption,
) (*mltrainingpb.ListTrainingJobsResponse, error) {
	if client.ListTrainingJobsFunc == nil {
		return client.MLTrainingServiceClient.ListTrainingJobs(ctx, in, opts...)
	}
	return client.ListTrainingJobsFunc(ctx, in, opts...)
}

// CancelTrainingJob calls the injected CancelTrainingJob or the real version.
func (client *MLTrainingServiceClient) CancelTrainingJob(ctx context.Context, in *mltrainingpb.CancelTrainingJobRequest,
	opts ...grpc.CallOption,
) (*mltrainingpb.CancelTrainingJobResponse, error) {
	if client.CancelTrainingJobFunc == nil {
		return client.MLTrainingServiceClient.CancelTrainingJob(ctx, in, opts...)
	}
	return client.CancelTrainingJobFunc(ctx, in, opts...)
}

// DeleteCompletedTrainingJob calls the injected DeleteCompletedTrainingJob or the real version.
func (client *MLTrainingServiceClient) DeleteCompletedTrainingJob(ctx context.Context, in *mltrainingpb.DeleteCompletedTrainingJobRequest,
	opts ...grpc.CallOption,
) (*mltrainingpb.DeleteCompletedTrainingJobResponse, error) {
	if client.DeleteCompletedTrainingJobFunc == nil {
		return client.MLTrainingServiceClient.DeleteCompletedTrainingJob(ctx, in, opts...)
	}
	return client.DeleteCompletedTrainingJobFunc(ctx, in, opts...)
}

// GetTrainingJobLogs calls the injected GetTrainingJobLogs or the real version.
func (client *MLTrainingServiceClient) GetTrainingJobLogs(ctx context.Context, in *mltrainingpb.GetTrainingJobLogsRequest,
	opts ...grpc.CallOption,
) (*mltrainingpb.GetTrainingJobLogsResponse, error) {
	if client.GetTrainingJobLogsFunc == nil {
		return client.MLTrainingServiceClient.GetTrainingJobLogs(ctx, in, opts...)
	}
	return client.GetTrainingJobLogsFunc(ctx, in, opts...)
}
