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
