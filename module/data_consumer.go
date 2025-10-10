package module

import (
	"context"
	"os"
	"time"

	"go.viam.com/rdk/app"
	"go.viam.com/rdk/utils"
)

type TabularDataOptions struct {
	TabularDataSourceType app.TabularDataSourceType
}

type ResourceDataConsumer struct {
	_dataClient *app.DataClient
}

func (r *ResourceDataConsumer) dataClient(ctx context.Context) (*app.DataClient, error) {
	if r._dataClient != nil {
		return r._dataClient, nil
	}
	viamClient, err := app.CreateViamClientFromEnvVars(ctx, nil, nil)
	if err != nil {
		return nil, err
	}
	r._dataClient = viamClient.DataClient()
	return r._dataClient, nil
}

func (r ResourceDataConsumer) TabularData(ctx context.Context, resourceName string, opts *TabularDataOptions) ([]map[string]any, error) {
	dataClient, err := r.dataClient(ctx)
	if err != nil {
		return nil, err
	}
	
	orgId := os.Getenv(utils.PrimaryOrgIDEnvVar)
	partId := os.Getenv(utils.MachinePartIDEnvVar)

	options := &TabularDataOptions{}
	if opts != nil {
		options = opts
	}

	query := []map[string]any{
		{
			"$match": map[string]any{
				"part_id":   partId,
				"component_name": resourceName,
				"time_received": map[string]any{
					"$gte": time.Now().Add(-24 * time.Hour),
				},
			},
		},
	}
	
	return dataClient.TabularDataByMQL(ctx, orgId, query, &app.TabularDataByMQLOptions{TabularDataSourceType: options.TabularDataSourceType})
}
