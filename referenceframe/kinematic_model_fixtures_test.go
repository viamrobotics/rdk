package referenceframe

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// updateKinematicModelFixtures rewrites every expected.json from what the loader produces today.
// Run it after a deliberate change to the loader and read the diff, since the other SDKs compare
// against the same files.
var updateKinematicModelFixtures = flag.Bool("update-kinematic-model-fixtures", false,
	"rewrite testfiles/kinematic_models/*/expected.json from the loader's output")

const kinematicModelFixtureDir = "testfiles/kinematic_models"

// kinematicModelFixtureErrors maps the error kinds the fixtures name, which every SDK's runner
// has to recognise in its own words, to what the Go loader says. The README beside the fixtures
// is the list of kinds.
var kinematicModelFixtureErrors = map[string]string{
	"unknown_field":         "unknown field",
	"infinite":              "Inf",
	"nan":                   "NaN",
	"malformed_json":        "syntax error",
	"empty_model":           "no links or joints",
	"missing_id":            "has no id",
	"reserved_id":           "reserved word",
	"shared_id":             "names both a link and a joint",
	"duplicate_id":          "is defined twice",
	"missing_parent":        "is not a link or joint",
	"cycle":                 "its own ancestor",
	"mimic_self":            "mimics itself",
	"mimic_not_joint":       "which is not a joint",
	"mimic_with_limits":     ErrMimicWithLimits.Error(),
	"mesh_missing":          "failed to read mesh",
	"mesh_path_absolute":    "is absolute",
	"mesh_path_uri":         "is a URI",
	"mesh_path_backslash":   "uses backslashes",
	"content_type_mismatch": "disagrees",
	"collision_format":      "is not stl or ply",
	"mesh_no_extension":     "has no extension",
	"inline_without_type":   "need a content type",
	"mesh_empty":            "neither bytes nor a source path",
}

type kinematicModelFixtureError struct {
	Kind     string   `json:"kind"`
	Mentions []string `json:"mentions"`
}

// The fixtures are the contract every SDK's loader is held to. A case with an expected.json has
// to load into exactly that message, bytes and all, and a case with an error.json has to be
// refused for the reason it names, with the error naming what it mentions.
func TestKinematicModelFixtures(t *testing.T) {
	entries, err := os.ReadDir(kinematicModelFixtureDir)
	test.That(t, err, test.ShouldBeNil)
	cases := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		cases++
		dir := filepath.Join(kinematicModelFixtureDir, entry.Name())
		t.Run(entry.Name(), func(t *testing.T) {
			modelPath := filepath.Join(dir, "model.json")
			if raw, err := os.ReadFile(filepath.Join(dir, "error.json")); err == nil {
				var want kinematicModelFixtureError
				test.That(t, json.Unmarshal(raw, &want), test.ShouldBeNil)
				says, known := kinematicModelFixtureErrors[want.Kind]
				test.That(t, known, test.ShouldBeTrue)
				_, err := ReadKinematicModelFile(modelPath)
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, says)
				for _, mention := range want.Mentions {
					test.That(t, err.Error(), test.ShouldContainSubstring, mention)
				}
				// the generic entry point refuses it the same way
				_, err = KinematicModelFromFile(modelPath, "")
				test.That(t, err, test.ShouldNotBeNil)
				return
			}

			pb, err := ReadKinematicModelFile(modelPath)
			test.That(t, err, test.ShouldBeNil)
			expectedPath := filepath.Join(dir, "expected.json")
			if *updateKinematicModelFixtures {
				raw, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(pb)
				test.That(t, err, test.ShouldBeNil)
				var buf bytes.Buffer
				test.That(t, json.Indent(&buf, raw, "", "  "), test.ShouldBeNil)
				buf.WriteByte('\n')
				test.That(t, os.WriteFile(expectedPath, buf.Bytes(), 0o600), test.ShouldBeNil)
			}
			raw, err := os.ReadFile(expectedPath)
			test.That(t, err, test.ShouldBeNil)
			expected := &commonpb.KinematicModel{}
			test.That(t, protojson.Unmarshal(raw, expected), test.ShouldBeNil)
			test.That(t, proto.Equal(pb, expected), test.ShouldBeTrue)

			// and what loads has to become a model, since that is what a loader is for
			model, err := ModelFromProto(pb, "")
			test.That(t, err, test.ShouldBeNil)
			test.That(t, model.Name(), test.ShouldEqual, pb.GetName())
		})
	}
	test.That(t, cases, test.ShouldBeGreaterThan, 20)
}
