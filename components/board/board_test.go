package board_test

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/board"
	_ "go.viam.com/rdk/components/board/fake"
	_ "go.viam.com/rdk/components/motor/fake"
	"go.viam.com/rdk/logging"
	robotimpltest "go.viam.com/rdk/robot/impltest"
	"go.viam.com/rdk/testutils"
)

func TestFromRobot(t *testing.T) {
	jsonData := `{
		"components": [
			{
				"name": "board1",
				"type": "board",
				"model": "fake",
				"attributes": {
					"digital_interrupts": [
						{
							"name": "encoder",
							"pin": "14"
						}
					]
				}
			},
			{
				"name": "m1",
				"type": "motor",
				"model": "fake",
				"attributes": {
					"board": "board1",
					"pins": {
						"pwm": "1"
					},
					"pwm_freq": 1000
				}
			}
		]
	}`

	conf := testutils.ConfigFromJSON(t, jsonData)
	logger := logging.NewTestLogger(t)
	r := robotimpltest.SetupLocalRobot(t, context.Background(), conf, logger)

	expected := []string{"board1"}
	testutils.VerifySameElements(t, board.NamesFromRobot(r), expected)

	b, err := board.FromRobot(r, "board1")
	test.That(t, err, test.ShouldBeNil)
	pin, err := b.GPIOPinByName("1")
	test.That(t, err, test.ShouldBeNil)
	pwmF, err := pin.PWMFreq(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pwmF, test.ShouldEqual, 1000)
	_, err = b.DigitalInterruptByName("encoder")
	test.That(t, err, test.ShouldBeNil)

	_, err = board.FromRobot(r, "m0")
	test.That(t, err, test.ShouldNotBeNil)
}
