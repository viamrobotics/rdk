package inject

import (
	"context"

	buildpb "go.viam.com/api/app/build/v1"
	"google.golang.org/grpc"
)

// BuildServiceClient is an injectable buildpb.BuildServiceClient.
type BuildServiceClient struct {
	buildpb.BuildServiceClient
	ListJobsFunc func(ctx context.Context, in *buildpb.ListJobsRequest,
		opts ...grpc.CallOption) (*buildpb.ListJobsResponse, error)
	StartBuildFunc func(ctx context.Context, in *buildpb.StartBuildRequest,
		opts ...grpc.CallOption) (*buildpb.StartBuildResponse, error)
	StartSourceUploadBuildFunc func(
		ctx context.Context, opts ...grpc.CallOption,
	) (buildpb.BuildService_StartSourceUploadBuildClient, error)
}

// ListJobs calls the injected ListJobsFunc or the real version.
func (bsc *BuildServiceClient) ListJobs(ctx context.Context, in *buildpb.ListJobsRequest,
	opts ...grpc.CallOption,
) (*buildpb.ListJobsResponse, error) {
	if bsc.ListJobsFunc == nil {
		return bsc.ListJobs(ctx, in, opts...)
	}
	return bsc.ListJobsFunc(ctx, in, opts...)
}

// StartBuild calls the injected StartBuildFunc or the real version.
func (bsc *BuildServiceClient) StartBuild(ctx context.Context, in *buildpb.StartBuildRequest,
	opts ...grpc.CallOption,
) (*buildpb.StartBuildResponse, error) {
	if bsc.StartBuildFunc == nil {
		return bsc.StartBuild(ctx, in, opts...)
	}
	return bsc.StartBuildFunc(ctx, in, opts...)
}

// StartSourceUploadBuild calls the injected StartSourceUploadBuildFunc or the
// real version. Tests can return a fake stream that captures Send calls.
func (bsc *BuildServiceClient) StartSourceUploadBuild(
	ctx context.Context, opts ...grpc.CallOption,
) (buildpb.BuildService_StartSourceUploadBuildClient, error) {
	if bsc.StartSourceUploadBuildFunc == nil {
		return bsc.BuildServiceClient.StartSourceUploadBuild(ctx, opts...)
	}
	return bsc.StartSourceUploadBuildFunc(ctx, opts...)
}
