package cli

import "github.com/urfave/cli/v2"

// MLTrainingUploadAction retrieves the logs for a specific build step.
func MLTrainingUploadAction(cCtx *cli.Context) error {
	c, err := newViamClient(cCtx)
	if err != nil {
		return err
	}
	return c.mlTrainingUploadAction(cCtx)
}

func (c *viamClient) mlTrainingUploadAction(cCtx *cli.Context) error {
	manifestPath := cCtx.String(mlTrainingFlagPath)
	manifest, err := loadManifest(manifestPath)
	if err != nil {
		return err
	}

}
