package referenceframe

import (
	"go.viam.com/test"
	"testing"

	"gonum.org/v1/gonum/num/dualquat"
)

/*
Create a test that successfully transforms the pose of *object from *frame1 into *frame2. The Orientation of *frame1 and *frame2
are the same, so the transformation is only made up of two translations.

|              |
|*frame1       |*object
|              |
|
|
|              *frame2
|________________
world
*/

func TestFrameTranslation(t *testing.T) {
	return nil
}
