package app

import (
	"context"
	"time"

	pb "go.viam.com/api/app/mltraining/v1"
	"go.viam.com/utils/rpc"
	"google.golang.org/genproto/googleapis/rpc/status"
)

// TrainingStatus respresents the status of a training job.
type TrainingStatus int

const (
	// TrainingStatusUnspecified respresents an unspecified training status.
	TrainingStatusUnspecified TrainingStatus = iota
	// TrainingStatusPending respresents a pending training job.
	TrainingStatusPending    
	// TrainingStatusInProgress respresents a training job that is in progress.
	TrainingStatusInProgress 
	// TrainingStatusCompleted respresents a completed training job.
	TrainingStatusCompleted   
	// TrainingStatusFailed respresents a failed training job.
	TrainingStatusFailed      
	// TrainingStatusCanceled respresents a canceled training job.
	TrainingStatusCanceled    
	// TrainingStatusCanceling respresents a training job that is being canceled.
	TrainingStatusCanceling   
)

// TrainingJobMetadata contains the metadata for a training job.
type TrainingJobMetadata struct {
	Id                  string                 
	DatasetId           string                 
	OrganizationId      string                 
	ModelName           string                 
	ModelVersion        string                 
	ModelType           ModelType              
	ModelFramework      ModelFramework         
	IsCustomJob         bool                   
	RegistryItemId      string                 
	RegistryItemVersion string                 
	Status              TrainingStatus         
	ErrorStatus         *status.Status         
	CreatedOn           *time.Time
	LastModified        *time.Time
	TrainingStarted     *time.Time
	TrainingEnded       *time.Time
	SyncedModelId       string                 
	Tags                []string               
}

// GetTrainingJobLogsOptions contains optional parameters for GetTrainingJobLogs.
type GetTrainingJobLogsOptions struct {
	PageToken *string
}

// TrainingJobLogEntry is a log entry from a training job.
type TrainingJobLogEntry struct {
	Level string
	Time *time.Time
	Message string
}

// MLTrainingClient is a gRPC client for method calls to the ML Training API.
type MLTrainingClient struct {
	client pb.MLTrainingServiceClient
}

func trainingStatusFromProto(status pb.TrainingStatus) TrainingStatus {
	switch status {
	case pb.TrainingStatus_TRAINING_STATUS_UNSPECIFIED:
		return TrainingStatusUnspecified
	case pb.TrainingStatus_TRAINING_STATUS_PENDING:
		return TrainingStatusPending    
	case pb.TrainingStatus_TRAINING_STATUS_IN_PROGRESS:
		return TrainingStatusInProgress 
	case pb.TrainingStatus_TRAINING_STATUS_COMPLETED:
		return TrainingStatusCompleted   
	case pb.TrainingStatus_TRAINING_STATUS_FAILED:
		return TrainingStatusFailed      
	case pb.TrainingStatus_TRAINING_STATUS_CANCELED:
		return TrainingStatusCanceled    
	case pb.TrainingStatus_TRAINING_STATUS_CANCELING:
		return TrainingStatusCanceling
	}
	return TrainingStatusUnspecified
}

func trainingStatusToProto(status TrainingStatus) pb.TrainingStatus {
	switch status {
	case TrainingStatusUnspecified:
		return pb.TrainingStatus_TRAINING_STATUS_UNSPECIFIED
	case TrainingStatusPending:
		return pb.TrainingStatus_TRAINING_STATUS_PENDING    
	case TrainingStatusInProgress:
		return pb.TrainingStatus_TRAINING_STATUS_IN_PROGRESS 
	case TrainingStatusCompleted:
		return pb.TrainingStatus_TRAINING_STATUS_COMPLETED   
	case TrainingStatusFailed:
		return pb.TrainingStatus_TRAINING_STATUS_FAILED      
	case TrainingStatusCanceled:
		return pb.TrainingStatus_TRAINING_STATUS_CANCELED    
	case TrainingStatusCanceling:
		return pb.TrainingStatus_TRAINING_STATUS_CANCELING
	}
	return pb.TrainingStatus_TRAINING_STATUS_UNSPECIFIED
}

func newMLTrainingClient(conn rpc.ClientConn) *MLTrainingClient {
	return &MLTrainingClient{client: pb.NewMLTrainingServiceClient(conn)}
}

// SubmitTrainingJob submits a training job request and returns its ID.
func (c *MLTrainingClient) SubmitTrainingJob(ctx context.Context, datasetID, organizationID, modelName, modelVersion string, modelType ModelType, tags []string) (string, error) {
	resp, err := c.client.SubmitTrainingJob(ctx, &pb.SubmitTrainingJobRequest{
		DatasetId: datasetID,
		OrganizationId: organizationID,
		ModelName: modelName,
		ModelVersion: modelVersion,
		ModelType: modelTypeToProto(modelType),
		Tags: tags,
	})
	if err != nil {
		return "", err
	}
	return resp.Id, nil
}

