package cli

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.uber.org/multierr"
	mltrainingpb "go.viam.com/api/app/mltraining/v1"
	packagespb "go.viam.com/api/app/packages/v1"
	v1 "go.viam.com/api/app/v1"
	"google.golang.org/protobuf/types/known/structpb"
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

type mlSubmitCustomTrainingJobArgs struct {
	DatasetID    string
	OrgID        string
	ModelName    string
	ModelVersion string
	ScriptName   string
	Version      string
	Args         []string
}

// MLSubmitCustomTrainingJob is the corresponding action for 'train submit-custom'.
func MLSubmitCustomTrainingJob(c *cli.Context, args mlSubmitCustomTrainingJobArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	trainingJobID, err := client.mlSubmitCustomTrainingJob(
		args.DatasetID, args.ScriptName, args.Version, args.OrgID,
		args.ModelName, args.ModelVersion, args.Args)
	if err != nil {
		return err
	}
	printf(c.App.Writer, "Submitted training job with ID %s", trainingJobID)
	return nil
}

type mlSubmitCustomTrainingJobWithUploadArgs struct {
	URL          string
	DatasetID    string
	ModelName    string
	ModelVersion string
	Path         string
	OrgID        string
	ModelOrgID   string
	ScriptName   string
	Version      string
	Framework    string
	ModelType    string
	Args         []string
}

// MLSubmitCustomTrainingJobWithUpload is the corresponding action for 'train submit-custom'.
func MLSubmitCustomTrainingJobWithUpload(c *cli.Context, args mlSubmitCustomTrainingJobWithUploadArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	resp, err := client.uploadTrainingScript(true, args.ModelType, args.Framework,
		args.URL, args.OrgID, args.ScriptName, args.Version, args.Path)
	if err != nil {
		return err
	}
	registryItemID := fmt.Sprintf("%s:%s", args.OrgID, args.ScriptName)

	moduleID := moduleID{
		prefix: args.OrgID,
		name:   args.ScriptName,
	}
	url := moduleID.ToDetailURL(client.baseURL.Hostname(), PackageTypeMLTraining)
	printf(c.App.Writer, "Version successfully uploaded! you can view your changes online here: %s. \n"+
		"To use your training script in the from-registry command, use %s as the script name", url,
		registryItemID)
	trainingJobID, err := client.mlSubmitCustomTrainingJob(
		args.DatasetID, registryItemID, resp.Version, args.ModelOrgID,
		args.ModelName, args.ModelVersion, args.Args)
	if err != nil {
		return err
	}
	printf(c.App.Writer, "Submitted training job with ID %s", trainingJobID)
	return nil
}

type mlSubmitTrainingJobArgs struct {
	DatasetID    string
	ModelOrgID   string
	ModelName    string
	ModelType    string
	ModelLabels  []string
	ModelVersion string
}

// MLSubmitTrainingJob is the corresponding action for 'train submit'.
func MLSubmitTrainingJob(c *cli.Context, args mlSubmitTrainingJobArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	trainingJobID, err := client.mlSubmitTrainingJob(
		args.DatasetID, args.ModelOrgID, args.ModelName, args.ModelVersion, args.ModelType,
		args.ModelLabels)
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
func (c *viamClient) mlSubmitCustomTrainingJob(datasetID, registryItemID, registryItemVersion, orgID, modelName,
	modelVersion string, args []string,
) (string, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return "", err
	}
	splitName := strings.Split(registryItemID, ":")
	if len(splitName) != 2 {
		return "", errors.Errorf("invalid training script name '%s'."+
			" Training script name must be in the form 'public-namespace:registry-name' for public training scripts"+
			" or 'org-id:registry-name' for private training scripts in organizations without a public namespace", registryItemID)
	}
	if modelVersion == "" {
		modelVersion = time.Now().Format("2006-01-02T15-04-05")
	}

	req := &mltrainingpb.SubmitCustomTrainingJobRequest{
		DatasetId:           datasetID,
		RegistryItemId:      registryItemID,
		RegistryItemVersion: registryItemVersion,
		OrganizationId:      orgID,
		ModelName:           modelName,
		ModelVersion:        modelVersion,
	}

	if len(args) > 0 {
		argMap := make(map[string]string)
		for _, optionVal := range args {
			splitOptionVal := strings.Split(optionVal, "=")
			if len(splitOptionVal) != 2 {
				return "", errors.Errorf("invalid format for command line arguments, passed: %s", args)
			}
			argMap[splitOptionVal[0]] = splitOptionVal[1]
		}
		req.Arguments = argMap
	}

	resp, err := c.mlTrainingClient.SubmitCustomTrainingJob(context.Background(), req)
	if err != nil {
		return "", errors.Wrapf(err, "received error from server")
	}
	return resp.Id, nil
}

