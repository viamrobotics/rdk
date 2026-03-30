package cli

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v3"
	apppb "go.viam.com/api/app/v1"
	structpb "google.golang.org/protobuf/types/known/structpb"
)

type metadataReadArgs struct {
	OrgID      string
	LocationID string
	MachineID  string
	PartID     string
}

// MetadataReadAction is the action for the CLI command "viam metadata read".
func MetadataReadAction(ctx context.Context, cmd *cli.Command, args metadataReadArgs) error {
	if args.OrgID == "" && args.LocationID == "" && args.MachineID == "" && args.PartID == "" {
		return errors.New("You must specify at least one of --organization-id, --location-id, --machine-id, --machine-part-id")
	}

	viamClient, err := newViamClient(ctx, cmd)
	if err != nil {
		printf(cmd.Root().ErrWriter, "error initializing the Viam client: "+err.Error())
		return err
	}

	// Organization
	if args.OrgID != "" {
		err = displayOrganizationMetadata(ctx, cmd, viamClient.client, args.OrgID)
		if err != nil {
			return err
		}
	}

	// Location
	if args.LocationID != "" {
		err = displayLocationMetadata(ctx, cmd, viamClient.client, args.LocationID)
		if err != nil {
			return err
		}
	}

	// Machine
	if args.MachineID != "" {
		err = displayMachineMetadata(ctx, cmd, viamClient.client, args.MachineID)
		if err != nil {
			return err
		}
	}

	// Machine Part
	if args.PartID != "" {
		err = displayMachinePartMetadata(ctx, cmd, viamClient.client, args.PartID)
		if err != nil {
			return err
		}
	}

	return nil
}

func displayOrganizationMetadata(ctx context.Context, cmd *cli.Command, viamClient apppb.AppServiceClient, organizationID string) error {
	resp, err := viamClient.GetOrganizationMetadata(ctx, &apppb.GetOrganizationMetadataRequest{
		OrganizationId: organizationID,
	})
	if err != nil {
		return errors.Wrap(err, "error fetching organization metadata")
	}

	return displayMetadata(cmd, "organization", organizationID, resp.GetData())
}

func displayLocationMetadata(ctx context.Context, cmd *cli.Command, viamClient apppb.AppServiceClient, locationID string) error {
	resp, err := viamClient.GetLocationMetadata(ctx, &apppb.GetLocationMetadataRequest{
		LocationId: locationID,
	})
	if err != nil {
		return errors.Wrap(err, "error fetching location metadata")
	}

	return displayMetadata(cmd, "location", locationID, resp.GetData())
}

func displayMachineMetadata(ctx context.Context, cmd *cli.Command, viamClient apppb.AppServiceClient, machineID string) error {
	resp, err := viamClient.GetRobotMetadata(ctx, &apppb.GetRobotMetadataRequest{
		Id: machineID,
	})
	if err != nil {
		return errors.Wrap(err, "error fetching machine metadata")
	}

	return displayMetadata(cmd, "machine", machineID, resp.GetData())
}

func displayMachinePartMetadata(ctx context.Context, cmd *cli.Command, viamClient apppb.AppServiceClient, partID string) error {
	resp, err := viamClient.GetRobotPartMetadata(ctx, &apppb.GetRobotPartMetadataRequest{
		Id: partID,
	})
	if err != nil {
		return errors.Wrap(err, "error fetching machine part metadata")
	}

	return displayMetadata(cmd, "part", partID, resp.GetData())
}

func displayMetadata(cmd *cli.Command, metadataType, metadataTypeID string, metadata *structpb.Struct) error {
	jsonData, err := metadata.MarshalJSON()
	if err != nil {
		return errors.Wrap(err, "error formatting metadata into JSON")
	}

	var prettyJSON bytes.Buffer
	err = json.Indent(&prettyJSON, jsonData, "", "\t")
	if err != nil {
		return errors.Wrap(err, "error formatting metadata into JSON")
	}

	printf(cmd.Root().Writer, "\nMetadata for %s %s:\n", metadataType, metadataTypeID)
	printf(cmd.Root().Writer, prettyJSON.String())

	return nil
}
