package utils_test

import (
	"encoding/json"
	"testing"
	"time"

	"go.viam.com/rdk/utils"
	"go.viam.com/test"
)

func FuzzDurationJSONRoundtrip(f *testing.F) {
	f.Add("not a duration")
	f.Add("2h")
	f.Add("1m")
	f.Add("5s")
	f.Add("12ms")
	f.Add("8us")
	f.Add("600ns")
	f.Add("1h2m10s")
	f.Add("`0`")
	f.Add("5")
	f.Fuzz(func(t *testing.T, s string) {
		// marshal input to JSON. this should always succeed.
		data, err := json.Marshal(s)
		test.That(t, err, test.ShouldBeNil)

		// unmarshal marshaled input directly to custom [utils.Duration].
		var ud utils.Duration
		errUD := json.Unmarshal(data, &ud)

		// parse input to built-in [time.Duration]
		td, errTD := time.ParseDuration(s)

		// the previous two marshall/parse operations should either both
		// succeed or both fail.
		if errUD != nil || errTD != nil {
			test.That(t, errUD, test.ShouldNotBeNil)
			test.That(t, errTD, test.ShouldNotBeNil)
			return
		}

		// if unmarshaling/parsing is successful, both durations should be equal.
		test.That(t, ud.Unwrap(), test.ShouldEqual, td)

		// marshal custom [util.Duration] value back to JSON. this should always succeed.
		// note that the resulting JSON value might not match initially marshaled input string.
		// the marshaling function for [util.Duration] is using [time.Duration.String], which
		// might "pad" the resulting string with zero values duration types.
		// for example, the following marshaling roundtrip is possible:
		// `"2h"` -> 2 * time.Hour -> `"2h0m0s""`.
		jsonUD, err := json.Marshal(ud)
		test.That(t, err, test.ShouldBeNil)

		// stringify and marshal built-in [time.Duration] value to JSON.
		// this should always succeed and match the result of marshaling the custom
		// [util.Duration] value.
		jsonTD, err := json.Marshal(td.String())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, jsonUD, test.ShouldResemble, jsonTD)
	})
}
