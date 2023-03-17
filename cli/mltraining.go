package cli

import (
	"context"
	mlpb "go.viam.com/api/app/mltraining/v1"
)

func (c *AppClient) SubmitTrainingJob(ctx context.Context, req *mlpb.SubmitTrainingJobRequest) (*mlpb.SubmitTrainingJobResponse, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}

	return c.mlClient.SubmitTrainingJob(ctx, req)
}
