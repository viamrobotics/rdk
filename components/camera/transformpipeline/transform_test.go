//go:build !no_media

package transformpipeline

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/invopop/jsonschema"
	"go.viam.com/test"
)

func TestJSONSchema(t *testing.T) {
	tr := &transformConfig{}
	schema := jsonschema.Reflect(tr)
	jsonBytes, err := json.MarshalIndent(schema, "", "  ")
	test.That(t, err, test.ShouldBeNil)
	jsonString := string(jsonBytes)
	for transformName := range registeredTransformConfigs {
		test.That(t, jsonString, test.ShouldContainSubstring, fmt.Sprintf("\"title\": \"%s\"", transformName))
	}
}
