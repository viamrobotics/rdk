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
	OrgID             string
	BundleID          string
	FirebaseConfigPath string
}

// SetFirebaseConfigAction uploads a Firebase config JSON for a specific app bundle ID.
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
		BundleId:   args.BundleID,
		ConfigJson: string(configJSON),
	})
	if err != nil {
		return fmt.Errorf("failed to set firebase config: %w", err)
	}

	fmt.Fprintf(cCtx.App.Writer, "Successfully set Firebase config for org %q, bundle %q\n", args.OrgID, args.BundleID)
	return nil
}

type readFirebaseConfigArgs struct {
	OrgID string
}

// ReadFirebaseConfigAction reads Firebase config metadata for a bundle ID.
// For security, only the org, bundle ID, and created on date are displayed, not the actual config content.
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

	fmt.Fprintf(cCtx.App.Writer, "Firebase config found:\n")
	fmt.Fprintf(cCtx.App.Writer, "  Organization ID: %s\n", args.OrgID)
	fmt.Fprintf(cCtx.App.Writer, "  Bundle ID:       %s\n", resp.BundleId)
	return nil
}

type deleteFirebaseConfigArgs struct {
	OrgID    string
	BundleID string
}

// DeleteFirebaseConfigAction deletes a Firebase config JSON for a specific app bundle ID.
func DeleteFirebaseConfigAction(cCtx *cli.Context, args deleteFirebaseConfigArgs) error {
	client, err := newViamClient(cCtx)
	if err != nil {
		return err
	}

	if err := client.ensureLoggedIn(); err != nil {
		return err
	}

	_, err = client.client.DeleteFirebaseConfig(cCtx.Context, &apppb.DeleteFirebaseConfigRequest{
		OrgId:    args.OrgID,
		BundleId: args.BundleID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete firebase config: %w", err)
	}

	fmt.Fprintf(cCtx.App.Writer, "Successfully deleted Firebase config for org %q, bundle %q\n", args.OrgID, args.BundleID)
	return nil
}
