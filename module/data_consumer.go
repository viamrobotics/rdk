package module

import (
	"context"
	"os"
	"sync"
	"time"

	"go.viam.com/rdk/app"
	"go.viam.com/rdk/utils"
)

// QueryTabularDataOptions provides additional input for QueryTabularDataForResource.
type QueryTabularDataOptions struct {
	TimeBack         time.Duration
	AdditionalStages []map[string]any

	app.TabularDataByMQLOptions
}

type queryBackend interface {
	TabularDataByMQL(context.Context, string, []map[string]interface{}, *app.TabularDataByMQLOptions) ([]map[string]interface{}, error)
}

// ResourceDataConsumer can be added as an anonymous struct member to a resource to enable historical module data queries.
type ResourceDataConsumer struct {
	// mu guards the lazy initialization of dataClient. This is embedded in
	// resources, whose methods the RDK may call concurrently, so the nil check
	// and the assignment in setDataClient have to happen under one lock.
	mu         sync.Mutex
	dataClient queryBackend
}

func (r *ResourceDataConsumer) setDataClient(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.dataClient != nil {
		return nil
	}

	viamClient, err := app.CreateViamClientFromEnvVars(ctx, nil, nil)
	if err != nil {
		return err
	}
	r.dataClient = viamClient.DataClient()
	return nil
}

// QueryTabularDataForResource will return historical data for a resource.
func (r *ResourceDataConsumer) QueryTabularDataForResource(
	ctx context.Context, resourceName string, opts *QueryTabularDataOptions,
) ([]map[string]any, error) {
	err := r.setDataClient(ctx)
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
	var queryOpts *app.TabularDataByMQLOptions
	if opts != nil {
		query = append(query, opts.AdditionalStages...)
		queryOpts = &opts.TabularDataByMQLOptions
	}

	return r.dataClient.TabularDataByMQL(ctx, orgID, query, queryOpts)
}
