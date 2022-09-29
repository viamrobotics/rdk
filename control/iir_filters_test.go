package control

import (
	"testing"

	"go.viam.com/test"
)

func TestIIRFilterButter(t *testing.T) {
	iirFlt := iirFilter{smpFreq: 10000, cutOffFreq: 1000, n: 4, ripple: 0.0, fltType: "lowpass"}

	test.That(t, iirFlt.n, test.ShouldEqual, 4)
	iirFlt.calculateABCoeffs()

	test.That(t, len(iirFlt.aCoeffs), test.ShouldEqual, 5)
	test.That(t, len(iirFlt.bCoeffs), test.ShouldEqual, 5)
	test.That(t, iirFlt.aCoeffs[0], test.ShouldAlmostEqual, 0.004824343357716)
	test.That(t, iirFlt.aCoeffs[1], test.ShouldAlmostEqual, 0.019297373430865)
	test.That(t, iirFlt.aCoeffs[2], test.ShouldAlmostEqual, 0.028946060146297)
	test.That(t, iirFlt.aCoeffs[3], test.ShouldAlmostEqual, 0.019297373430865)
	test.That(t, iirFlt.aCoeffs[4], test.ShouldAlmostEqual, 0.004824343357716)

	test.That(t, iirFlt.bCoeffs[1], test.ShouldAlmostEqual, -2.369513007182)
	test.That(t, iirFlt.bCoeffs[2], test.ShouldAlmostEqual, 2.313988414416)
	test.That(t, iirFlt.bCoeffs[3], test.ShouldAlmostEqual, -1.054665405879)
	test.That(t, iirFlt.bCoeffs[4], test.ShouldAlmostEqual, 0.187379492368)

	iirFlt = iirFilter{smpFreq: 10000, cutOffFreq: 2000, n: 2, ripple: 0.0, fltType: "lowpass"}
	iirFlt.calculateABCoeffs()
	test.That(t, len(iirFlt.aCoeffs), test.ShouldEqual, 3)
	test.That(t, len(iirFlt.bCoeffs), test.ShouldEqual, 3)
	test.That(t, iirFlt.aCoeffs[0], test.ShouldAlmostEqual, 0.206572083826148)
	test.That(t, iirFlt.aCoeffs[1], test.ShouldAlmostEqual, 0.413144167652296)
	test.That(t, iirFlt.aCoeffs[2], test.ShouldAlmostEqual, 0.206572083826148)
	test.That(t, iirFlt.bCoeffs[1], test.ShouldAlmostEqual, -0.369527377351)
	test.That(t, iirFlt.bCoeffs[2], test.ShouldAlmostEqual, 0.195815712656)

	iirFlt = iirFilter{smpFreq: 10000, cutOffFreq: 3000, n: 6, ripple: 0.0, fltType: "lowpass"}
	iirFlt.calculateABCoeffs()
	test.That(t, len(iirFlt.aCoeffs), test.ShouldEqual, 7)
	test.That(t, len(iirFlt.bCoeffs), test.ShouldEqual, 7)
	test.That(t, iirFlt.aCoeffs[0], test.ShouldAlmostEqual, 0.070115413492454)
	test.That(t, iirFlt.aCoeffs[1], test.ShouldAlmostEqual, 0.420692480954722)
	test.That(t, iirFlt.aCoeffs[2], test.ShouldAlmostEqual, 1.051731202386805)
	test.That(t, iirFlt.aCoeffs[3], test.ShouldAlmostEqual, 1.402308269849073)
	test.That(t, iirFlt.aCoeffs[4], test.ShouldAlmostEqual, 1.051731202386805)
	test.That(t, iirFlt.aCoeffs[5], test.ShouldAlmostEqual, 0.420692480954722)
	test.That(t, iirFlt.aCoeffs[6], test.ShouldAlmostEqual, 0.070115413492454)

	test.That(t, iirFlt.bCoeffs[1], test.ShouldAlmostEqual, 1.187600680176)
	test.That(t, iirFlt.bCoeffs[2], test.ShouldAlmostEqual, 1.305213349289)
	test.That(t, iirFlt.bCoeffs[3], test.ShouldAlmostEqual, 0.674327525298)
	test.That(t, iirFlt.bCoeffs[4], test.ShouldAlmostEqual, 0.263469348280)
	test.That(t, iirFlt.bCoeffs[5], test.ShouldAlmostEqual, 0.051753033880)
	test.That(t, iirFlt.bCoeffs[6], test.ShouldAlmostEqual, 0.005022526595)

	iirFlt = iirFilter{smpFreq: 10000, cutOffFreq: 3000, n: 6, ripple: 0.0, fltType: "highpass"}
	iirFlt.calculateABCoeffs()
	test.That(t, len(iirFlt.aCoeffs), test.ShouldEqual, 7)
	test.That(t, len(iirFlt.bCoeffs), test.ShouldEqual, 7)

	test.That(t, iirFlt.aCoeffs[0], test.ShouldAlmostEqual, 0.010312874762664)
	test.That(t, iirFlt.aCoeffs[1], test.ShouldAlmostEqual, -0.061877248575986)
	test.That(t, iirFlt.aCoeffs[2], test.ShouldAlmostEqual, 0.154693121439966)
	test.That(t, iirFlt.aCoeffs[3], test.ShouldAlmostEqual, -0.206257495253288)
	test.That(t, iirFlt.aCoeffs[4], test.ShouldAlmostEqual, 0.154693121439966)
	test.That(t, iirFlt.aCoeffs[5], test.ShouldAlmostEqual, -0.061877248575986)
	test.That(t, iirFlt.aCoeffs[6], test.ShouldAlmostEqual, 0.010312874762664)

	test.That(t, iirFlt.bCoeffs[1], test.ShouldAlmostEqual, 1.187600680176)
	test.That(t, iirFlt.bCoeffs[2], test.ShouldAlmostEqual, 1.305213349289)
	test.That(t, iirFlt.bCoeffs[3], test.ShouldAlmostEqual, 0.674327525298)
	test.That(t, iirFlt.bCoeffs[4], test.ShouldAlmostEqual, 0.263469348280)
	test.That(t, iirFlt.bCoeffs[5], test.ShouldAlmostEqual, 0.051753033880)
	test.That(t, iirFlt.bCoeffs[6], test.ShouldAlmostEqual, 0.005022526595)
}

