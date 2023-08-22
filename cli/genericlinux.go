package cli

import (
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

func UploadBoardDefAction(c *cli.Context) error {
	orgIDArg := c.String(boardFlagOrgID)
	nameArg := c.String(boardFlagName)
	versionArg := c.String(boardFlagVersion)
	tarballPath := c.Args().First()
	if c.Args().Len() > 1 {
		return errors.New("too many arguments passed to upload command. " +
			"make sure to specify flag and optional arguments before the required positional package argument")
	}
	if tarballPath == "" {
		return errors.New("no package to upload -- please provide an archive containing your board file. use --help for more information")
	}

	client, err := newAppClient(c)
	if err != nil {
		return err
	}

	//nolint:gosec
	file, err := os.Open(tarballPath)
	if err != nil {
		return err
	}
	// TODO(APP-2226): support .tar.xz
	if !strings.HasSuffix(file.Name(), ".tar.gz") {
		return errors.New("you must upload your module in the form of a .tar.gz")
	}
	response, err := client.uploadBoardDefFile(moduleID, versionArg, platformArg, file)
	if err != nil {
		return err
	}
	return nil
}
