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
			collectionType: "hot_store",
			pipelineName:   "",
			expectedType:   hotStoreCollectionType,
			expectedError:  nil,
		},
		"pipeline_sink_with_pipeline": {
			collectionType: "pipeline_sink",
			pipelineName:   "my-pipeline",
			expectedType:   pipelineSinkCollectionType,
			expectedError:  nil,
		},
		"pipeline_sink_without_pipeline": {
			collectionType: "pipeline_sink",
			pipelineName:   "",
			expectedType:   unspecifiedCollectionType,
			expectedError:  errPipelineNameRequired,
		},
		"hot_store_with_pipeline": {
			collectionType: "hot_store",
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
			collectionType: "HOT_STORE",
			pipelineName:   "",
			expectedType:   unspecifiedCollectionType,
			expectedError:  errInvalidCollectionType,
		},
		"pipeline_sink_invalid_case_with_pipeline": {
			collectionType: "PIPELINE_SINK",
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
		"valid_json_array": {
			fileContent: `[
				{"name": 1, "email": -1},
				{"unique": true}
			]`,
			expectedResult: [][]byte{
				[]byte(`{"name": 1, "email": -1}`),
				[]byte(`{"unique": true}`),
			},
			expectedError: false,
		},
		"empty_array": {
			fileContent:    `[]`,
			expectedResult: [][]byte{},
			expectedError:  false,
		},
		"single_element": {
			fileContent: `[
				{"name": 1}
			]`,
			expectedResult: [][]byte{
				[]byte(`{"name": 1}`),
			},
			expectedError: false,
		},
		"invalid_json": {
			fileContent:   `{"name": 1}`,
			expectedError: true,
		},
		"malformed_json": {
			fileContent:   `[{"name": 1`,
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
