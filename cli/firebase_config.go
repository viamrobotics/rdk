package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"
	apppb "go.viam.com/api/app/v1"
)

const (
	firebaseConfigFlagPath = "firebase-config-path"
)

type setFirebaseConfigArgs struct {
	OrgID              string
	AppID              string
	FirebaseConfigPath string
}

// SetFirebaseConfigAction uploads a Firebase config JSON for a specific app ID.
func SetFirebaseConfigAction(ctx context.Context, cmd *cli.Command, args setFirebaseConfigArgs) error {
	client, err := newViamClient(ctx, cmd)
	if err != nil {
		return err
	}

	configJSON, err := os.ReadFile(args.FirebaseConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read firebase config file %q: %w", args.FirebaseConfigPath, err)
	}

	_, err = client.client.SetFirebaseConfig(ctx, &apppb.SetFirebaseConfigRequest{
		OrgId:      args.OrgID,
		AppId:      args.AppID,
		ConfigJson: string(configJSON),
	})
	if err != nil {
		return fmt.Errorf("failed to set firebase config: %w", err)
	}

	return nil
}

type readFirebaseConfigArgs struct {
	OrgID string
}

// ReadFirebaseConfigAction reads Firebase config metadata for an organization.
// For security, only the organization and app ID are displayed, not the config JSON.
func ReadFirebaseConfigAction(ctx context.Context, cmd *cli.Command, args readFirebaseConfigArgs) error {
	client, err := newViamClient(ctx, cmd)
	if err != nil {
		return err
	}

	resp, err := client.client.GetFirebaseConfig(ctx, &apppb.GetFirebaseConfigRequest{
		OrgId: args.OrgID,
	})
	if err != nil {
		return fmt.Errorf("failed to read firebase config: %w", err)
	}

	printf(cmd.Root().Writer, "Firebase config for organization %q:", args.OrgID)
	printf(cmd.Root().Writer, "App ID: %q", resp.GetAppId())
	return nil
}

type deleteFirebaseConfigArgs struct {
	OrgID string
	AppID string
}

// DeleteFirebaseConfigAction deletes a Firebase config JSON for a specific app ID.
func DeleteFirebaseConfigAction(ctx context.Context, cmd *cli.Command, args deleteFirebaseConfigArgs) error {
	client, err := newViamClient(ctx, cmd)
	if err != nil {
		return err
	}

	_, err = client.client.DeleteFirebaseConfig(ctx, &apppb.DeleteFirebaseConfigRequest{
		OrgId: args.OrgID,
		AppId: args.AppID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete firebase config: %w", err)
	}

	return nil
}
