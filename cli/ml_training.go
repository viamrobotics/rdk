package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
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
	trainFlagJobID          = "job-id"
	trainFlagJobStatus      = "job-status"
	trainFlagModelOrgID     = "model-org-id"
	trainFlagModelVersion   = "model-version"
	trainFlagModelType      = "model-type"
	trainFlagModelFramework = "model-framework"
	trainFlagModelLabels    = "model-labels"

	trainingStatusPrefix = "TRAINING_STATUS_"

	// Flags for test-local command
	trainFlagContainerVersion        = "container-version"
	trainFlagDatasetFile             = "dataset-file"
	trainFlagDatasetRoot             = "dataset-root"
	trainFlagModelOutputDirectory    = "model-output-directory"
	trainFlagCustomArgs              = "custom-args"
	trainFlagTrainingScriptDirectory = "training-script-directory"
)

var validArgumentKeyRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type mlSubmitCustomTrainingJobArgs struct {
	DatasetID        string
	OrgID            string
	ModelName        string
	ModelVersion     string
	ScriptName       string
	Version          string
	ContainerVersion string
	Args             []string
}

// MLSubmitCustomTrainingJob is the corresponding action for 'train submit-custom'.
func MLSubmitCustomTrainingJob(c *cli.Context, args mlSubmitCustomTrainingJobArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	trainingJobID, err := client.mlSubmitCustomTrainingJob(
		args.DatasetID, args.ScriptName, args.Version, args.OrgID,
		args.ModelName, args.ModelVersion, args.ContainerVersion, args.Args)
	if err != nil {
		return err
	}
	printf(c.App.Writer, "Submitted training job with ID %s", trainingJobID)
	return nil
}

type mlSubmitCustomTrainingJobWithUploadArgs struct {
	URL              string
	DatasetID        string
	ModelName        string
	ModelVersion     string
	Path             string
	OrgID            string
	ModelOrgID       string
	ScriptName       string
	Version          string
	Framework        string
	ModelType        string
	ContainerVersion string
	Args             []string
}

// MLSubmitCustomTrainingJobWithUpload is the corresponding action for 'train submit-custom'.
func MLSubmitCustomTrainingJobWithUpload(c *cli.Context, args mlSubmitCustomTrainingJobWithUploadArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	if args.ModelName == args.ScriptName {
		return errors.New("model name and script name must be different")
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
		args.ModelName, args.ModelVersion, args.ContainerVersion, args.Args)
	if err != nil {
		return err
	}
	printf(c.App.Writer, "Submitted training job with ID %s", trainingJobID)
	return nil
}

type mlSubmitTrainingJobArgs struct {
	DatasetID      string
	ModelOrgID     string
	ModelName      string
	ModelType      string
	ModelFramework string
	ModelLabels    []string
	ModelVersion   string
}

type mlListContainersArgs struct {
	IncludeURIs bool
}

type prettyPrintContainer struct {
	Name        string
	EndOfLife   string
	Description string
	Framework   string
	URI         string `json:",omitempty"`
}

// MLListContainers is the corresponding action for 'train containers'.
func MLListContainers(c *cli.Context, args mlListContainersArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	supportedContainers, err := client.mlTrainingClient.ListSupportedContainers(
		context.Background(), &mltrainingpb.ListSupportedContainersRequest{})
	if err != nil {
		return err
	}

	var returnContainers []prettyPrintContainer
	for _, v := range supportedContainers.ContainerMap {
		container := prettyPrintContainer{
			Name:        v.Key,
			Description: v.Description,
			Framework:   v.Framework,
			EndOfLife:   v.Eol.AsTime().Format(time.RFC1123),
		}
		if args.IncludeURIs {
			container.URI = v.Uri
		}
		returnContainers = append(returnContainers, container)
	}
	b, err := json.MarshalIndent(returnContainers, "", "  ")
	if err != nil {
		return err
	}
	printf(c.App.Writer, "%s", b)
	return nil
}

// MLSubmitTrainingJob is the corresponding action for 'train submit'.
func MLSubmitTrainingJob(c *cli.Context, args mlSubmitTrainingJobArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	trainingJobID, err := client.mlSubmitTrainingJob(
		args.DatasetID, args.ModelOrgID, args.ModelName, args.ModelVersion, args.ModelType,
		args.ModelFramework, args.ModelLabels)
	if err != nil {
		return err
	}
	printf(c.App.Writer, "Submitted training job with ID %s", trainingJobID)
	return nil
}

