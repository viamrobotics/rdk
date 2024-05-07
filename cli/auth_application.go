package cli

import (
	"github.com/urfave/cli/v2"
	apppb "go.viam.com/api/app/v1"
)

// RegisterAuthApplicationAction is the corresponding action for 'third-party-auth-app register'.
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

	infof(cCtx.App.Writer, "Successfully created auth application")
	printf(cCtx.App.Writer, "%v", resp)
	warningf(cCtx.App.Writer, "Keep this information somewhere safe as you wont be shown it again; "+
		"it contains the secret to your auth application")
	return nil
}

// UpdateAuthApplicationAction is the corresponding action for 'third-party-auth-app update'.
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
	printf(cCtx.App.Writer, "%v", resp)
	return nil
}
