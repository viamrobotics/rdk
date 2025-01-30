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
	printInferenceResponse(resp)
	return resp, nil
}

// printInferenceResponse prints a neat representation of the GetInferenceResponse.
func printInferenceResponse(resp *mlinferencepb.GetInferenceResponse) {
	fmt.Println("Inference Response:")
	fmt.Println("Output Tensors:")
	if resp.OutputTensors != nil {
		for name, tensor := range resp.OutputTensors.Tensors {
			fmt.Printf("  Tensor Name: %s\n", name)
			fmt.Printf("    Shape: %v\n", tensor.Shape)
			if tensor.Tensor != nil {
				fmt.Print("    Values: [")
				for i, value := range tensor.GetDoubleTensor().GetData() {
					if i > 0 {
						fmt.Print(", ")
					}
					fmt.Printf("%.4f", value)
				}
				fmt.Println("]")
			} else {
				fmt.Println("    No values available.")
			}
		}
	} else {
		fmt.Println("  No output tensors.")
	}

	fmt.Println("Annotations:")
	if resp.Annotations != nil {
		for _, bbox := range resp.Annotations.Bboxes {
			fmt.Printf("  Bounding Box ID: %s, Label: %s\n", bbox.Id, bbox.Label)
			fmt.Printf("    Coordinates: [%f, %f, %f, %f]\n", bbox.XMinNormalized, bbox.YMinNormalized, bbox.XMaxNormalized, bbox.YMaxNormalized)
			if bbox.Confidence != nil {
				fmt.Printf("    Confidence: %.4f\n", *bbox.Confidence)
			}
		}
		for _, classification := range resp.Annotations.Classifications {
			fmt.Printf("  Classification Label: %s\n", classification.Label)
			if classification.Confidence != nil {
				fmt.Printf("    Confidence: %.4f\n", *classification.Confidence)
			}
		}
	} else {
		fmt.Println("  No annotations.")
	}
}
