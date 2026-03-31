package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v3"
	mlinferencepb "go.viam.com/api/app/mlinference/v1"
)

const (
	inferenceFlagBinaryDataID = "binary-data-id"
	inferenceFlagModelOrgID   = "model-org-id"
	inferenceFlagModelVersion = "model-version"
)

type mlInferenceInferArgs struct {
	OrgID        string
	BinaryDataID string
	ModelOrgID   string
	ModelName    string
	ModelVersion string
}

// MLInferenceInferAction is the corresponding action for 'inference infer'.
func MLInferenceInferAction(ctx context.Context, cmd *cli.Command, args mlInferenceInferArgs) error {
	if args.OrgID == "" {
		return errors.New("must provide an organization ID to run an ML inference")
	}
	client, err := newViamClient(ctx, cmd)
	if err != nil {
		return err
	}

	_, err = client.mlRunInference(
		ctx,
		args.OrgID, args.BinaryDataID,
		args.ModelOrgID, args.ModelName, args.ModelVersion)
	if err != nil {
		return err
	}
	return nil
}

// mlRunInference runs inference on an image with the specified parameters.
func (c *viamClient) mlRunInference(ctx context.Context, orgID, binaryDataID, modelOrgID,
	modelName, modelVersion string,
) (*mlinferencepb.GetInferenceResponse, error) {
	if err := c.ensureLoggedIn(ctx); err != nil {
		return nil, err
	}

	req := &mlinferencepb.GetInferenceRequest{
		OrganizationId:      orgID,
		BinaryDataId:        binaryDataID,
		RegistryItemId:      fmt.Sprintf("%s:%s", modelOrgID, modelName),
		RegistryItemVersion: modelVersion,
	}

	resp, err := c.mlInferenceClient.GetInference(context.Background(), req)
	if err != nil {
		return nil, errors.Wrapf(err, "received error from server")
	}
	c.printInferenceResponse(resp)
	return resp, nil
}

// printInferenceResponse prints a neat representation of the GetInferenceResponse.
func (c *viamClient) printInferenceResponse(resp *mlinferencepb.GetInferenceResponse) {
	printf(c.c.Root().Writer, "Inference Response:")
	printf(c.c.Root().Writer, "Output Tensors:")
	if resp.OutputTensors != nil {
		for name, tensor := range resp.OutputTensors.Tensors {
			printf(c.c.Root().Writer, "  Tensor Name: %s", name)
			printf(c.c.Root().Writer, "    Shape: %v", tensor.Shape)
			if tensor.Tensor != nil {
				var sb strings.Builder
				for i, value := range tensor.GetDoubleTensor().GetData() {
					if i > 0 {
						sb.WriteString(", ")
					}
					sb.WriteString(fmt.Sprintf("%.4f", value))
				}
				printf(c.c.Root().Writer, "    Values: [%s]", sb.String())
			} else {
				printf(c.c.Root().Writer, "    No values available.")
			}
		}
	} else {
		printf(c.c.Root().Writer, "  No output tensors.")
	}

	printf(c.c.Root().Writer, "Annotations:")
	printf(c.c.Root().Writer, "Bounding Box Format: [x_min, y_min, x_max, y_max]")
	if resp.Annotations != nil {
		for _, bbox := range resp.Annotations.Bboxes {
			printf(c.c.Root().Writer, "  Bounding Box ID: %s, Label: %s",
				bbox.Id, bbox.Label)
			printf(c.c.Root().Writer, "    Coordinates: [%f, %f, %f, %f]",
				bbox.XMinNormalized, bbox.YMinNormalized, bbox.XMaxNormalized, bbox.YMaxNormalized)
			if bbox.Confidence != nil {
				printf(c.c.Root().Writer, "    Confidence: %.4f", *bbox.Confidence)
			}
		}
		for _, classification := range resp.Annotations.Classifications {
			printf(c.c.Root().Writer, "  Classification Label: %s", classification.Label)
			if classification.Confidence != nil {
				printf(c.c.Root().Writer, "    Confidence: %.4f", *classification.Confidence)
			}
		}
	} else {
		printf(c.c.Root().Writer, "  No annotations.")
	}
}
