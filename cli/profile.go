package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
)

// CLI profiles allow a user to run commands with a specific API key without having to
// log out and log back in. A profile is added with `viam profiles add`, updated with
// `viam profiles udpate`, etc. Once a profile is added, a command can be run using a
// profile by using the `--profile {profile name}` global flag.
//
// If one wants to use a profile for an entire shell session without having to specify it
// each time, the user can set the `VIAM_CLI_PROFILE_NAME` env var. The CLI will then use
// that env var to find a profile, provided the global flag is not set. To override the env
// var without unsetting it, simply use the `--disable-profiles` flag.

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

	//nolint:gosec
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
		return fmt.Errorf("attempted to add new profile %s but it already existed", args.ProfileName)
	}
	profile.APIKey.KeyCrypto = args.Key
	profile.APIKey.KeyID = args.KeyID
	profile.Name = args.ProfileName
	profiles[args.ProfileName] = profile

	if err := writeProfiles(profiles); err != nil {
		return err
	}

	var addOrUpdate string
	if isAdd {
		addOrUpdate = "added"
	} else {
		addOrUpdate = "updated"
	}

	printf(c.App.Writer, "Successfully %s profile %s", addOrUpdate, args.ProfileName)
	return nil
}

// AddProfileAction adds a new CLI profile.
func AddProfileAction(c *cli.Context, args addOrUpdateProfileArgs) error {
	return addOrUpdateProfile(c, args, true)
}

// UpdateProfileAction updates an existing CLI profile, or adds it if it doesn't already exist.
func UpdateProfileAction(c *cli.Context, args addOrUpdateProfileArgs) error {
	return addOrUpdateProfile(c, args, false)
}

type removeProfileArgs struct {
	ProfileName string
}

// bool return indicates whether a profile has been specified, as opposed to either not existing
// or existing as an env var. This is relevant because we want to fall back on default behavior
// if an env var profile isn't found, but return an error if a profile specified with the `--profile`
// flag isn't found.
func whichProfile(args *globalArgs) (*string, bool) {
	// profile hasn't been specified for this command
	if args.Profile != "" {
		return &args.Profile, true
	}

	if envProfile := os.Getenv(profileEnvVar); envProfile != "" {
		return &envProfile, false
	}

	return nil, false
}

// RemoveProfileAction removes a CLI profile.
func RemoveProfileAction(c *cli.Context, args removeProfileArgs) error {
	profiles, err := getProfiles()
	if err != nil {
		return err
	}

	delete(profiles, args.ProfileName)
	if err := os.Remove(getCLIProfilePath(args.ProfileName)); err != nil {
		return err
	}
	if err := writeProfiles(profiles); err != nil {
		return err
	}

	printf(c.App.Writer, "Successfully deleted profile %s", args.ProfileName)

	return nil
}

// ListProfilesAction lists all currently supported profiles.
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
