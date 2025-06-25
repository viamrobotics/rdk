package cli

import (
	"encoding/json"
	"os"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	pb "go.viam.com/api/app/data/v1"
	"go.viam.com/test"
)

var (
	mqlString = `[
		{"$match": { "component_name": "dragino" }},
		{"$group": {
			"_id": "$part_id",
			"count": { "$sum": 1 }, // a comment just for fun
			"avgTemp": { "$avg": "$data.readings.TempC_SHT" },
			"avgHum": { "$avg": "$data.readings.Hum_SHT" }
		}},
	]`
	mqlBSON = []bson.M{
		{"$match": bson.M{"component_name": "dragino"}},
		{"$group": bson.M{
			"_id":     "$part_id",
			"count":   bson.M{"$sum": 1},
			"avgTemp": bson.M{"$avg": "$data.readings.TempC_SHT"},
			"avgHum":  bson.M{"$avg": "$data.readings.Hum_SHT"},
		}},
	}
)

func TestParseMQL(t *testing.T) {
	testCases := map[string]struct {
		mqlString     string
		mqlFile       string
		expectedError bool
		expectedBSON  []bson.M
	}{
		"valid MQL string": {
			mqlString:     mqlString,
			mqlFile:       "",
			expectedError: false,
			expectedBSON:  mqlBSON,
		},
		"valid MQL file": {
			mqlString:     "",
			mqlFile:       createTempMQLFile(t, mqlString),
			expectedError: false,
			expectedBSON:  mqlBSON,
		},
		"empty string and file": {
			mqlString:     "",
			mqlFile:       "",
			expectedError: true,
		},
		"both string and file provided": {
			mqlString:     mqlString,
			mqlFile:       createTempMQLFile(t, mqlString),
			expectedError: true,
		},
		"invalid MQL JSON string": {
			mqlString:     `[{"$match": {"component_name": "dragino"`, // missing closing brackets
			mqlFile:       "",
			expectedError: true,
		},
		"invalid MQL JSON file": {
			mqlString:     "",
			mqlFile:       createTempMQLFile(t, `[{"$match": {"component_name": "dragino"`), // missing closing brackets
			expectedError: true,
		},
		"invalid MQL file path": {
			mqlString:     "",
			mqlFile:       "invalid/path/to/mql.json",
			expectedError: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			if tc.mqlFile != "" {
				defer os.Remove(tc.mqlFile)
			}

			mqlBytes, err := parseMQL(tc.mqlString, tc.mqlFile)
			if tc.expectedError {
				test.That(t, err, test.ShouldNotBeNil)
				return
			}
			test.That(t, err, test.ShouldBeNil)

			for i, bsonBytes := range mqlBytes {
				var bsonM bson.M
				err = bson.Unmarshal(bsonBytes, &bsonM)
				test.That(t, err, test.ShouldBeNil)
				testBSONResemble(t, bsonM, tc.expectedBSON[i])
			}
		})
	}
}

// testBSONResemble compares two bson.M objects and asserts that they are equal.
func testBSONResemble(t *testing.T, actual, expected bson.M) {
	t.Helper()

	actualJSON, err := json.Marshal(actual)
	test.That(t, err, test.ShouldBeNil)

	expectedJSON, err := json.Marshal(expected)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, string(actualJSON), test.ShouldEqualJSON, string(expectedJSON))
}

func createTempMQLFile(t *testing.T, mql string) string {
	t.Helper()

	f, err := os.CreateTemp("", "mql.json")
	test.That(t, err, test.ShouldBeNil)

	_, err = f.WriteString(mql)
	test.That(t, err, test.ShouldBeNil)
	err = f.Close()
	test.That(t, err, test.ShouldBeNil)

	return f.Name()
}

func TestMQLJSON(t *testing.T) {
	// expectedJSON is a vanilla JSON representation of the MQL string.
	expectedJSON := `[{"$match":{"component_name":"dragino"}},
		{"$group":{"_id":"$part_id","count":{"$sum":1},
			"avgTemp":{"$avg":"$data.readings.TempC_SHT"},
			"avgHum":{"$avg":"$data.readings.Hum_SHT"}}}]`

	bsonBytes := make([][]byte, len(mqlBSON))
	var err error
	for i, bsonDoc := range mqlBSON {
		bsonBytes[i], err = bson.Marshal(bsonDoc)
		if err != nil {
			break
		}
	}
	json, err := mqlJSON(bsonBytes)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, json, test.ShouldEqualJSON, expectedJSON)
}

func TestDataSourceTypeToProto(t *testing.T) {
	testCases := map[string]struct {
		dataSourceType string
		expectedType   pb.TabularDataSourceType
		expectedError  bool
	}{
		"standard": {
			dataSourceType: "standard",
			expectedType:   pb.TabularDataSourceType_TABULAR_DATA_SOURCE_TYPE_STANDARD,
			expectedError:  false,
		},
		"hotstorage": {
			dataSourceType: "hotstorage",
			expectedType:   pb.TabularDataSourceType_TABULAR_DATA_SOURCE_TYPE_HOT_STORAGE,
			expectedError:  false,
		},
		"unknown": {
			dataSourceType: "unknown",
			expectedError:  true,
		},
		"empty": {
			dataSourceType: "",
			expectedType:   pb.TabularDataSourceType_TABULAR_DATA_SOURCE_TYPE_STANDARD,
			expectedError:  true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			dataSourceType, err := dataSourceTypeToProto(tc.dataSourceType)
			if tc.expectedError {
				test.That(t, err, test.ShouldNotBeNil)
				return
			}
			test.That(t, err, test.ShouldBeNil)
			test.That(t, dataSourceType, test.ShouldEqual, tc.expectedType)
		})
	}
}
