package cli

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	v1 "go.viam.com/api/app/data/v1"
	mlinferencepb "go.viam.com/api/app/mlinference/v1"
)

// InferenceInferArgs holds the arguments for the inference action.
type InferenceInferArgs struct {
	OrgID          string
	FileOrgID      string
	FileID         string
	FileLocationID string
	ModelID        string
	// TODO: remove ModelOrgID
	ModelOrgID   string
	ModelVersion string
}

// InferenceInferAction is the corresponding action for 'inference infer'.
func InferenceInferAction(c *cli.Context, args InferenceInferArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	// Print the arguments
	fmt.Println("OrgID: ", args.OrgID)
	fmt.Println("FileOrgID: ", args.FileOrgID)
	fmt.Println("FileID: ", args.FileID)
	fmt.Println("FileLocationID: ", args.FileLocationID)
	fmt.Println("ModelID: ", args.ModelID)
	fmt.Println("ModelOrgID: ", args.ModelOrgID)
	fmt.Println("ModelVersion: ", args.ModelVersion)

	inferenceJobID, err := client.runInference(
		args.OrgID, args.FileOrgID, args.FileID, args.FileLocationID,
		args.ModelID, args.ModelOrgID, args.ModelVersion)
	if err != nil {
		return err
	}
	printf(c.App.Writer, "Submitted inference job with ID %s", inferenceJobID)
	return nil
}

// runInference runs inference on an image with the specified parameters.
func (c *viamClient) runInference(orgID, fileOrgID, fileID, fileLocation, modelID, modelOrgID, modelVersion string) (*mlinferencepb.GetInferenceResponse, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}

	req := &mlinferencepb.GetInferenceRequest{
		OrganizationId: orgID,
		BinaryId: &v1.BinaryID{
			FileId:         fileID,
			OrganizationId: fileOrgID,
			LocationId:     fileLocation,
		},
		RegistryItemId:      modelID,
		RegistryItemVersion: modelVersion,
	}

	resp, err := c.mlInferenceClient.GetInference(context.Background(), req)
	if err != nil {
		return nil, errors.Wrapf(err, "received error from server")
	}
	fmt.Println("OutputTensors: ", resp.OutputTensors)
	fmt.Println("Annotations: ", resp.Annotations)
	return resp, nil
}