func TestIIRFilterChebyshevI(t *testing.T) {
	iirFlt := iirFilter{smpFreq: 10000, cutOffFreq: 1000, n: 4, ripple: 0.5, fltType: "lowpass"}

	test.That(t, iirFlt.n, test.ShouldEqual, 4)
	iirFlt.calculateABCoeffs()

	test.That(t, len(iirFlt.aCoeffs), test.ShouldEqual, 5)
	test.That(t, len(iirFlt.bCoeffs), test.ShouldEqual, 5)

	test.That(t, iirFlt.bCoeffs[1], test.ShouldAlmostEqual, -2.7640305047044187)
	test.That(t, iirFlt.bCoeffs[2], test.ShouldAlmostEqual, 3.1228526783585413)
	test.That(t, iirFlt.bCoeffs[3], test.ShouldAlmostEqual, -1.6645530241054278)
	test.That(t, iirFlt.bCoeffs[4], test.ShouldAlmostEqual, 0.3502229603332013)

	test.That(t, iirFlt.aCoeffs[0], test.ShouldAlmostEqual, 0.00620090579054177)
	test.That(t, iirFlt.aCoeffs[1], test.ShouldAlmostEqual, 0.024803623162167082)
	test.That(t, iirFlt.aCoeffs[2], test.ShouldAlmostEqual, 0.03720543474325062)
	test.That(t, iirFlt.aCoeffs[3], test.ShouldAlmostEqual, 0.024803623162167082)
	test.That(t, iirFlt.aCoeffs[4], test.ShouldAlmostEqual, 0.00620090579054177)
}

func TestIIRFilterDesign(t *testing.T) {
	iirFlt, err := design(2000, 4250, 3.0, 30.0, 10000)
	test.That(t, err, test.ShouldBeNil)
	iirFlt.Reset()
	test.That(t, iirFlt.n, test.ShouldEqual, 2)
	test.That(t, iirFlt.cutOffFreq, test.ShouldAlmostEqual, 2001.7973929699663)

	test.That(t, len(iirFlt.aCoeffs), test.ShouldEqual, 3)
	test.That(t, len(iirFlt.bCoeffs), test.ShouldEqual, 3)
	test.That(t, iirFlt.bCoeffs[1], test.ShouldAlmostEqual, -0.3681885321516074)
	test.That(t, iirFlt.bCoeffs[2], test.ShouldAlmostEqual, 0.1956396086108966)

	test.That(t, iirFlt.aCoeffs[0], test.ShouldAlmostEqual, 0.2068627691148223)
	test.That(t, iirFlt.aCoeffs[1], test.ShouldAlmostEqual, 0.4137255382296446)
	test.That(t, iirFlt.aCoeffs[2], test.ShouldAlmostEqual, 0.2068627691148223)

	iirFlt, err = design(10, 40, 3.0, 30.0, 100)
	test.That(t, err, test.ShouldBeNil)
	iirFlt.Reset()
	test.That(t, iirFlt.n, test.ShouldEqual, 2)
	test.That(t, len(iirFlt.aCoeffs), test.ShouldEqual, 3)
	test.That(t, len(iirFlt.bCoeffs), test.ShouldEqual, 3)
	test.That(t, iirFlt.bCoeffs[1], test.ShouldAlmostEqual, -1.1420783035180786)
	test.That(t, iirFlt.bCoeffs[2], test.ShouldAlmostEqual, 0.4124032098038791)

	test.That(t, iirFlt.aCoeffs[0], test.ShouldAlmostEqual, 0.0675812265714501)
	test.That(t, iirFlt.aCoeffs[1], test.ShouldAlmostEqual, 0.1351624531429003)
	test.That(t, iirFlt.aCoeffs[2], test.ShouldAlmostEqual, 0.0675812265714501)
}
