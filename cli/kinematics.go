package cli

import (
	"context"
	"fmt"
	"io"
	"math"
	"math/rand"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"
	commonpb "go.viam.com/api/common/v1"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const (
	// every GetKinematics answer carries the meshes, so past this the arm is slow to talk to
	kinematicsMeshBytesWarning = 8 << 20
	// the decimator refuses fewer triangles than a box
	kinematicsMinTriangles = 12
)

type kinematicsConvertArgs struct {
	InputFile   string
	OutputFile  string
	PackageRoot string
	Decimate    float64
}

// KinematicsConvertAction converts a URDF, SVA or DH file into a kinematic model file with its
// meshes written beside it, then reads the file back and checks its forward kinematics against
// the source, so a conversion that lost something fails here rather than on a robot.
func KinematicsConvertAction(ctx context.Context, cmd *cli.Command, args kinematicsConvertArgs) error {
	w := cmd.Root().Writer
	if strings.ToLower(filepath.Ext(args.OutputFile)) != ".json" {
		return fmt.Errorf("--%s must name a .json file, got %q", kinematicsFlagOutputFile, args.OutputFile)
	}
	if args.Decimate < 0 || args.Decimate > 1 || math.IsNaN(args.Decimate) {
		return fmt.Errorf("--%s must be in (0, 1], got %v", kinematicsFlagDecimate, args.Decimate)
	}

	model, err := readKinematicsInput(args.InputFile, args.PackageRoot, w)
	if err != nil {
		return err
	}
	pb, err := referenceframe.ModelToProto(model)
	if err != nil {
		return fmt.Errorf("%s cannot be written as a kinematic model: %w", args.InputFile, err)
	}
	if args.Decimate > 0 && args.Decimate < 1 {
		if err := decimateCollisionMeshes(pb, args.Decimate, w); err != nil {
			return err
		}
	}
	count, total := countMeshBytes(pb)
	if err := referenceframe.WriteKinematicModelFile(args.OutputFile, pb); err != nil {
		return err
	}
	if err := checkConvertedModel(model, args.OutputFile); err != nil {
		return err
	}
	printf(w, "Wrote %s with %d meshes, %s of mesh data", args.OutputFile, count, formatBytes(total))
	if total > kinematicsMeshBytesWarning {
		warningf(w, "%s of mesh data rides on every kinematics request for this model, consider --%s",
			formatBytes(total), kinematicsFlagDecimate)
	}
	return nil
}

func readKinematicsInput(inputFile, packageRoot string, w io.Writer) (referenceframe.Model, error) {
	switch strings.ToLower(filepath.Ext(inputFile)) {
	case ".urdf":
		model, skipped, err := referenceframe.ReadURDFModel(inputFile, packageRoot)
		if err != nil {
			return nil, err
		}
		for _, reason := range skipped {
			warningf(w, "skipped %s", reason)
		}
		return model, nil
	case ".json":
		if packageRoot != "" {
			return nil, fmt.Errorf("--%s applies to a URDF, %s is not one", kinematicsFlagPackageRoot, inputFile)
		}
		return referenceframe.ParseModelJSONFile(inputFile, "")
	default:
		return nil, fmt.Errorf("%s is not a .urdf or .json file", inputFile)
	}
}

// decimateCollisionMeshes replaces every collision mesh with a conservative hull of at most the
// given fraction of its triangles, written as PLY since that is the form the decimator produces.
func decimateCollisionMeshes(pb *commonpb.KinematicModel, ratio float64, w io.Writer) error {
	decimate := func(owner string, mesh *commonpb.Mesh) error {
		if mesh == nil {
			return nil
		}
		m, err := spatialmath.NewMeshFromProto(spatialmath.NewZeroPose(), mesh, "")
		if err != nil {
			return fmt.Errorf("%s: %w", owner, err)
		}
		before := len(m.Triangles())
		target := max(int(math.Ceil(ratio*float64(before))), kinematicsMinTriangles)
		if target >= before {
			return nil
		}
		decimated, err := m.ConservativeDecimate(target)
		if err != nil {
			return fmt.Errorf("%s: %w", owner, err)
		}
		mesh.Mesh = decimated.TrianglesToPLYBytes(false)
		mesh.ContentType = "ply"
		printf(w, "decimated %s from %d to %d triangles", owner, before, len(decimated.Triangles()))
		return nil
	}
	for _, link := range pb.GetLinks() {
		for i, g := range link.GetCollision() {
			if err := decimate(fmt.Sprintf("link %q collision %d", link.GetId(), i), g.GetMesh()); err != nil {
				return err
			}
		}
	}
	for _, joint := range pb.GetJoints() {
		if err := decimate(fmt.Sprintf("joint %q geometry", joint.GetId()), joint.GetGeometry().GetMesh()); err != nil {
			return err
		}
	}
	return nil
}

func countMeshBytes(pb *commonpb.KinematicModel) (count, total int) {
	add := func(mesh *commonpb.Mesh) {
		if len(mesh.GetMesh()) > 0 {
			count++
			total += len(mesh.GetMesh())
		}
	}
	for _, link := range pb.GetLinks() {
		for _, g := range link.GetCollision() {
			add(g.GetMesh())
		}
		for _, g := range link.GetVisual() {
			add(g.GetMesh())
		}
	}
	for _, joint := range pb.GetJoints() {
		add(joint.GetGeometry().GetMesh())
	}
	return count, total
}

// checkConvertedModel reads the file back the way a module will and compares forward kinematics
// with the source at random inputs.
func checkConvertedModel(source referenceframe.Model, outputPath string) error {
	pb, err := referenceframe.ReadKinematicModelFile(outputPath)
	if err != nil {
		return fmt.Errorf("the written file does not read back: %w", err)
	}
	converted, err := referenceframe.ModelFromProto(pb, source.Name())
	if err != nil {
		return fmt.Errorf("the written file does not read back as a model: %w", err)
	}
	if len(converted.DoF()) != len(source.DoF()) {
		return fmt.Errorf("the written model has %d degrees of freedom, the source has %d", len(converted.DoF()), len(source.DoF()))
	}
	//nolint:gosec
	r := rand.New(rand.NewSource(1))
	for range 10 {
		inputs := referenceframe.RandomFrameInputs(source, r)
		want, err := source.Transform(inputs)
		if err != nil {
			return err
		}
		got, err := converted.Transform(inputs)
		if err != nil {
			return err
		}
		if !spatialmath.PoseAlmostEqualEps(want, got, 1e-6) {
			return fmt.Errorf("the written model disagrees with the source at inputs %v: %v against %v", inputs, got, want)
		}
	}
	return nil
}

func formatBytes(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/float64(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(n)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
