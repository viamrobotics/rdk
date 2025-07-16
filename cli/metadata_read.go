package cli

import (
	"bytes"
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	apppb "go.viam.com/api/app/v1"
	structpb "google.golang.org/protobuf/types/known/structpb"
)

type metadataReadArgs struct {
	OrganizationID string
	LocationID     string
	MachineID      string
	MachinePartID  string
}

// MetadataReadAction is the action for the CLI command "viam metadata read".
func MetadataReadAction(ctx *cli.Context, args metadataReadArgs) error {
	if args.OrganizationID == "" && args.LocationID == "" && args.MachineID == "" && args.MachinePartID == "" {
		return errors.New("You must specify at least one of --organization-id, --location-id, --machine-id, --machine-part-id")
	}

	viamClient, err := newViamClient(ctx)
	if err != nil {
		printf(ctx.App.ErrWriter, "error initializing the Viam client: "+err.Error())
		return err
	}

	// Organization
	if args.OrganizationID != "" {
		err = displayOrganizationMetadata(ctx, viamClient.client, args.OrganizationID)
		if err != nil {
			return err
		}
	}

	// Location
	if args.LocationID != "" {
		err = displayLocationMetadata(ctx, viamClient.client, args.LocationID)
		if err != nil {
			return err
		}
	}

	// Machine
	if args.MachineID != "" {
		err = displayMachineMetadata(ctx, viamClient.client, args.MachineID)
		if err != nil {
			return err
		}
	}

	// Machine Part
	if args.MachinePartID != "" {
		err = displayMachinePartMetadata(ctx, viamClient.client, args.MachinePartID)
		if err != nil {
			return err
		}
	}

	return nil
}

func displayOrganizationMetadata(ctx *cli.Context, viamClient apppb.AppServiceClient, organizationID string) error {
	resp, err := viamClient.GetOrganizationMetadata(ctx.Context, &apppb.GetOrganizationMetadataRequest{
		OrganizationId: organizationID,
	})
	if err != nil {
		return errors.Wrap(err, "error fetching organization metadata")
	}

	return displayMetadata(ctx, "organization", organizationID, resp.GetData())
}

func displayLocationMetadata(ctx *cli.Context, viamClient apppb.AppServiceClient, locationID string) error {
	resp, err := viamClient.GetLocationMetadata(ctx.Context, &apppb.GetLocationMetadataRequest{
		LocationId: locationID,
	})
	if err != nil {
		return errors.Wrap(err, "error fetching location metadata")
	}

	return displayMetadata(ctx, "location", locationID, resp.GetData())
}

func displayMachineMetadata(ctx *cli.Context, viamClient apppb.AppServiceClient, machineID string) error {
	resp, err := viamClient.GetRobotMetadata(ctx.Context, &apppb.GetRobotMetadataRequest{
		Id: machineID,
	})
	if err != nil {
		return errors.Wrap(err, "error fetching machine metadata")
	}

	return displayMetadata(ctx, "machine", machineID, resp.GetData())
}

func displayMachinePartMetadata(ctx *cli.Context, viamClient apppb.AppServiceClient, machinePartID string) error {
	resp, err := viamClient.GetRobotPartMetadata(ctx.Context, &apppb.GetRobotPartMetadataRequest{
		Id: machinePartID,
	})
	if err != nil {
		return errors.Wrap(err, "error fetching machine part metadata")
	}

	return displayMetadata(ctx, "machine part", machinePartID, resp.GetData())
}

func displayMetadata(ctx *cli.Context, metadataType, metadataTypeID string, metadata *structpb.Struct) error {
	jsonData, err := metadata.MarshalJSON()
	if err != nil {
		return errors.Wrap(err, "error formatting metadata into JSON")
	}

	var prettyJSON bytes.Buffer
	err = json.Indent(&prettyJSON, jsonData, "", "\t")
	if err != nil {
		return errors.Wrap(err, "error formatting metadata into JSON")
	}

	printf(ctx.App.Writer, "\nMetadata for %s %s:\n", metadataType, metadataTypeID)
	printf(ctx.App.Writer, prettyJSON.String())

	return nil
}