type dataGetTrainingJobArgs struct {
	JobID string
}

// DataGetTrainingJob is the corresponding action for 'data train get'.
func DataGetTrainingJob(c *cli.Context, args dataGetTrainingJobArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	job, err := client.dataGetTrainingJob(args.JobID)
	if err != nil {
		return err
	}
	printf(c.App.Writer, "Training job: %s", job)
	return nil
}

// dataGetTrainingJob gets a training job with the given ID.
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

type mlGetTrainingJobLogsArgs struct {
	JobID string
}

// MLGetTrainingJobLogs is the corresponding action for 'data train logs'.
func MLGetTrainingJobLogs(c *cli.Context, args mlGetTrainingJobLogsArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	logs, err := client.mlGetTrainingJobLogs(args.JobID)
	if err != nil {
		return err
	}

	if len(logs) == 0 {
		printf(c.App.Writer, "No logs found for job %s", trainFlagJobID)
	}
	for _, log := range logs {
		printf(c.App.Writer, "{\"Timestamp\": \"%s\", \"Level\": \"%s\", \"Message\": \"%s\"}", log.Time.AsTime(), log.Level, log.Message)
	}
	return nil
}

// mlGetTrainingJobLogs gets the training job logs with the given ID.
func (c *viamClient) mlGetTrainingJobLogs(trainingJobID string) ([]*mltrainingpb.TrainingJobLogEntry, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	var allLogs []*mltrainingpb.TrainingJobLogEntry
	var page string

	// Loop to fetch and accumulate results
	for {
		resp, err := c.mlTrainingClient.GetTrainingJobLogs(context.Background(),
			&mltrainingpb.GetTrainingJobLogsRequest{Id: trainingJobID, PageToken: &page})
		if err != nil {
			return nil, err
		}
		allLogs = append(allLogs, resp.GetLogs()...)

		if resp.GetNextPageToken() == "" {
			break
		}
		page = resp.GetNextPageToken()
	}
	return allLogs, nil
}

type dataCancelTrainingJobArgs struct {
	JobID string
}

// DataCancelTrainingJob is the corresponding action for 'data train cancel'.
func DataCancelTrainingJob(c *cli.Context, args dataCancelTrainingJobArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	id := args.JobID
	if err := client.dataCancelTrainingJob(id); err != nil {
		return err
	}
	printf(c.App.Writer, "Successfully sent cancellation request for training job %s", id)
	return nil
}

// dataCancelTrainingJob cancels a training job with the given ID.
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

type dataListTrainingJobsArgs struct {
	OrgID     string
	JobStatus string
}

// DataListTrainingJobs is the corresponding action for 'data train list'.
func DataListTrainingJobs(c *cli.Context, args dataListTrainingJobsArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	jobs, err := client.dataListTrainingJobs(args.OrgID, args.JobStatus)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		printf(c.App.Writer, "Training job: %s\n", job)
	}
	return nil
}

