package boat

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

var testMotorConfig = []motorConfig{
	{Name: "starboard-rotation", XOffset: 0.3, YOffset: 0, AngleDegrees: 0, Weight: 1},
	{Name: "port-rotation", XOffset: -0.3, YOffset: 0, AngleDegrees: 0, Weight: 1},
	{Name: "forward", XOffset: 0, YOffset: -0.3, AngleDegrees: 0, Weight: 1},
	{Name: "reverse", XOffset: 0, YOffset: 0.3, AngleDegrees: 180, Weight: 1},
	{Name: "starboard-lateral", XOffset: 0.45, YOffset: 0, AngleDegrees: 90, Weight: 1},
	{Name: "port-lateral", XOffset: -0.45, YOffset: 0, AngleDegrees: -90, Weight: 1},
}

func TestBoatConfig(t *testing.T) {
	cfg := boatConfig{
		Motors: testMotorConfig,
		Length: .5,
		Width:  .5,
	}

	max := cfg.maxWeights()
	test.That(t, max.linearY, test.ShouldAlmostEqual, 4, testTheta)
	test.That(t, max.linearX, test.ShouldAlmostEqual, 2, testTheta)
	test.That(t, max.angular, test.ShouldAlmostEqual, .845, testTheta) // TODO(erh): is this right?

	g := cfg.computeGoal(r3.Vector{0, 1, 0}, r3.Vector{})
	test.That(t, g.linearY, test.ShouldAlmostEqual, 4)

	g = cfg.computeGoal(r3.Vector{1, 1, 0}, r3.Vector{})
	test.That(t, g.linearX, test.ShouldAlmostEqual, 2)
	test.That(t, g.linearY, test.ShouldAlmostEqual, 2)

	g = cfg.computeGoal(r3.Vector{.2, 1, 0}, r3.Vector{})
	test.That(t, g.linearX, test.ShouldAlmostEqual, .4)
	test.That(t, g.linearY, test.ShouldAlmostEqual, 2)

	powers := cfg.computePower(r3.Vector{0, 1, 0}, r3.Vector{})
	test.That(t, powers[0], test.ShouldAlmostEqual, 1, testTheta)
	test.That(t, powers[1], test.ShouldAlmostEqual, 1, testTheta)
	test.That(t, powers[2], test.ShouldAlmostEqual, 1, testTheta)
	test.That(t, powers[3], test.ShouldAlmostEqual, -1, testTheta)
	test.That(t, powers[4], test.ShouldAlmostEqual, 0, testTheta)
	test.That(t, powers[5], test.ShouldAlmostEqual, 0, testTheta)

	powers = cfg.computePower(r3.Vector{0, -1, 0}, r3.Vector{})
	test.That(t, powers[0], test.ShouldAlmostEqual, -1, testTheta)
	test.That(t, powers[1], test.ShouldAlmostEqual, -1, testTheta)
	test.That(t, powers[2], test.ShouldAlmostEqual, -1, testTheta)
	test.That(t, powers[3], test.ShouldAlmostEqual, 1, testTheta)
	test.That(t, powers[4], test.ShouldAlmostEqual, 0, testTheta)
	test.That(t, powers[5], test.ShouldAlmostEqual, 0, testTheta)

	powers = cfg.computePower(r3.Vector{0, 0, 0}, r3.Vector{Z: 1})
	test.That(t, powers[0], test.ShouldAlmostEqual, 1, testTheta)
	test.That(t, powers[1], test.ShouldAlmostEqual, -1, testTheta)
	test.That(t, powers[2], test.ShouldAlmostEqual, 0, testTheta)
	test.That(t, powers[3], test.ShouldAlmostEqual, 0, testTheta)
	test.That(t, powers[4], test.ShouldAlmostEqual, 0, testTheta)
	test.That(t, powers[5], test.ShouldAlmostEqual, 0, testTheta)

	powers = cfg.computePower(r3.Vector{0, 0, 0}, r3.Vector{Z: -1})
	test.That(t, powers[0], test.ShouldAlmostEqual, -1, testTheta)
	test.That(t, powers[1], test.ShouldAlmostEqual, 1, testTheta)
	test.That(t, powers[2], test.ShouldAlmostEqual, 0, testTheta)
	test.That(t, powers[3], test.ShouldAlmostEqual, 0, testTheta)
	test.That(t, powers[4], test.ShouldAlmostEqual, 0, testTheta)
	test.That(t, powers[5], test.ShouldAlmostEqual, 0, testTheta)

	t.Run("matrix-base", func(t *testing.T) {
		m := cfg.computePowerOutputAsMatrix([]float64{0, 0, 0, 0, 0, 0})
		r, c := m.Dims()
		test.That(t, 3, test.ShouldEqual, r)
		test.That(t, 1, test.ShouldEqual, c)

		for idx, w := range cfg.weights() {
			powers := make([]float64, 6)
			powers[idx] = 1
			out := cfg.computePowerOutput(powers)
			test.That(t, w, test.ShouldResemble, out)
		}
	})

	l, a := r3.Vector{1, 0, 0}, r3.Vector{}
	powers = cfg.computePower(l, a)
	test.That(t, cfg.computePowerOutput(powers), weightsAlmostEqual, cfg.computeGoal(l, a))

	l, a = r3.Vector{0, 1, 0}, r3.Vector{}
	powers = cfg.computePower(l, a)
	test.That(t, cfg.computePowerOutput(powers), weightsAlmostEqual, cfg.computeGoal(l, a))

	l, a = r3.Vector{-.5, 1, 0}, r3.Vector{}
	powers = cfg.computePower(l, a)
	test.That(t, cfg.computePowerOutput(powers), weightsAlmostEqual, cfg.computeGoal(l, a))

	l, a = r3.Vector{}, r3.Vector{Z: .125}
	powers = cfg.computePower(l, a)
	test.That(t, cfg.computePowerOutput(powers), weightsAlmostEqual, cfg.computeGoal(l, a))
}

func weightsAlmostEqual(actual interface{}, expected ...interface{}) string {
	a := actual.(motorWeights)
	e := expected[0].(motorWeights)

	if s := test.ShouldAlmostEqual(a.linearX, e.linearX, testTheta); s != "" {
		return "x: " + s
	}

	if s := test.ShouldAlmostEqual(a.linearY, e.linearY, testTheta); s != "" {
		return "y: " + s
	}

	if s := test.ShouldAlmostEqual(a.angular, e.angular, testTheta); s != "" {
		return "angular: " + s
	}

	return ""
}

func BenchmarkComputePower(b *testing.B) {
	cfg := boatConfig{
		Motors: testMotorConfig,
		Length: .5,
		Width:  .5,
	}

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		cfg.computePower(r3.Vector{0, 1, 0}, r3.Vector{})
	}
}
