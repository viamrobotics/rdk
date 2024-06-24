package cli

import (
	"encoding/json"

	"github.com/urfave/cli/v2"
	apppb "go.viam.com/api/app/v1"
)

// RegisterAuthApplicationAction is the corresponding action for 'auth-app register'.
func RegisterAuthApplicationAction(c *cli.Context) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	return client.registerAuthApplicationAction(c)
}

func (c *viamClient) registerAuthApplicationAction(cCtx *cli.Context) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	orgID := cCtx.String(generalFlagOrgID)
	applicationName := cCtx.String(authApplicationFlagName)
	originURIs := cCtx.StringSlice(authApplicationFlagOriginURIs)
	redirectURIs := cCtx.StringSlice(authApplicationFlagRedirectURIs)
	logoutURI := cCtx.String(authApplicationFlagLogoutURI)

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

// UpdateAuthApplicationAction is the corresponding action for 'auth-app update'.
func UpdateAuthApplicationAction(c *cli.Context) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	return client.updateAuthApplicationAction(c)
}

func (c *viamClient) updateAuthApplicationAction(cCtx *cli.Context) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	orgID := cCtx.String(generalFlagOrgID)
	applicationID := cCtx.String(authApplicationFlagApplicationID)
	applicationName := cCtx.String(authApplicationFlagName)
	originURIs := cCtx.StringSlice(authApplicationFlagOriginURIs)
	redirectURIs := cCtx.StringSlice(authApplicationFlagRedirectURIs)
	logoutURI := cCtx.String(authApplicationFlagLogoutURI)

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

// GetAuthApplicationAction is the corresponding action for 'auth-app get'.
func GetAuthApplicationAction(c *cli.Context) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	return client.getAuthApplicationAction(c)
}

func (c *viamClient) getAuthApplicationAction(cCtx *cli.Context) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	orgID := cCtx.String(generalFlagOrgID)
	applicationID := cCtx.String(authApplicationFlagApplicationID)

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
