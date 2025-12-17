package referenceframe

import (
	"testing"

	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/test"

	spatial "go.viam.com/rdk/spatialmath"
)

type trivialFrame struct{}

func (tF *trivialFrame) Name() string {
	return ""
}

func (tF *trivialFrame) Hash() int {
	return 123
}

func (tF *trivialFrame) Transform([]Input) (spatial.Pose, error) {
	return nil, nil
}

func (tF *trivialFrame) Interpolate([]Input, []Input, float64) ([]Input, error) {
	return nil, nil
}

func (tF *trivialFrame) Geometries([]Input) (*GeometriesInFrame, error) {
	return nil, nil
}

func (tF *trivialFrame) InputFromProtobuf(*pb.JointPositions) []Input {
	return nil
}

func (tF *trivialFrame) ProtobufFromInput([]Input) *pb.JointPositions {
	return nil
}

func (tF *trivialFrame) MarshalJSON() ([]byte, error) {
	return []byte{}, nil
}

func (tF *trivialFrame) UnmarshalJSON([]byte) error {
	return nil
}

func (tF *trivialFrame) DoF() []Limit {
	return nil
}

func TestImplementerRegistration(t *testing.T) {
	type staticFrame struct {
		*trivialFrame
	}
	// test that we get an error trying to register an already registered frame type
	err := RegisterFrameImplementer((*staticFrame)(nil), "static")
	test.That(t, err, test.ShouldNotBeNil)

	// test that we can successfully register a Frame implementation
	err = RegisterFrameImplementer((*trivialFrame)(nil), "trivial")
	test.That(t, err, test.ShouldBeNil)
}
