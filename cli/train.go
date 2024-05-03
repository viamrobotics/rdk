package cli

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	mltrainingpb "go.viam.com/api/app/mltraining/v1"
)

const (
	trainFlagJobID        = "job-id"
	trainFlagJobStatus    = "job-status"
	trainFlagModelOrgID   = "model-org-id"
	trainFlagModelName    = "model-name"
	trainFlagModelVersion = "model-version"
	trainFlagModelType    = "model-type"
	trainFlagModelLabels  = "model-labels"

	trainingStatusPrefix = "TRAINING_STATUS_"
)

// MLSubmitCustomTrainingJob is the corresponding action for 'train submit-custom'.
func MLSubmitCustomTrainingJob(c *cli.Context) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	err = client.uploadTrainingScript(true, c.String(trainFlagModelType), c.String(mlTrainingFlagFramework),
		c.String(trainFlagModelOrgID), c.String(mlTrainingFlagName), c.String(mlTrainingFlagVersion),
		c.Path(mlTrainingFlagPath))
	if err != nil {
		return err
	}
	registryItemID := fmt.Sprintf("%s:%s", c.String(trainFlagModelOrgID), c.String(mlTrainingFlagName))
	printf(c.App.Writer, "successfully uploaded training script to %s", registryItemID)
	trainingJobID, err := client.mlSubmitCustomTrainingJob(
		c.String(datasetFlagDatasetID), registryItemID, c.String(trainFlagModelOrgID),
		c.String(trainFlagModelName), c.String(trainFlagModelVersion))
	if err != nil {
		return err
	}
	printf(c.App.Writer, "Submitted training job with ID %s", trainingJobID)
	return nil
}

// MLSubmitTrainingJob is the corresponding action for 'train submit'.
func MLSubmitTrainingJob(c *cli.Context) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	trainingJobID, err := client.mlSubmitTrainingJob(
		c.String(datasetFlagDatasetID), c.String(trainFlagModelOrgID),
		c.String(trainFlagModelName), c.String(trainFlagModelVersion),
		c.String(trainFlagModelType), c.StringSlice(trainFlagModelLabels))
	if err != nil {
		return err
	}
	printf(c.App.Writer, "Submitted training job with ID %s", trainingJobID)
	return nil
}

// mlSubmitTrainingJob trains on data with the specified filter.
func (c *viamClient) mlSubmitTrainingJob(datasetID, orgID, modelName, modelVersion, modelType string,
	labels []string,
) (string, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return "", err
	}
	if modelVersion == "" {
		modelVersion = time.Now().Format("2006-01-02T15-04-05")
	}
	modelTypeEnum, ok := mltrainingpb.ModelType_value["MODEL_TYPE_"+strings.ToUpper(modelType)]
	if !ok || modelTypeEnum == int32(mltrainingpb.ModelType_MODEL_TYPE_UNSPECIFIED) {
		return "", errors.Errorf("%s must be a valid ModelType, got %s. See `viam train submit --help` for supported options",
			trainFlagModelType, modelType)
	}

	resp, err := c.mlTrainingClient.SubmitTrainingJob(context.Background(),
		&mltrainingpb.SubmitTrainingJobRequest{
			DatasetId:      datasetID,
			OrganizationId: orgID, ModelName: modelName, ModelVersion: modelVersion,
			ModelType: mltrainingpb.ModelType(modelTypeEnum), Tags: labels,
		})
	if err != nil {
		return "", errors.Wrapf(err, "received error from server")
	}
	return resp.Id, nil
}

// mlSubmitCustomTrainingJob trains on data with the specified dataset and registry item.
func (c *viamClient) mlSubmitCustomTrainingJob(datasetID, registryItemID, orgID, modelName,
	modelVersion string,
) (string, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return "", err
	}
	if modelVersion == "" {
		modelVersion = time.Now().Format("2006-01-02T15-04-05")
	}

	resp, err := c.mlTrainingClient.SubmitCustomTrainingJob(context.Background(),
		&mltrainingpb.SubmitCustomTrainingJobRequest{
			DatasetId:      datasetID,
			RegistryItemId: registryItemID,
			OrganizationId: orgID,
			ModelName:      modelName,
			ModelVersion:   modelVersion,
		})
	if err != nil {
		return "", errors.Wrapf(err, "received error from server")
	}
	return resp.Id, nil
}