// mlSubmitTrainingJob trains on data with the specified filter.
func (c *viamClient) mlSubmitTrainingJob(datasetID, orgID, modelName, modelVersion, modelType, modelFramework string,
	labels []string,
) (string, error) {
	if modelVersion == "" {
		modelVersion = time.Now().Format("2006-01-02T15-04-05")
	}
	modelTypeEnum, ok := mltrainingpb.ModelType_value["MODEL_TYPE_"+strings.ToUpper(modelType)]
	if !ok || modelTypeEnum == int32(mltrainingpb.ModelType_MODEL_TYPE_UNSPECIFIED) {
		return "", errors.Errorf("%s must be a valid ModelType, got %s. See `viam train submit --help` for supported options",
			trainFlagModelType, modelType)
	}
	modelFrameworkEnum, ok := mltrainingpb.ModelFramework_value["MODEL_FRAMEWORK_"+strings.ToUpper(modelFramework)]
	if !ok || modelFrameworkEnum == int32(mltrainingpb.ModelFramework_MODEL_FRAMEWORK_UNSPECIFIED) {
		return "", errors.Errorf("%s must be a valid ModelFramework, got %s. See `viam train submit --help` for supported options",
			trainFlagModelFramework, modelFramework)
	}

	resp, err := c.mlTrainingClient.SubmitTrainingJob(context.Background(),
		&mltrainingpb.SubmitTrainingJobRequest{
			DatasetId:      datasetID,
			OrganizationId: orgID, ModelName: modelName, ModelVersion: modelVersion,
			ModelType: mltrainingpb.ModelType(modelTypeEnum), ModelFramework: mltrainingpb.ModelFramework(modelFrameworkEnum),
			Tags: labels,
		})
	if err != nil {
		return "", errors.Wrapf(err, "received error from server")
	}
	return resp.Id, nil
}

// mlSubmitCustomTrainingJob trains on data with the specified dataset and registry item.
func (c *viamClient) mlSubmitCustomTrainingJob(datasetID, registryItemID, registryItemVersion, orgID, modelName,
	modelVersion, containerVersion string, args []string,
) (string, error) {
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
		ContainerVersion:    containerVersion,
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

type mlTrainingScriptTestLocalArgs struct {
	ContainerVersion        string
	DatasetFile             string
	DatasetRoot             string
	ModelOutputDirectory    string
	TrainingScriptDirectory string
	CustomArgs              []string
}

// MLTrainingScriptTestLocalAction runs training locally in a Docker container.
func MLTrainingScriptTestLocalAction(c *cli.Context, args mlTrainingScriptTestLocalArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	// Check if Docker is available
	if err := checkDockerAvailable(); err != nil {
		return err
	}

	scriptDirAbs, datasetRootAbs, outputDirAbs, err := getAbsolutePaths(args)
	if err != nil {
		return err
	}

	// Ensure the dataset file path is relative and doesn't escape the dataset root
	datasetFileRelative := filepath.Clean(args.DatasetFile)
	if !filepath.IsLocal(datasetFileRelative) {
		return errors.Errorf("dataset file path must be relative to dataset root and cannot escape it: %s", args.DatasetFile)
	}

	// Validate required paths exist
	if err := validatePaths(scriptDirAbs, datasetRootAbs,
		filepath.Join(datasetRootAbs, datasetFileRelative), outputDirAbs); err != nil {
		return err
	}

	// ensure the dataset file path is in Linux format
	datasetFileRelative = filepath.ToSlash(datasetFileRelative)
	// Create temporary training script
	tmpScript, err := createTrainingScript(args.CustomArgs, datasetFileRelative)
	if err != nil {
		return err
	}

	//nolint:errcheck
	defer os.Remove(tmpScript)

	// Get container image name
	containerImageURI, err := getContainerImageURI(client, args.ContainerVersion)
	if err != nil {
		return err
	}

	// Build docker run command
	dockerArgs := buildDockerRunArgs(scriptDirAbs, datasetRootAbs, outputDirAbs, tmpScript, containerImageURI)
	// Setup context with signal handling for Ctrl+C
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	//nolint:gosec
	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)
	cmd.Stdout = c.App.Writer
	cmd.Stderr = c.App.ErrWriter

	warningf(c.App.ErrWriter, "If this is your first time running training, "+
		"it may take a few minutes to download the container image. "+
		"This is normal and will not affect the training process.")

	if err := cmd.Run(); err != nil {
		// Check if the command was interrupted
		if ctx.Err() == context.Canceled {
			printf(c.App.Writer, "\nTraining interrupted by user")
			return errors.New("training interrupted")
		}

		// Provide additional context for platform-related errors
		errMsg := err.Error()
		if strings.Contains(errMsg, "platform") || strings.Contains(errMsg, "architecture") {
			return errors.Wrap(err, "failed to run training in Docker container. "+
				"Note: Training containers only support linux/x86_64 (amd64). "+
				"On ARM systems, ensure Docker Desktop is configured to enable Rosetta 2 emulation for x86_64 containers")
		}

		return errors.Wrap(err, "failed to run training in Docker container")
	}

	return nil
}

