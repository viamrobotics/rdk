package cli

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
	apppb "go.viam.com/api/app/v1"
)

type defaultsSetOrgArgs struct {
	OrgID string
}

func getDefaultOrg(cmd *cli.Command) (string, error) {
	config, err := ConfigFromCache(cmd)
	if err != nil {
		return "", err
	}

	return config.DefaultOrg, nil
}

// returns the provided org argument if non-empty else the default org value if set, else empty string
func orgOrDefault(cmd *cli.Command, orgStr string) string {
	if orgStr != "" {
		return orgStr
	}
	org, err := getDefaultOrg(cmd)
	if err != nil {
		return ""
	}

	return org
}

func locationOrDefault(cmd *cli.Command, locStr string) string {
	if locStr != "" {
		return locStr
	}

	loc, err := getDefaultLocation(cmd)
	if err != nil {
		return ""
	}

	return loc
}

func getDefaultLocation(cmd *cli.Command) (string, error) {
	config, err := ConfigFromCache(cmd)
	if err != nil {
		return "", err
	}

	return config.DefaultLocation, nil
}

// verifies that a passed org exists and is accessible, then sets it as the default within the config
func (c *viamClient) setDefaultOrg(ctx context.Context, cmd *cli.Command, config *Config, orgStr string) (*Config, error) {
	// we're setting a new default org, so try to verify that it actually exists and there's
	// permission to access it
	if orgStr != "" {
		if orgs, err := c.listOrganizations(ctx); err != nil {
			warningf(cmd.Root().ErrWriter, "unable to verify existence of org %s: %v", orgStr, err)
		} else {
			orgFound := false
			for _, org := range orgs {
				if orgStr == org.Id {
					orgFound = true
					break
				}
			}
			if !orgFound {
				var profileWarning string
				if gArgs, err := getGlobalArgs(cmd); err == nil {
					currProfile, _ := whichProfile(gArgs)
					if currProfile != nil && *currProfile != "" {
						profileWarning = ". You are currently logged in with profile %s. Did you mean to add a default to top level config?"
					}
				}
				return nil, fmt.Errorf("no org found matching ID %s%s", orgStr, profileWarning)
			}
		}
	}

	config.DefaultOrg = orgStr
	return config, nil
}

func (c *viamClient) writeDefaultOrg(ctx context.Context, cmd *cli.Command, config *Config, orgStr string) error {
	config, err := c.setDefaultOrg(ctx, cmd, config, orgStr)
	if err != nil {
		return err
	}

	return storeConfigToCache(config)
}

func writeDefaultOrg(ctx context.Context, cmd *cli.Command, orgStr string) error {
	client, err := newViamClient(ctx, cmd)
	if err != nil {
		return err
	}

	config, err := ConfigFromCache(cmd)
	if err != nil {
		return err
	}

	return client.writeDefaultOrg(ctx, cmd, config, orgStr)
}

func (c *viamClient) setDefaultLocation(ctx context.Context, cmd *cli.Command, config *Config, locStr string) (*Config, error) {
	var err error
	// we're setting a new default location arg, so verify that the location exists and is
	// accessible given the current auth settings and default org argument.
	if locStr != "" {
		orgs := []*apppb.Organization{}

		if config.DefaultOrg == "" {
			warningf(cmd.Root().ErrWriter, "attempting to set a default location argument when no default org argument is set."+
				" This can work, but may result in unexpected behavior.")

			orgs, err = c.listOrganizations(ctx)
			if err != nil {
				warningf(cmd.Root().ErrWriter, "unable to list organizations to find location %s: %v", locStr, err)
			}
		} else {
			org, err := c.getOrg(ctx, config.DefaultOrg)
			if err != nil {
				warningf(cmd.Root().ErrWriter, "unable to lookup org with default org value %s", config.DefaultOrg)
			} else {
				orgs = append(orgs, org)
			}
		}

		locFound := false
		for _, org := range orgs {
			locs, err := c.listLocations(ctx, org.Id)
			if err != nil {
				warningf(cmd.Root().ErrWriter, "unable to list locations for org %s: %v", org.Id, err)
				continue
			}
			for _, loc := range locs {
				if locStr == loc.Id {
					locFound = true
					break
				}
			}
			if locFound {
				break
			}
		}

		if !locFound {
			var profileWarning string
			if gArgs, err := getGlobalArgs(cmd); err == nil {
				currProfile, _ := whichProfile(gArgs)
				if currProfile != nil && *currProfile != "" {
					profileWarning = ". You are currently logged in with profile %s. Did you mean to add a default to top level config?"
				}
			}
			var forOrgWarning string
			if config.DefaultOrg != "" {
				forOrgWarning = fmt.Sprintf(" in default org %s", config.DefaultOrg)
			}
			return nil, fmt.Errorf("no location found matching ID %s%s%s", locStr, forOrgWarning, profileWarning)
		}
	}

	config.DefaultLocation = locStr
	return config, nil
}

func (c *viamClient) writeDefaultLocation(ctx context.Context, cmd *cli.Command, config *Config, locationStr string) error {
	config, err := c.setDefaultLocation(ctx, cmd, config, locationStr)
	if err != nil {
		return err
	}

	return storeConfigToCache(config)
}

func writeDefaultLocation(ctx context.Context, cmd *cli.Command, locationStr string) error {
	client, err := newViamClient(ctx, cmd)
	if err != nil {
		return err
	}

	config, err := ConfigFromCache(cmd)
	if err != nil {
		return err
	}

	return client.writeDefaultLocation(ctx, cmd, config, locationStr)
}

func defaultsSetOrgAction(ctx context.Context, cmd *cli.Command, args defaultsSetOrgArgs) error {
	return writeDefaultOrg(ctx, cmd, args.OrgID)
}

func defaultsClearOrgAction(ctx context.Context, cmd *cli.Command, args emptyArgs) error {
	return writeDefaultOrg(ctx, cmd, "")
}

type defaultsSetLocationArgs struct {
	LocationID string
}

func defaultsSetLocationAction(ctx context.Context, cmd *cli.Command, args defaultsSetLocationArgs) error {
	return writeDefaultLocation(ctx, cmd, args.LocationID)
}

func defaultsClearLocationAction(ctx context.Context, cmd *cli.Command, args emptyArgs) error {
	return writeDefaultLocation(ctx, cmd, "")
}