// dataListTrainingJobs lists training jobs for the given org.
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
func allTrainingStatusValues() []string {
	var formattedStatuses []string
	for status := range mltrainingpb.TrainingStatus_value {
		formattedStatus := strings.ToLower(strings.TrimPrefix(status, trainingStatusPrefix))
		formattedStatuses = append(formattedStatuses, formattedStatus)
	}

	slices.Sort(formattedStatuses)
	return formattedStatuses
}

func defaultTrainingStatus() string {
	return strings.ToLower(strings.TrimPrefix(mltrainingpb.TrainingStatus_TRAINING_STATUS_UNSPECIFIED.String(), trainingStatusPrefix))
}

type mlTrainingUploadArgs struct {
	Path       string
	OrgID      string
	ScriptName string
	Version    string
	Framework  string
	Type       string
	Draft      bool
	URL        string
}

// MLTrainingUploadAction uploads a new custom training script.
func MLTrainingUploadAction(c *cli.Context, args mlTrainingUploadArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	_, err = client.uploadTrainingScript(args.Draft, args.Type,
		args.Framework, args.URL, args.OrgID, args.ScriptName,
		args.Version, args.Path,
	)
	if err != nil {
		return err
	}

	moduleID := moduleID{
		prefix: args.OrgID,
		name:   args.ScriptName,
	}
	url := moduleID.ToDetailURL(client.baseURL.Hostname(), PackageTypeMLTraining)
	printf(c.App.Writer, "Version successfully uploaded! you can view your changes online here: %s. \n"+
		"To use your training script in the from-registry command, use %s:%s as the script name", url,
		moduleID.prefix, moduleID.name)
	return nil
}

func (c *viamClient) uploadTrainingScript(draft bool, modelType, framework, url, orgID, name, version, path string) (
	*packagespb.CreatePackageResponse, error,
) {
	metadata, err := createMetadata(draft, modelType, framework, url)
	if err != nil {
		return nil, err
	}
	metadataStruct, err := convertMetadataToStruct(*metadata)
	if err != nil {
		return nil, err
	}

	resp, err := c.uploadPackage(orgID,
		name,
		version,
		string(PackageTypeMLTraining),
		path,
		metadataStruct,
	)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

type mlTrainingUpdateArgs struct {
	OrgID       string
	ScriptName  string
	Visibility  string
	Description string
	URL         string
}

// MLTrainingUpdateAction updates the visibility of training scripts.
func MLTrainingUpdateAction(c *cli.Context, args mlTrainingUpdateArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	err = client.updateTrainingScript(args.OrgID, args.ScriptName,
		args.Visibility, args.Description, args.URL,
	)
	if err != nil {
		return err
	}

	moduleID := moduleID{
		prefix: args.OrgID,
		name:   args.ScriptName,
	}
	url := moduleID.ToDetailURL(client.baseURL.Hostname(), PackageTypeMLTraining)
	printf(c.App.Writer, "Training script successfully updated! you can view your changes online here: %s", url)
	return nil
}

func (c *viamClient) updateTrainingScript(orgID, name, visibility, description, url string) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	// Get registry item
	itemID := fmt.Sprintf("%s:%s", orgID, name)
	resp, err := c.client.GetRegistryItem(c.c.Context, &v1.GetRegistryItemRequest{
		ItemId: itemID,
	})
	if err != nil {
		return err
	}
	visibilityProto, err := convertVisibilityToProto(visibility)
	if err != nil {
		return err
	}
	// Get and validate description and visibility
	updatedDescription := resp.GetItem().GetDescription()
	if description != "" {
		updatedDescription = description
	}
	if updatedDescription == "" && *visibilityProto == v1.Visibility_VISIBILITY_PUBLIC {
		return errors.New("no existing description for registry item, description must be provided")
	}
	var stringURL *string
	if url != "" {
		stringURL = &url
	} else {
		stringURL = nil
	}
	// Update registry item
	if _, err = c.client.UpdateRegistryItem(c.c.Context, &v1.UpdateRegistryItemRequest{
		ItemId:      itemID,
		Type:        resp.GetItem().GetType(),
		Description: updatedDescription,
		Visibility:  *visibilityProto,
		Url:         stringURL,
	}); err != nil {
		return err
	}
	return nil
}

