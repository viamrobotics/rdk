package calibrate

import (
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"
)

func TestNewCorner(t *testing.T) {
	expected := Corner{X: 100.0, Y: 200.0, R: 0}
	got := NewCorner(100, 200)
	test.That(t, got, test.ShouldResemble, expected)
}

func TestNormalizeCorners(t *testing.T) {
	C := []Corner{
		{X: 100, Y: 50},
		{X: 200, Y: 50},
		{X: 100, Y: 150},
		{X: 200, Y: 150},
	}
	expected := []Corner{
		{X: -1, Y: -1, R: 0},
		{X: 1, Y: -1, R: 0},
		{X: -1, Y: 1, R: 0},
		{X: 1, Y: 1, R: 0},
	}
	got := NormalizeCorners(C)
	test.That(t, got, test.ShouldResemble, expected)

	got = NormalizeCorners(nil)
	test.That(t, got, test.ShouldBeNil)

	C = []Corner{{X: 87, Y: 603, R: 3.14159}}
	got = NormalizeCorners(C)
	test.That(t, got, test.ShouldResemble, C)
}

func TestSortCornerListByX(t *testing.T) {
	C := []Corner{
		{X: 300, Y: 50},
		{X: 220, Y: 60},
		{X: 100, Y: 100},
		{X: 200, Y: 151},
	}
	expected := []Corner{
		{X: 100, Y: 100},
		{X: 200, Y: 151},
		{X: 220, Y: 60},
		{X: 300, Y: 50},
	}
	got := SortCornerListByX(C)
	test.That(t, got, test.ShouldResemble, expected)
}

func TestSortCornerListByY(t *testing.T) {
	C := []Corner{
		{X: 300, Y: 50},
		{X: 220, Y: 60},
		{X: 100, Y: 100},
		{X: 200, Y: 151},
	}
	expected := []Corner{
		{X: 300, Y: 50},
		{X: 220, Y: 60},
		{X: 100, Y: 100},
		{X: 200, Y: 151},
	}
	got := SortCornerListByY(C)
	test.That(t, got, test.ShouldResemble, expected)
}

func TestGetAndShowCorners(t *testing.T) {
	c, err := GetAndShowCorners(artifact.MustPath("calibrate/chess3.jpeg"), artifact.MustPath("calibrate/chessOUT3nums.jpeg"), 50)
	got := SortCornerListByX(c)

	expected := []Corner{
		{535, 460, 6.88028101376e+11},
		{562, 320, 4.7888680633344e+11},
		{606, 367, 5.51086485236e+11},
		{614, 263, 4.6939313582064e+11},
		{635, 335, 5.9875863149916e+11},
		{656, 217, 5.15709383904e+11},
		{660, 296, 7.634555136e+11},
		{664, 425, 7.7524594557696e+11},
		{673, 199, 4.7520395878676e+11},
		{681, 273, 4.59400385719e+11},
		{691, 384, 4.77020342336e+11},
		{700, 244, 7.3645397377024e+11},
		{713, 341, 7.80777295551e+11},
		{734, 313, 5.1665809984704e+11},
		{735, 499, 8.345681096575599e+11},
		{750, 277, 7.2543196471616e+11},
		{759, 448, 6.613033718905599e+11},
		{767, 257, 5.1670236000636e+11},
		{778, 393, 8.7129044248864e+11},
		{779, 229, 5.3786925773136e+11},
		{798, 357, 5.4987880445504e+11},
		{809, 317, 7.7914301297976e+11},
		{825, 292, 7.4905354475264e+11},
		{834, 258, 8.19482231312e+11},
		{845, 240, 5.4757764236736e+11},
		{860, 459, 8.0012800512e+11},
		{876, 414, 6.76531328e+11},
		{883, 364, 6.9895097716736e+11},
		{895, 332, 6.8979451032576e+11},
		{899, 295, 6.5900135528136e+11},
		{909, 271, 5.6986944685e+11},
		{913, 240, 7.8899551986304e+11},
		{968, 547, 4.7041328218176e+11},
		{973, 423, 8.70191590205e+11},
		{976, 485, 7.71630076224e+11},
		{979, 339, 5.68414672805e+11},
		{981, 383, 7.26548140125e+11},
		{982, 274, 7.543573644665599e+11},
		{985, 309, 7.2444105404244e+11},
		{989, 252, 4.8854010691584e+11},
		{1058, 253, 1.00118549470464e+12},
		{1066, 313, 8.69776912116e+11},
		{1068, 287, 7.3569422029824e+11},
		{1075, 392, 9.1285808328e+11},
		{1077, 356, 5.08856898816e+11},
		{1089, 500, 4.9767433502976e+11},
		{1145, 290, 8.6269347588816e+11},
		{1166, 363, 6.19502594256e+11},
		{1194, 456, 4.98050232896e+11},
		{1240, 590, 4.5866218616064e+11},
	}

	test.That(t, got, test.ShouldResemble, expected)
	test.That(t, err, test.ShouldBeNil)
}
