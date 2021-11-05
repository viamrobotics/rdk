package slipdetection

import (
	"sync"
	"testing"

	"go.viam.com/test"
)

type mockReadingsHistoryProviderAlwaysSlipping struct{}

func (mockRHP *mockReadingsHistoryProviderAlwaysSlipping) GetPreviousMatrices() [][][]int {
	result := make([][][]int, 0)
	expectedMatrix1 := [][]int{
		{62, 62, 0},
		{0, 0, 0},
	}
	expectedMatrix11 := [][]int{
		{61, 61, 0},
		{0, 61, 0},
	}
	expectedMatrix2 := [][]int{
		{0, 1, 1},
		{0, 1, 1},
	}
	expectedMatrix21 := [][]int{
		{0, 0, 0},
		{0, 0, 0},
	}
	result = append(result, expectedMatrix1)
	result = append(result, expectedMatrix11)
	result = append(result, expectedMatrix2)
	result = append(result, expectedMatrix21)
	return result
}

type mockReadingsHistoryProviderNeverSlipping struct{}

func (mockRHP *mockReadingsHistoryProviderNeverSlipping) GetPreviousMatrices() [][][]int {
	result := make([][][]int, 0)
	expectedMatrix1 := [][]int{
		{2, 2, 0},
		{0, 0, 0},
	}
	expectedMatrix11 := [][]int{
		{2, 2, 0},
		{0, 0, 0},
	}
	expectedMatrix2 := [][]int{
		{2, 2, 0},
		{0, 0, 0},
	}
	expectedMatrix21 := [][]int{
		{2, 2, 0},
		{0, 0, 0},
	}
	result = append(result, expectedMatrix1)
	result = append(result, expectedMatrix11)
	result = append(result, expectedMatrix2)
	result = append(result, expectedMatrix21)
	return result
}

func TestDetectSlip(t *testing.T) {
	slipDetector := mockReadingsHistoryProviderNeverSlipping{}

	// bad version test
	_, err := DetectSlip(&slipDetector, &sync.Mutex{}, 1, 40.0, 2)
	test.That(t, err, test.ShouldNotBeNil)

	// DetectSlipV0: asking for more frames than exist yet should return false
	isSlipping, err := DetectSlip(&slipDetector, &sync.Mutex{}, 0, 40.0, 5)
	test.That(t, isSlipping, test.ShouldBeFalse)
	test.That(t, err, test.ShouldBeNil)

	// DetectSlipV0: no slip case
	isSlipping, err = DetectSlip(&slipDetector, &sync.Mutex{}, 0, 40.0, 2)
	test.That(t, isSlipping, test.ShouldBeFalse)
	test.That(t, err, test.ShouldBeNil)

	alwaysSlippingDetector := mockReadingsHistoryProviderAlwaysSlipping{}

	isSlipping, err = DetectSlip(&alwaysSlippingDetector, &sync.Mutex{}, 0, 40.0, 4)
	test.That(t, isSlipping, test.ShouldBeTrue)
	test.That(t, err, test.ShouldBeNil)
}