// ModelType refers to the type of the model.
type ModelType string

// ModelType enumeration.
const (
	ModelTypeUnspecified               = ModelType("unspecified")
	ModelTypeSingleLabelClassification = ModelType("single_label_classification")
	ModelTypeMultiLabelClassification  = ModelType("multi_label_classification")
	ModelTypeObjectDetection           = ModelType("object_detection")
)

var modelTypes = []string{
	string(ModelTypeUnspecified), string(ModelTypeSingleLabelClassification),
	string(ModelTypeMultiLabelClassification), string(ModelTypeObjectDetection),
}

// ModelFramework refers to the backend framework of the model.
type ModelFramework string

// ModelFramework enumeration.
const (
	ModelFrameworkUnspecified = ModelFramework("unspecified")
	ModelFrameworkTFLite      = ModelFramework("tflite")
	ModelFrameworkTensorFlow  = ModelFramework("tensorflow")
	ModelFrameworkPyTorch     = ModelFramework("pytorch")
	ModelFrameworkONNX        = ModelFramework("onnx")
)

var modelFrameworks = []string{
	string(ModelFrameworkUnspecified), string(ModelFrameworkTFLite), string(ModelFrameworkTensorFlow),
	string(ModelFrameworkPyTorch), string(ModelFrameworkONNX),
}

// MLMetadata struct stores package info for ML training packages.
type MLMetadata struct {
	Draft     bool
	ModelType string
	Framework string
	URL       string
}

func createMetadata(draft bool, modelType, framework, url string) (*MLMetadata, error) {
	t, typeErr := findValueOrSetDefault(modelTypes, modelType, string(ModelTypeUnspecified))
	f, frameWorkErr := findValueOrSetDefault(modelFrameworks, framework, string(ModelFrameworkUnspecified))

	if typeErr != nil || frameWorkErr != nil {
		return nil, errors.Wrap(multierr.Combine(typeErr, frameWorkErr), "failed to set metadata")
	}

	return &MLMetadata{
		Draft:     draft,
		ModelType: t,
		Framework: f,
		URL:       url,
	}, nil
}

// findValueOrSetDefault either finds the matching value from all possible values,
// sets a default if the value is not present, or errors if the value is not permissible.
func findValueOrSetDefault(arr []string, val, defaultVal string) (string, error) {
	if val == "" {
		return defaultVal, nil
	}
	for _, str := range arr {
		if str == val {
			return val, nil
		}
	}
	return "", errors.New("value must be one of: " + strings.Join(arr, ", "))
}

var (
	modelTypeKey      = "model_type"
	modelFrameworkKey = "model_framework"
	draftKey          = "draft"
	urlKey            = "url"
)

func convertMetadataToStruct(metadata MLMetadata) (*structpb.Struct, error) {
	metadataMap := make(map[string]interface{})
	metadataMap[modelTypeKey] = metadata.ModelType
	metadataMap[modelFrameworkKey] = metadata.Framework
	metadataMap[draftKey] = metadata.Draft
	metadataMap[urlKey] = metadata.URL
	metadataStruct, err := structpb.NewStruct(metadataMap)
	if err != nil {
		return nil, err
	}
	return metadataStruct, nil
}

func convertVisibilityToProto(visibility string) (*v1.Visibility, error) {
	var visibilityProto v1.Visibility
	switch visibility {
	case "public":
		visibilityProto = v1.Visibility_VISIBILITY_PUBLIC
	case "private":
		visibilityProto = v1.Visibility_VISIBILITY_PRIVATE
	default:
		return nil, errors.New("invalid visibility provided, must be either public or private")
	}

	return &visibilityProto, nil
}
