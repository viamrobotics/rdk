package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

type addOrUpdateProfileArgs struct {
	ProfileName string
	Key         string
	KeyID       string
}

func getProfiles() (*profiles, error) {
	rd, err := os.ReadFile(getCLIProfilesPath())
	if err != nil {
		return nil, err
	}

	var profiles profiles
	if err := json.Unmarshal(rd, profiles); err != nil {
		return nil, err
	}
	return &profiles, nil
}

func writeProfiles(profiles profiles) error {
	md, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(getCLIProfilesPath(), md, 0o640)

}

func addOrUpdateProfile(c *cli.Context, args addOrUpdateProfileArgs, isAdd bool) error {
	profiles, err := getProfiles()
	if err != nil {
		return err
	}

	apiKey := apiKey{}
	if profiles.profiles[args.ProfileName] != apiKey && isAdd {
		return errors.New(fmt.Sprintf("Attempted to add new profile %s but it already existed", args.ProfileName))
	}
	apiKey.KeyCrypto = args.Key
	apiKey.KeyID = args.KeyID
	profiles.profiles[args.ProfileName] = apiKey

	return writeProfiles(*profiles)
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

func RemoveProfileAction(c *cli.Context, args removeProfileArgs) error {
	profiles, err := getProfiles()
	if err != nil {
		return err
	}

	delete(profiles.profiles, args.ProfileName)

	return writeProfiles(*profiles)
}

type profiles struct {
	currentProfile string
	profiles       map[string]apiKey
}