// SubmitCustomTrainingJob submits a custom training job request and returns its ID.
func (c *MLTrainingClient) SubmitCustomTrainingJob(ctx context.Context, datasetID, registryItemID, registryItemVersion, organizationID, modelName, modelVersion string, arguments map[string]string) (string, error) {
	resp, err := c.client.SubmitCustomTrainingJob(ctx, &pb.SubmitCustomTrainingJobRequest{
		DatasetId: datasetID,
		RegistryItemId: registryItemID,
		RegistryItemVersion: registryItemVersion,
		OrganizationId: organizationID,
		ModelName: modelName,
		ModelVersion: modelVersion,
		Arguments: arguments,
	})
	if err != nil {
		return "", err
	}
	return resp.Id, nil
}

// GetTrainingJob retrieves a training job by its ID.
func (c *MLTrainingClient) GetTrainingJob(ctx context.Context, id string) (*TrainingJobMetadata, error) {
	resp, err := c.client.GetTrainingJob(ctx, &pb.GetTrainingJobRequest{
		Id: id,
	})
	if err != nil {
		return nil, err
	}
	return trainingJobMetadataFromProto(resp.Metadata), nil
}

// ListTrainingJobs lists training jobs for a given organization ID and training status.
func (c *MLTrainingClient) ListTrainingJobs(ctx context.Context, organizationID string, status TrainingStatus) ([]*TrainingJobMetadata, error) {
	resp, err := c.client.ListTrainingJobs(ctx, &pb.ListTrainingJobsRequest{
		OrganizationId: organizationID,
		Status: trainingStatusToProto(status),
	})
	if err != nil {
		return nil, err
	}
	var jobs []*TrainingJobMetadata
	for _, job := range(resp.Jobs) {
		jobs = append(jobs, trainingJobMetadataFromProto(job))
	}
	return jobs, nil
}

// CancelTrainingJob cancels a training job that has not yet completed.
func (c *MLTrainingClient) CancelTrainingJob(ctx context.Context, id string) error {
	_, err := c.client.CancelTrainingJob(ctx, &pb.CancelTrainingJobRequest{
		Id: id,
	})
	return err
}

// DeleteCompletedTrainingJob removes a completed training job from the database, whether the job succeeded or failed.
func (c *MLTrainingClient) DeleteCompletedTrainingJob(ctx context.Context, id string) error {
	_, err := c.client.DeleteCompletedTrainingJob(ctx, &pb.DeleteCompletedTrainingJobRequest{
		Id: id,
	})
	return err
}

// GetTrainingJobLogs gets the logs and the next page token for a given custom training job.
func (c *MLTrainingClient) GetTrainingJobLogs(ctx context.Context, id string, opts *GetTrainingJobLogsOptions) ([]*TrainingJobLogEntry, string, error) {
	var token *string
	if opts != nil {
		token = opts.PageToken
	}
	resp, err := c.client.GetTrainingJobLogs(ctx, &pb.GetTrainingJobLogsRequest{
		Id: id,
		PageToken: token,
	})
	if err != nil {
		return nil, "", err
	}

	var logs []*TrainingJobLogEntry
	for _, log := range(resp.Logs) {
		logs = append(logs, trainingJobLogEntryFromProto(log))
	}
	return logs, resp.NextPageToken, nil
}

func trainingJobLogEntryFromProto (log *pb.TrainingJobLogEntry) *TrainingJobLogEntry {
	time := log.Time.AsTime()
	return &TrainingJobLogEntry{
		Level: log.Level,
		Time: &time,
		Message: log.Message,
	}
}

func trainingJobMetadataFromProto(metadata *pb.TrainingJobMetadata) *TrainingJobMetadata {
	createdOn := metadata.CreatedOn.AsTime()
	lastModified := metadata.LastModified.AsTime()
	started := metadata.TrainingStarted.AsTime()
	ended := metadata.TrainingEnded.AsTime()
	return &TrainingJobMetadata{
		Id: metadata.Id,
		DatasetId: metadata.DatasetId,
		OrganizationId: metadata.OrganizationId,
		ModelName: metadata.ModelName,
		ModelVersion: metadata.ModelVersion,
		ModelType: modelTypeFromProto(metadata.ModelType),
		ModelFramework: modelFrameworkFromProto(metadata.ModelFramework),
		IsCustomJob: metadata.IsCustomJob,
		RegistryItemId: metadata.RegistryItemId,
		RegistryItemVersion: metadata.Id,
		Status: trainingStatusFromProto(metadata.Status),
		ErrorStatus: metadata.ErrorStatus,
		CreatedOn: &createdOn,
		LastModified: &lastModified,
		TrainingStarted: &started,
		TrainingEnded: &ended,
		SyncedModelId: metadata.SyncedModelId,
		Tags: metadata.Tags,
	}
}
