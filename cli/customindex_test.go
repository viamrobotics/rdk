package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"os"
	"testing"

	"github.com/urfave/cli/v2"
	pb "go.viam.com/api/app/data/v1"
	"go.viam.com/test"
)

func TestValidateCollectionTypeArgs(t *testing.T) {
	testCases := map[string]struct {
		collectionType string
		pipelineName   string
		expectedType   pb.IndexableCollection
		expectedError  error
	}{
		"hot_store_without_pipeline": {
			collectionType: "hot-storage",
			pipelineName:   "",
			expectedType:   hotStoreCollectionType,
			expectedError:  nil,
		},
		"pipeline_sink_with_pipeline": {
			collectionType: "pipeline-sink",
			pipelineName:   "my-pipeline",
			expectedType:   pipelineSinkCollectionType,
			expectedError:  nil,
		},
		"pipeline_sink_without_pipeline": {
			collectionType: "pipeline-sink",
			pipelineName:   "",
			expectedType:   unspecifiedCollectionType,
			expectedError:  errPipelineNameRequired,
		},
		"hot_store_with_pipeline": {
			collectionType: "hot-storage",
			pipelineName:   "my-pipeline",
			expectedType:   unspecifiedCollectionType,
			expectedError:  errPipelineNameNotAllowed,
		},
		"unknown_collection_type": {
			collectionType: "unknown",
			pipelineName:   "",
			expectedType:   unspecifiedCollectionType,
			expectedError:  errInvalidCollectionType,
		},
		"empty_collection_type": {
			collectionType: "",
			pipelineName:   "",
			expectedType:   unspecifiedCollectionType,
			expectedError:  errInvalidCollectionType,
		},
		"invalid_case_sensitivity": {
			collectionType: "HOT-STORAGE",
			pipelineName:   "",
			expectedType:   unspecifiedCollectionType,
			expectedError:  errInvalidCollectionType,
		},
		"pipeline_sink_invalid_case_with_pipeline": {
			collectionType: "PIPELINE-SINK",
			pipelineName:   "my-pipeline",
			expectedType:   unspecifiedCollectionType,
			expectedError:  errInvalidCollectionType,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			app := &cli.App{}
			set := flag.NewFlagSet("test", 0)
			set.String(dataFlagCollectionType, tc.collectionType, "")
			set.String(dataFlagPipelineName, tc.pipelineName, "")
			set.Parse(nil)
			ctx := cli.NewContext(app, set, nil)

			collectionType, err := validateCollectionTypeArgs(ctx, tc.collectionType)

			if tc.expectedError != nil {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err, test.ShouldEqual, tc.expectedError)
			} else {
				test.That(t, err, test.ShouldBeNil)
			}
			test.That(t, collectionType, test.ShouldEqual, tc.expectedType)
		})
	}
}

func TestReadJSONToByteSlices(t *testing.T) {
	testCases := map[string]struct {
		fileContent    string
		expectedResult [][]byte
		expectedError  bool
	}{
		"valid_with_key_and_options": {
			fileContent: `{
				"key": {"resource_name": 1, "method_name": 1},
				"options": {"sparse": true}
			}`,
			expectedResult: [][]byte{
				[]byte(`{"resource_name": 1, "method_name": 1}`),
				[]byte(`{"sparse": true}`),
			},
			expectedError: false,
		},
		"valid_with_key_only": {
			fileContent: `{
				"key": {"name": 1, "email": -1}
			}`,
			expectedResult: [][]byte{
				[]byte(`{"name": 1, "email": -1}`),
			},
			expectedError: false,
		},
		"valid_with_empty_options": {
			fileContent: `{
				"key": {"timestamp": -1},
				"options": {}
			}`,
			expectedResult: [][]byte{
				[]byte(`{"timestamp": -1}`),
				[]byte(`{}`),
			},
			expectedError: false,
		},
		"missing_key_field": {
			fileContent: `{
				"options": {"unique": true}
			}`,
			expectedError: true,
		},
		"invalid_json_structure": {
			fileContent: `[
				{"key": {"name": 1}}
			]`,
			expectedError: true,
		},
		"malformed_json": {
			fileContent:   `{"key": {"name": 1}`,
			expectedError: true,
		},
		"empty_object": {
			fileContent:   `{}`,
			expectedError: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "test-*.json")
			test.That(t, err, test.ShouldBeNil)
			defer os.Remove(tmpFile.Name())

			_, err = tmpFile.WriteString(tc.fileContent)
			test.That(t, err, test.ShouldBeNil)
			tmpFile.Close()

			result, err := readJSONToByteSlices(tmpFile.Name())
			if tc.expectedError {
				test.That(t, err, test.ShouldNotBeNil)
				return
			}

			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(result), test.ShouldEqual, len(tc.expectedResult))

			// Compare each byte slice (ignoring whitespace differences in JSON)
			for i := range result {
				var resultJSON, expectedJSON interface{}
				err := json.Unmarshal(result[i], &resultJSON)
				test.That(t, err, test.ShouldBeNil)
				err = json.Unmarshal(tc.expectedResult[i], &expectedJSON)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, resultJSON, test.ShouldResemble, expectedJSON)
			}
		})
	}

	t.Run("file_not_found", func(t *testing.T) {
		_, err := readJSONToByteSlices("nonexistent-file.json")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, errors.Is(err, os.ErrNotExist), test.ShouldBeTrue)
	})
}