// DataGetTrainingJob is the corresponding action for 'data train get'.
func DataGetTrainingJob(c *cli.Context) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	job, err := client.dataGetTrainingJob(c.String(trainFlagJobID))
	if err != nil {
		return err
	}
	printf(c.App.Writer, "Training job: %s", job)
	return nil
}

// dataGetTrainingJob gets a training job with the given ID.
//    // job, err := client.dataGetTrainingJob(job_id)
func (c *viamClient) dataGetTrainingJob(trainingJobID string) (*mltrainingpb.TrainingJobMetadata, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	resp, err := c.mlTrainingClient.GetTrainingJob(context.Background(), &mltrainingpb.GetTrainingJobRequest{Id: trainingJobID})
	if err != nil {
		return nil, err
	}
	return resp.Metadata, nil
}

// DataCancelTrainingJob is the corresponding action for 'data train cancel'.
func DataCancelTrainingJob(c *cli.Context) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	id := c.String(trainFlagJobID)
	if err := client.dataCancelTrainingJob(id); err != nil {
		return err
	}
	printf(c.App.Writer, "Successfully sent cancellation request for training job %s", id)
	return nil
}

// dataCancelTrainingJob cancels a training job with the given ID.
//    // job, err := client.dataCancelTrainingJob(job_id)
func (c *viamClient) dataCancelTrainingJob(trainingJobID string) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
	if _, err := c.mlTrainingClient.CancelTrainingJob(
		context.Background(), &mltrainingpb.CancelTrainingJobRequest{Id: trainingJobID}); err != nil {
		return err
	}
	return nil
}

// DataListTrainingJobs is the corresponding action for 'data train list'.
func DataListTrainingJobs(c *cli.Context) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	jobs, err := client.dataListTrainingJobs(c.String(generalFlagOrgID), c.String(trainFlagJobStatus))
	if err != nil {
		return err
	}
	for _, job := range jobs {
		printf(c.App.Writer, "Training job: %s\n", job)
	}
	return nil
}

// dataListTrainingJobs lists training jobs for the given org.
//    // job, err := client.dataListTrainingJobs(orgID, "")
func (c *viamClient) dataListTrainingJobs(orgID, status string) ([]*mltrainingpb.TrainingJobMetadata, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}

	if status == "" {
		status = "unspecified"
	}
	statusEnum, ok := mltrainingpb.TrainingStatus_value[trainingStatusPrefix+strings.ToUpper(status)]
	if !ok {
		return nil, errors.Errorf("%s must be a valid TrainingStatus, got %s. See `viam train list --help` for supported options",
			trainFlagJobStatus, status)
	}

	resp, err := c.mlTrainingClient.ListTrainingJobs(context.Background(), &mltrainingpb.ListTrainingJobsRequest{
		OrganizationId: orgID,
		Status:         mltrainingpb.TrainingStatus(statusEnum),
	})
	if err != nil {
		return nil, err
	}
	return resp.Jobs, nil
}

// allTrainingStatusValues returns the accepted values for the trainFlagJobStatus flag.
//    // statuses := client.allTrainingStatusValues()
func allTrainingStatusValues() string {
	var formattedStatuses []string
	for status := range mltrainingpb.TrainingStatus_value {
		formattedStatus := strings.ToLower(strings.TrimPrefix(status, trainingStatusPrefix))
		formattedStatuses = append(formattedStatuses, formattedStatus)
	}

	slices.Sort(formattedStatuses)
	return "[" + strings.Join(formattedStatuses, ", ") + "]"
}
