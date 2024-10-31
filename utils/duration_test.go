package utils_test

import (
	"encoding/json"
	"testing"
	"time"

	"go.viam.com/rdk/utils"
	"go.viam.com/test"
)

func FuzzDurationJSON(f *testing.F) {
	f.Add("not a duration")
	f.Add("2h")
	f.Add("1m")
	f.Add("5s")
	f.Add("12ms")
	f.Add("8us")
	f.Add("600ns")
	f.Add("1h2m10s")
	f.Fuzz(func(t *testing.T, s string) {
		data := []byte(s)
		var (
			ud *utils.Duration
			td *time.Duration
		)

		errUD := json.Unmarshal(data, &ud)
		errTD := json.Unmarshal(data, &td)

		if errUD != nil || errTD != nil {
			test.That(t, errUD, test.ShouldResemble, errTD)
			return
		}

		test.That(t, ud, test.ShouldEqualJSON, td)
	})
}
