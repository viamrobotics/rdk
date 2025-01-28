package cli

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	v1 "go.viam.com/api/app/data/v1"
	mlinferencepb "go.viam.com/api/app/mlinference/v1"
)

const (
	inferenceFlagFileOrgID      = "file-org-id"
	inferenceFlagFileID         = "file-id"
	inferenceFlagFileLocationID = "file-location-id"
	inferenceFlagModelID        = "model-id"
	inferenceFlagModelVersionID = "model-version"
)

type mlInferenceInferArgs struct {
	OrgID          string
	FileOrgID      string
	FileID         string
	FileLocationID string
	ModelID        string
	ModelVersion   string
}

// MLInferenceInferAction is the corresponding action for 'inference infer'.
func MLInferenceInferAction(c *cli.Context, args mlInferenceInferArgs) error {
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
	fmt.Println("ModelVersion: ", args.ModelVersion)

	_, err = client.mlRunInference(
		args.OrgID, args.FileOrgID, args.FileID, args.FileLocationID,
		args.ModelID, args.ModelVersion)
	if err != nil {
		return err
	}
	return nil
}

// mlRunInference runs inference on an image with the specified parameters.
func (c *viamClient) mlRunInference(orgID, fileOrgID, fileID, fileLocation, modelID, modelVersion string) (*mlinferencepb.GetInferenceResponse, error) {
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
