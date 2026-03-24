package cli

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
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
func SetFirebaseConfigAction(cCtx *cli.Context, args setFirebaseConfigArgs) error {
	client, err := newViamClient(cCtx)
	if err != nil {
		return err
	}

	configJSON, err := os.ReadFile(args.FirebaseConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read firebase config file %q: %w", args.FirebaseConfigPath, err)
	}

	if err := client.ensureLoggedIn(); err != nil {
		return err
	}

	_, err = client.client.SetFirebaseConfig(cCtx.Context, &apppb.SetFirebaseConfigRequest{
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
// For security, only the organization and app (bundle) ID are displayed, not the config JSON.
func ReadFirebaseConfigAction(cCtx *cli.Context, args readFirebaseConfigArgs) error {
	client, err := newViamClient(cCtx)
	if err != nil {
		return err
	}

	if err := client.ensureLoggedIn(); err != nil {
		return err
	}

	resp, err := client.client.GetFirebaseConfig(cCtx.Context, &apppb.GetFirebaseConfigRequest{
		OrgId: args.OrgID,
	})
	if err != nil {
		return fmt.Errorf("failed to read firebase config: %w", err)
	}

	printf(cCtx.App.Writer, "Firebase config for organization %q:", args.OrgID)
	printf(cCtx.App.Writer, "App ID: %q", resp.GetAppId())
	return nil
}

type deleteFirebaseConfigArgs struct {
	OrgID string
	AppID string
}

// DeleteFirebaseConfigAction deletes a Firebase config JSON for a specific app ID.
func DeleteFirebaseConfigAction(cCtx *cli.Context, args deleteFirebaseConfigArgs) error {
	client, err := newViamClient(cCtx)
	if err != nil {
		return err
	}

	if err := client.ensureLoggedIn(); err != nil {
		return err
	}

	_, err = client.client.DeleteFirebaseConfig(cCtx.Context, &apppb.DeleteFirebaseConfigRequest{
		OrgId: args.OrgID,
		AppId: args.AppID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete firebase config: %w", err)
	}

	return nil
}
