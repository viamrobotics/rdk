package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.uber.org/multierr"
)

const profileEnvVar = "VIAM_CLI_PROFILE_NAME"

type addOrUpdateProfileArgs struct {
	ProfileName string
	Key         string
	KeyID       string
}

func getProfiles() (map[string]profile, error) {
	rd, err := os.ReadFile(getCLIProfilesPath())
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		rd = make([]byte, 0)
	}

	profiles := make(map[string]profile)
	if err := json.Unmarshal(rd, &profiles); err != nil {
		return nil, err
	}
	return profiles, nil
}

func writeProfiles(profiles map[string]profile) error {
	md, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(getCLIProfilesPath(), md, 0o640)

}

func addOrUpdateProfile(c *cli.Context, args addOrUpdateProfileArgs, isAdd bool) error {
	profiles, err := getProfiles()
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		profiles = make(map[string]profile)
	}

	profile, alreadyExists := profiles[args.ProfileName]
	if isAdd && alreadyExists {
		return errors.New(fmt.Sprintf("Attempted to add new profile %s but it already existed", args.ProfileName))
	}
	profile.APIKey.KeyCrypto = args.Key
	profile.APIKey.KeyID = args.KeyID
	profile.Name = args.ProfileName
	profiles[args.ProfileName] = profile

	return writeProfiles(profiles)
}

func AddProfileAction(c *cli.Context, args addOrUpdateProfileArgs) error {
	return addOrUpdateProfile(c, args, true)
}

func UpdateProfileAction(c *cli.Context, args addOrUpdateProfileArgs) error {
	return addOrUpdateProfile(c, args, false)
}

type removeProfileArgs struct {
	ProfileName string
}

func whichProfile(args *globalArgs) (_ *string, profileSpecified bool) {
	// profile hasn't been specified for this command
	if args.Profile != "" {
		profileSpecified = true
		return &args.Profile, profileSpecified
	}

	if envProfile := os.Getenv(profileEnvVar); envProfile != "" {
		return &envProfile, profileSpecified
	}

	return nil, profileSpecified
}

func RemoveProfileAction(c *cli.Context, args removeProfileArgs) error {
	profiles, err := getProfiles()
	if err != nil {
		return err
	}

	delete(profiles, args.ProfileName)
	removeConfErr := os.Remove(getCLIProfilePath(args.ProfileName))
	writeProfilesErr := writeProfiles(profiles)

	return multierr.Combine(removeConfErr, writeProfilesErr)
}

func ListProfilesAction(c *cli.Context, args emptyArgs) error {
	profiles, err := getProfiles()
	if err != nil {
		return err
	}

	for p := range profiles {
		printf(c.App.Writer, p)
	}

	return nil
}

type profile struct {
	Name   string
	APIKey apiKey
}
