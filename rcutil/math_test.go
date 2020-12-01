package rcutil

import (
	"testing"
)

func TestAbs1(t *testing.T) {
	if 5 != AbsInt(5) {
		t.Errorf("wtf")
	}

	if 5 != AbsInt(-5) {
		t.Errorf("wtf")
	}

	if 0 != AbsInt(0) {
		t.Errorf("wtf")
	}

}

func TestSquare1(t *testing.T) {
	if 4.0 != Square(2.0) {
		t.Errorf("eliot can't do math")
	}

	if 4 != SquareInt(2) {
		t.Errorf("eliot can't do math")
	}

}