// createTrainingScript creates a temporary shell script file for running training inside the container.
func createTrainingScript(customArgs []string, datasetFileRelative string) (string, error) {
	scriptContent, err := buildTrainingScript(customArgs, datasetFileRelative)
	if err != nil {
		return "", err
	}

	tmpScript, err := os.CreateTemp("", "viam-training-*.sh")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temporary script file")
	}

	//nolint:errcheck
	defer tmpScript.Close()

	if _, err := tmpScript.WriteString(scriptContent); err != nil {
		return "", errors.Wrap(err, "failed to write to temporary script file")
	}

	//nolint:gosec
	if err := os.Chmod(tmpScript.Name(), 0o700); err != nil {
		return "", errors.Wrap(err, "failed to make script executable")
	}

	return tmpScript.Name(), nil
}

// buildDockerRunArgs constructs the arguments for the docker run command.
// NOTE: Google Vertex AI training containers are only available for linux/x86_64 (amd64).
// On ARM systems (e.g., Apple Silicon Macs), Docker will use Rosetta 2 emulation which
// may be slower but ensures compatibility with the same containers used in cloud training.
func buildDockerRunArgs(scriptDir, datasetRoot, outputDir, tmpScript, containerImageURI string) []string {
	return []string{
		"run",
		"-i", // Interactive mode to ensure signals are properly handled
		"--entrypoint", "/bin/bash",
		"--platform", "linux/x86_64",
		"--rm",
		"-v", fmt.Sprintf("%s:/training_script", scriptDir),
		"-v", fmt.Sprintf("%s:/dataset_root", datasetRoot),
		"-v", fmt.Sprintf("%s:/model_output", outputDir),
		"-v", fmt.Sprintf("%s:/run_training.sh", tmpScript),
		"-w", "/dataset_root", // Set working directory to dataset root so relative paths resolve correctly
		containerImageURI,
		"/run_training.sh",
	}
}

// checkDockerAvailable checks if Docker is installed and running.
func checkDockerAvailable() error {
	// Create a context with timeout for Docker commands
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "--version")
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return errors.New("Docker command timed out. Please check if Docker is responding")
		}
		return errors.New("Docker is not available. Please install Docker and ensure it is running. " +
			"Visit https://docs.docker.com/get-docker/ for installation instructions")
	}

	// Check if Docker daemon is running
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd = exec.CommandContext(ctx, "docker", "ps")
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return errors.New("Docker daemon is not responding. It may be starting up - please wait and try again")
		}
		return errors.New("Docker daemon is not running. Please start Docker and try again")
	}

	return nil
}

// validatePaths validates that required paths exist.
func validatePaths(scriptDir, datasetRoot, datasetFile, outputDir string) error {
	// Check training script directory exists
	if _, err := os.Stat(scriptDir); os.IsNotExist(err) {
		return errors.Errorf("training script directory does not exist: %s", scriptDir)
	}

	// Check for required files in training script directory
	setupPyPath := filepath.Join(scriptDir, "setup.py")
	if _, err := os.Stat(setupPyPath); os.IsNotExist(err) {
		return errors.Errorf("setup.py not found in training script directory: %s", scriptDir)
	}

	trainingPyPath := filepath.Join(scriptDir, "model", "training.py")
	if _, err := os.Stat(trainingPyPath); os.IsNotExist(err) {
		return errors.Errorf("model/training.py not found in training script directory: %s", scriptDir)
	}

	// Check dataset root directory exists
	if _, err := os.Stat(datasetRoot); os.IsNotExist(err) {
		return errors.Errorf("dataset root directory does not exist: %s", datasetRoot)
	}

	// Check dataset file exists
	if _, err := os.Stat(datasetFile); os.IsNotExist(err) {
		return errors.Errorf("dataset file does not exist: %s", datasetFile)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return errors.Wrapf(err, "failed to create model output directory")
	}

	return nil
}

