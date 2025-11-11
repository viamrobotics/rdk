package module

import (
	"context"
	"os"
	"time"

	datapb "go.viam.com/api/app/data/v1"

	"go.viam.com/rdk/app"
	"go.viam.com/rdk/utils"
)

// QueryTabularDataOptions provides additional input for QueryTabularDataForResource.
type QueryTabularDataOptions struct {
	TimeBack         time.Duration
	AdditionalStages []map[string]any
}

// ResourceDataConsumer can be added as an anonymous struct member to a resource to enable historical module data queries.
type ResourceDataConsumer struct {
	_dataClient *app.DataClient
}

// DataClient is a constructor method for initializing the DataClient of ResourceDataConsumer.
// The client parameter should be nil in most usage except unit tests.
func (r *ResourceDataConsumer) DataClient(ctx context.Context, client datapb.DataServiceClient) (*app.DataClient, error) {
	if r._dataClient != nil {
		return r._dataClient, nil
	}

	if client != nil {
		r._dataClient = app.CreateDataClientWithDataServiceClient(client)
		return r._dataClient, nil
	}

	viamClient, err := app.CreateViamClientFromEnvVars(ctx, nil, nil)
	if err != nil {
		return nil, err
	}
	r._dataClient = viamClient.DataClient()
	return r._dataClient, nil
}

// QueryTabularDataForResource will return historical data for a resource.
func (r ResourceDataConsumer) QueryTabularDataForResource(
	ctx context.Context, resourceName string, opts *QueryTabularDataOptions,
) ([]map[string]any, error) {
	dataClient, err := r.DataClient(ctx, nil)
	if err != nil {
		return nil, err
	}

	orgID := os.Getenv(utils.PrimaryOrgIDEnvVar)
	partID := os.Getenv(utils.MachinePartIDEnvVar)

	timeBack := -24 * time.Hour
	if opts != nil && opts.TimeBack != 0 {
		timeBack = opts.TimeBack
	}
	if timeBack > 0 {
		timeBack = -timeBack
	}

	query := []map[string]any{
		{
			"$match": map[string]any{
				"part_id":        partID,
				"component_name": resourceName,
				"time_received": map[string]any{
					"$gte": time.Now().Add(timeBack),
				},
			},
		},
	}

	if opts != nil {
		query = append(query, opts.AdditionalStages...)
	}

	return dataClient.TabularDataByMQL(ctx, orgID, query, nil)
}
