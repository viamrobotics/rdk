package rexec

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/edaniels/test"
)

func TestProcessConfigRoundTripJSON(t *testing.T) {
	config := ProcessConfig{
		Name:    "hello",
		Args:    []string{"1", "2", "3"},
		CWD:     "dir",
		OneShot: true,
		Log:     true,
	}
	md, err := json.Marshal(config)
	test.That(t, err, test.ShouldBeNil)

	var rt ProcessConfig
	test.That(t, json.Unmarshal(md, &rt), test.ShouldBeNil)
	test.That(t, rt, test.ShouldResemble, config)

	var rtLower ProcessConfig
	test.That(t, json.Unmarshal(bytes.ToLower(md), &rtLower), test.ShouldBeNil)
	test.That(t, rtLower, test.ShouldResemble, config)
}
