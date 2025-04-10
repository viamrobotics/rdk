package cli

import (
	"os"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.viam.com/test"
)

var (
	mqlString = `[{"$match": {"component_name": "dragino"}}, {"$group": {"_id": "$part_id", "count": {"$sum": 1}}}]`
	mqlBSON   = []bson.M{
		{"$match": bson.M{"component_name": "dragino"}},
		{"$group": bson.M{"_id": "$part_id", "count": bson.M{"$sum": 1}}},
	}
)

func TestParseMQL(t *testing.T) {
	expectedBytes := expectedBSONBytes(t)
	testCases := map[string]struct {
		mqlString     string
		mqlFile       string
		expectedError bool
		expectedBytes [][]byte
	}{
		"valid MQL string": {
			mqlString:     mqlString,
			mqlFile:       "",
			expectedError: false,
			expectedBytes: expectedBytes,
		},
		"valid MQL file": {
			mqlString:     "",
			mqlFile:       createTempMQLFile(t),
			expectedError: false,
			expectedBytes: expectedBytes,
		},
		"empty string and file": {
			mqlString:     "",
			mqlFile:       "",
			expectedError: true,
		},
		"both string and file provided": {
			mqlString:     mqlString,
			mqlFile:       createTempMQLFile(t),
			expectedError: true,
		},
		"invalid MQL JSON string": {
			mqlString:     `[{"$match": {"component_name": "dragino"}}`, // missing closing bracket
			mqlFile:       "",
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
			test.That(t, mqlBytes, test.ShouldResemble, tc.expectedBytes)
		})
	}
}

func createTempMQLFile(t *testing.T) string {
	t.Helper()

	f, err := os.CreateTemp("", "mql.json")
	test.That(t, err, test.ShouldBeNil)

	_, err = f.WriteString(mqlString)
	test.That(t, err, test.ShouldBeNil)
	err = f.Close()
	test.That(t, err, test.ShouldBeNil)

	return f.Name()
}

func expectedBSONBytes(t *testing.T) [][]byte {
	t.Helper()

	expectedBSONBytes := make([][]byte, len(mqlBSON))
	var err error
	for i, bsonDoc := range mqlBSON {
		expectedBSONBytes[i], err = bson.Marshal(bsonDoc)
		if err != nil {
			break
		}
	}
	return expectedBSONBytes
}
