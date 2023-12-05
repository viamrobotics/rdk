package logging

import (
	"encoding/json"
	"testing"

	"go.viam.com/test"
)

func TestRoundTrip(t *testing.T) {
	for _, level := range []Level{DEBUG, INFO, WARN, ERROR} {
		serialzied := level.String()
		parsed, err := LevelFromString(serialzied)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, parsed, test.ShouldEqual, level)
	}
}

func TestJSONRoundTrip(t *testing.T) {
	type AllLevelStruct struct {
		Debug Level
		Info  Level
		Warn  Level
		Error Level
	}

	levels := AllLevelStruct{DEBUG, INFO, WARN, ERROR}

	serialized, err := json.Marshal(levels)
	test.That(t, err, test.ShouldBeNil)

	var parsed AllLevelStruct
	json.Unmarshal(serialized, &parsed)
	test.That(t, levels, test.ShouldResemble, parsed)
}

func TestJSONErrors(t *testing.T) {
	var level Level
	err := json.Unmarshal([]byte(`{}`), &level)
	test.That(t, err, test.ShouldNotBeNil)
	err = json.Unmarshal([]byte(`Debug"`), &level)
	test.That(t, err, test.ShouldNotBeNil)
	err = json.Unmarshal([]byte(`"not a level"`), &level)
	test.That(t, err, test.ShouldNotBeNil)
}
