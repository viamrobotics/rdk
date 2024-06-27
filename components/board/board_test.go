package board_test

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func TestFromRobot(t *testing.T) {
	r := &inject.Robot{}
	rs := map[resource.Name]resource.Resource{
		board.Named("board1"): inject.NewBoard("board1"),
		generic.Named("g"):    inject.NewGenericComponent("g"),
	}
	r.MockResourcesFromMap(rs)

	expected := []string{"board1"}
	testutils.VerifySameElements(t, board.NamesFromRobot(r), expected)

	_, err := board.FromRobot(r, "board1")
	test.That(t, err, test.ShouldBeNil)

	_, err = board.FromRobot(r, "board0")
	test.That(t, err, test.ShouldNotBeNil)

	_, err = board.FromRobot(r, "g")
	test.That(t, err, test.ShouldNotBeNil)
}
