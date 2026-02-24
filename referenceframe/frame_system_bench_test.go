package referenceframe

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/utils"
	"go.viam.com/test"

	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// buildStaticBenchFS creates a frame system with a chain of static frames.
func buildStaticBenchFS(n int) (*FrameSystem, *LinearInputs) {
	fs := NewEmptyFrameSystem("bench")
	parent := fs.World()
	for i := 0; i < n; i++ {
		f := NewZeroStaticFrame(generateStringID(i))
		_ = fs.AddFrame(f, parent)
		parent = f
	}
	return fs, NewLinearInputs()
}

// buildRotationalBenchFS creates a frame system with a chain of rotational frames.
func buildRotationalBenchFS(n int) (*FrameSystem, *LinearInputs) {
	fs := NewEmptyFrameSystem("bench")
	li := NewLinearInputs()
	parent := fs.World()
	for i := 0; i < n; i++ {
		name := generateStringID(i)
		f, _ := NewRotationalFrame(name, spatial.R4AA{RX: 0, RY: 0, RZ: 1}, Limit{-3.14, 3.14})
		_ = fs.AddFrame(f, parent)
		li.Put(name, []Input{0.5})
		parent = f
	}
	return fs, li
}

// generateStringID returns a unique frame name for index i.
func generateStringID(i int) string {
	return string(rune('a'+i%26)) + string(rune('0'+i/26%10))
}

func BenchmarkTransformToDQ(b *testing.B) {
	b.Run("static_frames", func(b *testing.B) {
		fs, li := buildStaticBenchFS(8)
		frames := fs.FrameNames()
		leaf := frames[len(frames)-1]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = fs.TransformToDQ(li, leaf, World)
		}
	})

	b.Run("rotational_frames", func(b *testing.B) {
		fs, li := buildRotationalBenchFS(6)
		frames := fs.FrameNames()
		leaf := frames[len(frames)-1]
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = fs.TransformToDQ(li, leaf, World)
		}
	})

	b.Run("simple_model", func(b *testing.B) {
		m, err := ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "xarm6")
		if err != nil {
			b.Fatal(err)
		}
		fs := NewEmptyFrameSystem("bench")
		_ = fs.AddFrame(m, fs.World())
		li := NewLinearInputs()
		li.Put("xarm6", make([]Input, len(m.DoF())))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = fs.TransformToDQ(li, "xarm6", World)
		}
	})
}

func TestTransformToDQZeroAllocs(t *testing.T) {
	t.Run("static_frames", func(t *testing.T) {
		fs, li := buildStaticBenchFS(8)
		frames := fs.FrameNames()
		leaf := frames[len(frames)-1]
		allocs := testing.AllocsPerRun(20, func() {
			_, _ = fs.TransformToDQ(li, leaf, World)
		})
		test.That(t, allocs, test.ShouldEqual, 0)
	})

	t.Run("rotational_frames", func(t *testing.T) {
		fs, li := buildRotationalBenchFS(6)
		frames := fs.FrameNames()
		leaf := frames[len(frames)-1]
		allocs := testing.AllocsPerRun(20, func() {
			_, _ = fs.TransformToDQ(li, leaf, World)
		})
		test.That(t, allocs, test.ShouldEqual, 0)
	})

	t.Run("translational_frame", func(t *testing.T) {
		fs := NewEmptyFrameSystem("bench")
		f, err := NewTranslationalFrame("trans1", r3.Vector{X: 1}, Limit{-10, 10})
		test.That(t, err, test.ShouldBeNil)
		err = fs.AddFrame(f, fs.World())
		test.That(t, err, test.ShouldBeNil)
		li := NewLinearInputs()
		li.Put("trans1", []Input{2.0})
		allocs := testing.AllocsPerRun(20, func() {
			_, _ = fs.TransformToDQ(li, "trans1", World)
		})
		test.That(t, allocs, test.ShouldEqual, 0)
	})

	t.Run("simple_model", func(t *testing.T) {
		m, err := ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "xarm6")
		test.That(t, err, test.ShouldBeNil)
		fs := NewEmptyFrameSystem("bench")
		err = fs.AddFrame(m, fs.World())
		test.That(t, err, test.ShouldBeNil)
		li := NewLinearInputs()
		li.Put("xarm6", make([]Input, len(m.DoF())))
		allocs := testing.AllocsPerRun(20, func() {
			_, _ = fs.TransformToDQ(li, "xarm6", World)
		})
		test.That(t, allocs, test.ShouldEqual, 0)
	})
}
