package module

import (
	"context"
	"errors"
	"os"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/app"
	"go.viam.com/rdk/utils"
)

type mockClient struct {
	orgID        string
	partID       string
	resourceName string
}

func (m *mockClient) TabularDataByMQL(
	ctx context.Context, orgID string, query []map[string]any, opts *app.TabularDataByMQLOptions,
) ([]map[string]any, error) {
	m.orgID = orgID
	queryMap, ok := query[0]["$match"].(map[string]any)

	if !ok {
		return nil, errors.New("Type error")
	}

	m.partID, ok = queryMap["part_id"].(string)
	if !ok {
		return nil, errors.New("Type error")
	}

	m.resourceName, ok = queryMap["component_name"].(string)
	if !ok {
		return nil, errors.New("Type error")
	}

	return query, nil
}

func TestQueryTabularDataForResource(t *testing.T) {
	client := &mockClient{}

	os.Setenv(utils.PrimaryOrgIDEnvVar, "my_org")
	os.Setenv(utils.MachinePartIDEnvVar, "part")

	dataConsumer := &ResourceDataConsumer{dataClient: client}
	query, err := dataConsumer.QueryTabularDataForResource(context.Background(), "resource", nil)

	test.That(t, err, test.ShouldBeNil)
	test.That(t, query, test.ShouldHaveLength, 1)
	test.That(t, client.orgID, test.ShouldEqual, "my_org")
	test.That(t, client.partID, test.ShouldEqual, "part")
	test.That(t, client.resourceName, test.ShouldEqual, "resource")
}
