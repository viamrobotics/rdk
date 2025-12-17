package cli

import (
	"fmt"

	"github.com/urfave/cli/v2"
	apppb "go.viam.com/api/app/v1"
)

type defaultsSetOrgArgs struct {
	Organization string
}

func getDefaultOrg(cCtx *cli.Context) (string, error) {
	config, err := ConfigFromCache(cCtx)
	if err != nil {
		return "", err
	}

	return config.DefaultOrg, nil
}

func writeDefaultOrg(cCtx *cli.Context, orgStr string) error {
	// we're setting a new default org, so try to verify that it actually exists and there's
	// permission to access it
	if orgStr != "" {
		client, err := newViamClient(cCtx)
		if err != nil {
			return err
		}

		if orgs, err := client.listOrganizations(); err != nil {
			warningf(cCtx.App.ErrWriter, "unable to verify existence of org %s: %v", orgStr, err)
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
				if gArgs, err := getGlobalArgs(cCtx); err == nil {
					currProfile, _ := whichProfile(gArgs)
					if currProfile != nil && *currProfile != "" {
						profileWarning = ". You are currently logged in with profile %s. Did you mean to add a default to top level config?"
					}
				}
				return fmt.Errorf("no org found matching name or ID %s%s", orgStr, profileWarning)
			}
		}
	}

	config, err := ConfigFromCache(cCtx)
	if err != nil {
		return err
	}

	config.DefaultOrg = orgStr

	return storeConfigToCache(config)

}

func writeDefaultLocation(cCtx *cli.Context, locStr string) error {
	config, err := ConfigFromCache(cCtx)
	if err != nil {
		return err
	}

	// we're setting a new default location arg, so verify that the location exists and is
	// accessible given the current auth settings and default org argument.
	if locStr != "" {
		client, err := newViamClient(cCtx)
		if err != nil {
			return err
		}
		orgs := []*apppb.Organization{}

		if config.DefaultOrg == "" {
			warningf(cCtx.App.ErrWriter, "attempting to set a default location argument when no default org argument is set."+
				" This can work, but may result in unexpected behavior.")

			orgs, err = client.listOrganizations()
			if err != nil {
				warningf(cCtx.App.ErrWriter, "unable to list organizations to find location %s: %v", locStr, err)
			}
		} else {
			org, err := client.getOrg(config.DefaultOrg)
			if err != nil {
				warningf(cCtx.App.ErrWriter, "unable to lookup org with default org value %s", config.DefaultOrg)
			} else {
				orgs = append(orgs, org)
			}
		}

		locFound := false
		for _, org := range orgs {
			locs, err := client.listLocations(org.Id)
			if err != nil {
				warningf(cCtx.App.ErrWriter, "unable to list locations for org %s: %v", org.Id, err)
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
			if gArgs, err := getGlobalArgs(cCtx); err == nil {
				currProfile, _ := whichProfile(gArgs)
				if currProfile != nil && *currProfile != "" {
					profileWarning = "You are currently logged in with profile %s. Did you mean to add a default to top level config?"
				}
			}
			var forOrgWarning string
			if config.DefaultOrg != "" {
				forOrgWarning = fmt.Sprintf(" in default org %s", config.DefaultOrg)
			}
			return fmt.Errorf("no location found matching name or ID %s%s. %s", locStr, forOrgWarning, profileWarning)
		}
	}

	config.DefaultLocation = locStr
	return storeConfigToCache(config)

}

func defaultsSetOrgAction(cCtx *cli.Context, args defaultsSetOrgArgs) error {
	return writeDefaultOrg(cCtx, args.Organization)
}

func defaultsClearOrgAction(cCtx *cli.Context, args emptyArgs) error {
	return writeDefaultOrg(cCtx, "")
}

type defaultsSetLocationArgs struct {
	Location string
}

func defaultsSetLocationAction(cCtx *cli.Context, args defaultsSetLocationArgs) error {
	return writeDefaultLocation(cCtx, args.Location)
}

func defaultsClearLocationAction(cCtx *cli.Context, args emptyArgs) error {
	return writeDefaultLocation(cCtx, "")
}