func getAbsolutePaths(args mlTrainingScriptTestLocalArgs) (string, string, string, error) {
	scriptDirAbs, err := filepath.Abs(args.TrainingScriptDirectory)
	if err != nil {
		return "", "", "", errors.Wrapf(err, "failed to get absolute path for training script directory")
	}
	datasetRootAbs, err := filepath.Abs(args.DatasetRoot)
	if err != nil {
		return "", "", "", errors.Wrapf(err, "failed to get absolute path for dataset root directory")
	}
	outputDirAbs, err := filepath.Abs(args.ModelOutputDirectory)
	if err != nil {
		return "", "", "", errors.Wrapf(err, "failed to get absolute path for output directory")
	}
	return scriptDirAbs, datasetRootAbs, outputDirAbs, nil
}

// buildTrainingScript builds the shell script content to run inside the container.
// datasetFileRelative is the path to the dataset file relative to the dataset root (which is the CWD).
func buildTrainingScript(customArgs []string, datasetFileRelative string) (string, error) {
	// Validate custom arguments format (key=value) before building script
	for _, arg := range customArgs {
		if !strings.Contains(arg, "=") {
			return "", errors.Errorf("invalid custom argument format: %s (expected key=value)", arg)
		}

		// Validate that the key portion only contains safe characters
		parts := strings.SplitN(arg, "=", 2)
		key := parts[0]
		if !isValidArgumentKey(key) {
			return "", errors.Errorf("invalid argument key: %s (only alphanumeric characters, underscores, and hyphens are allowed)", key)
		}
	}

	var script strings.Builder

	// Script header and setup
	script.WriteString("#!/bin/bash\n")
	script.WriteString("set -e\n\n")
	script.WriteString("echo \"Installing training script package...\"\n")
	script.WriteString("pip3 install --no-cache-dir /training_script\n\n")
	script.WriteString("echo \"Running training...\"\n")

	// Build Python training command
	script.WriteString("python3 -m model.training")
	script.WriteString(" --dataset_file=")
	script.WriteString(strconv.Quote(datasetFileRelative))
	script.WriteString(" --model_output_directory=/model_output")

	// Add custom arguments
	for _, arg := range customArgs {
		parts := strings.SplitN(arg, "=", 2)
		key := parts[0]
		value := parts[1]

		script.WriteString(" --")
		script.WriteString(key)
		script.WriteString("=")
		// Use strconv.Quote for proper shell quoting - it returns a double-quoted string
		// with all special characters properly escaped
		script.WriteString(strconv.Quote(value))
	}
	script.WriteString("\n\n")

	// Script footer
	script.WriteString("echo \"Training completed successfully!\"\n")

	return script.String(), nil
}

// isValidArgumentKey validates that an argument key only contains safe characters.
// Allowed characters: letters (a-z, A-Z), digits (0-9), underscores (_), and hyphens (-).
func isValidArgumentKey(key string) bool {
	return key != "" && validArgumentKeyRegex.MatchString(key)
}

// getContainerImageURI returns the full container image URI based on the version.
func getContainerImageURI(c *viamClient, version string) (string, error) {
	res, err := c.mlTrainingClient.ListSupportedContainers(context.Background(), &mltrainingpb.ListSupportedContainersRequest{})
	if err != nil {
		return "", errors.Wrapf(err, "failed to list supported containers")
	}

	containerKeyList := []string{}
	for key := range res.ContainerMap {
		containerKeyList = append(containerKeyList, key)
	}
	slices.Sort(containerKeyList)

	container, ok := res.ContainerMap[version]
	if !ok {
		warningf(c.c.App.ErrWriter, "Container version %s not found. Supported versions: %s. "+
			"Attempting to use provided value as container URI: %s", version, strings.Join(containerKeyList, ", "), version)
		return version, nil
	}
	return container.Uri, nil
}
