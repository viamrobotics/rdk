package encoder_test

import (
	"context"
	"testing"

	"go.viam.com/test"

	_ "go.viam.com/rdk/components/board/fake"
	"go.viam.com/rdk/components/encoder"
	_ "go.viam.com/rdk/components/encoder/incremental"
	"go.viam.com/rdk/logging"
	robotimpltest "go.viam.com/rdk/robot/impltest"
	"go.viam.com/rdk/testutils"
)

func TestFromRobot(t *testing.T) {
	jsonData := `{
		"components": [
			{
				"name": "e1",
				"type": "encoder",
				"model": "incremental",
				"depends_on": [
					"board1"
				],
				"attributes": {
					"board": "board1",
					"pins": {
						"a": "encoder",
						"b": "encoder-b"
					}
				}
			},
			{
				"name": "board1",
				"type": "board",
				"model": "fake",
				"attributes": {
					"digital_interrupts": [
						{
							"name": "encoder",
							"pin": "14"
						},
						{
							"name": "encoder-b",
							"pin": "15"
						}
					]
				}
			}
		]
	}`

	conf := testutils.ConfigFromJSON(t, jsonData)
	logger := logging.NewTestLogger(t)
	r := robotimpltest.SetupLocalRobot(t, context.Background(), conf, logger)

	expected := []string{"e1"}
	testutils.VerifySameElements(t, encoder.NamesFromRobot(r), expected)

	enc, err := encoder.FromRobot(r, "e1")
	test.That(t, err, test.ShouldBeNil)
	props, err := enc.Properties(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, props.TicksCountSupported, test.ShouldBeTrue)
	test.That(t, props.AngleDegreesSupported, test.ShouldBeFalse)

	_, err = encoder.FromRobot(r, "e0")
	test.That(t, err, test.ShouldNotBeNil)
}
