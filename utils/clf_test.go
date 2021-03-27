package utils

import (
	"bufio"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAcesSplit(t *testing.T) {
	pcs := clfSplit("a [b] [c d] e [e f g]")
	assert.Equal(t, 5, len(pcs))
	assert.Equal(t, "a", pcs[0])
	assert.Equal(t, "[b]", pcs[1])
	assert.Equal(t, "[c d]", pcs[2])
	assert.Equal(t, "e", pcs[3])
	assert.Equal(t, "[e f g]", pcs[4])
}

func TestCLF(t *testing.T) {
	f, err := os.Open("data/aces_sample.clf")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	r := bufio.NewReader(f)

	clf := CLFReader{}

	numLines := 0
	err = clf.Process(r, func(data map[string]interface{}) error {
		numLines++
		v, ok := data["x"]
		if ok {
			_, ok := v.(float64)
			if !ok {
				t.Errorf("not a float64")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 21, numLines)

}
