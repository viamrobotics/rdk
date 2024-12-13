package cli

import (
	"encoding/json"

	"github.com/urfave/cli/v2"
	apppb "go.viam.com/api/app/v1"
)

type registerAuthApplicationArgs struct {
	OrgID           string
	ApplicationName string
	OriginURIs      []string
	RedirectURIs    []string
	LogoutURI       string
}

// RegisterAuthApplicationAction is the corresponding action for 'auth-app register'.
func RegisterAuthApplicationAction(c *cli.Context, args registerAuthApplicationArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	return client.registerAuthApplicationAction(c, args)
}

func (c *viamClient) registerAuthApplicationAction(cCtx *cli.Context, args registerAuthApplicationArgs) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	orgID := args.OrgID
	applicationName := args.ApplicationName
	originURIs := args.OriginURIs
	redirectURIs := args.RedirectURIs
	logoutURI := args.LogoutURI

	req := &apppb.RegisterAuthApplicationRequest{
		OrgId:           orgID,
		ApplicationName: applicationName,
		OriginUris:      originURIs,
		RedirectUris:    redirectURIs,
		LogoutUri:       logoutURI,
	}
	resp, err := c.endUserClient.RegisterAuthApplication(c.c.Context, req)
	if err != nil {
		return err
	}

	infof(cCtx.App.Writer, "Successfully registered auth application")
	formatOutput, err := json.MarshalIndent(resp, "", "\t")
	if err != nil {
		return err
	}
	printf(cCtx.App.Writer, "%s", formatOutput)
	warningf(cCtx.App.Writer, "Keep this information somewhere safe; "+
		"it contains the secret to your auth application")
	return nil
}

type updateAuthApplicationArgs struct {
	OrgID           string
	ApplicationID   string
	ApplicationName string
	OriginURIs      []string
	RedirectURIs    []string
	LogoutURI       string
}

// UpdateAuthApplicationAction is the corresponding action for 'auth-app update'.
func UpdateAuthApplicationAction(c *cli.Context, args updateAuthApplicationArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	return client.updateAuthApplicationAction(c, args)
}

func (c *viamClient) updateAuthApplicationAction(cCtx *cli.Context, args updateAuthApplicationArgs) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	orgID := args.OrgID
	applicationID := args.ApplicationID
	applicationName := args.ApplicationName
	originURIs := args.OriginURIs
	redirectURIs := args.RedirectURIs
	logoutURI := args.LogoutURI

	req := &apppb.UpdateAuthApplicationRequest{
		OrgId:           orgID,
		ApplicationId:   applicationID,
		ApplicationName: applicationName,
		OriginUris:      originURIs,
		RedirectUris:    redirectURIs,
		LogoutUri:       logoutURI,
	}
	resp, err := c.endUserClient.UpdateAuthApplication(c.c.Context, req)
	if err != nil {
		return err
	}

	infof(cCtx.App.Writer, "Successfully updated auth application")
	formatOutput, err := json.MarshalIndent(resp, "", "\t")
	if err != nil {
		return err
	}
	printf(cCtx.App.Writer, "%s", formatOutput)
	return nil
}

type getAuthApplicationArgs struct {
	OrgID         string
	ApplicationID string
}

// GetAuthApplicationAction is the corresponding action for 'auth-app get'.
func GetAuthApplicationAction(c *cli.Context, args getAuthApplicationArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	return client.getAuthApplicationAction(c, args)
}

func (c *viamClient) getAuthApplicationAction(cCtx *cli.Context, args getAuthApplicationArgs) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	orgID := args.OrgID
	applicationID := args.ApplicationID

	req := &apppb.GetAuthApplicationRequest{
		OrgId:         orgID,
		ApplicationId: applicationID,
	}
	resp, err := c.endUserClient.GetAuthApplication(c.c.Context, req)
	if err != nil {
		return err
	}

	formatOutput, err := json.MarshalIndent(resp, "", "\t")
	if err != nil {
		return err
	}
	printf(cCtx.App.Writer, "%s", formatOutput)
	warningf(cCtx.App.Writer, "Keep this information somewhere safe; "+
		"it contains the secret to your auth application")
	return nil
}
