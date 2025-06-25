package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	v1 "go.viam.com/api/app/build/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"

	"go.viam.com/rdk/cli/module_generate/modulegen"
	modgen "go.viam.com/rdk/cli/module_generate/scripts"
	"go.viam.com/rdk/testutils/inject"
)

func TestGenerateModuleAction(t *testing.T) {
	testModule := modulegen.ModuleInputs{
		ModuleName:       "my-module",
		IsPublic:         false,
		Namespace:        "my-org",
		Language:         "python",
		Resource:         "arm component",
		ResourceType:     "component",
		ResourceSubtype:  "arm",
		ModelName:        "my-model",
		EnableCloudBuild: true,
		GeneratorVersion: "0.1.0",
		GeneratedOn:      time.Now().UTC(),

		ModulePascal:          "MyModule",
		ModuleLowercase:       "mymodule",
		API:                   "rdk:component:arm",
		ResourceSubtypePascal: "Arm",
		ModelPascal:           "MyModel",
		ModelSnake:            "my-model",
		ModelTriple:           "my-org:my-module:my-model",
		ModelReadmeLink:       "model-readme-link",

		SDKVersion: "0.0.0",
	}

	cCtx := newTestContext(t, map[string]any{"local": true})
	gArgs, _ := getGlobalArgs(cCtx)
	globalArgs := *gArgs

	testDir := t.TempDir()
	testChdir(t, testDir)
	modulePath := filepath.Join(testDir, testModule.ModuleName)

	t.Run("test setting up module directory", func(t *testing.T) {
		_, err := os.Stat(filepath.Join(testDir, testModule.ModuleName))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, setupDirectories(cCtx, testModule.ModuleName, globalArgs), test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(testDir, testModule.ModuleName))
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("test render common files", func(t *testing.T) {
		setupDirectories(cCtx, testModule.ModuleName, globalArgs)

		err := renderCommonFiles(cCtx, testModule, globalArgs)
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(modulePath, ".viam-gen-info"))
		test.That(t, err, test.ShouldBeNil)

		viamGenInfo, err := os.Open(filepath.Join(modulePath, ".viam-gen-info"))
		test.That(t, err, test.ShouldBeNil)
		defer viamGenInfo.Close()
		bytes, err := io.ReadAll(viamGenInfo)
		test.That(t, err, test.ShouldBeNil)
		var module modulegen.ModuleInputs
		err = json.Unmarshal(bytes, &module)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, module.ModuleName, test.ShouldEqual, testModule.ModuleName)

		_, err = os.Stat(filepath.Join(modulePath, "README.md"))
		test.That(t, err, test.ShouldBeNil)

		readme, err := os.Open(filepath.Join(modulePath, "README.md"))
		test.That(t, err, test.ShouldBeNil)
		defer readme.Close()
		bytes, err = io.ReadAll(readme)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(bytes), test.ShouldContainSubstring, "Module "+testModule.ModuleName)
		test.That(t, string(bytes), test.ShouldContainSubstring, "Model "+testModule.ModelTriple)

		// cloud build enabled
		_, err = os.Stat(filepath.Join(modulePath, ".github"))
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(modulePath, ".github", "workflows"))
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(modulePath, ".github", "workflows", "deploy.yml"))
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("test copy python template", func(t *testing.T) {
		setupDirectories(cCtx, testModule.ModuleName, globalArgs)
		err := copyLanguageTemplate(cCtx, "python", testModule.ModuleName, globalArgs)
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(modulePath, "src"))
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(modulePath, "build.sh"))
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(modulePath, "setup.sh"))
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(modulePath, "run.sh"))
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(modulePath, ".gitignore"))
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("test render template", func(t *testing.T) {
		setupDirectories(cCtx, testModule.ModuleName, globalArgs)
		_ = os.Mkdir(filepath.Join(modulePath, "src"), 0o755)
		_, err := os.Stat(filepath.Join(modulePath, "src"))
		test.That(t, err, test.ShouldBeNil)

		err = renderTemplate(cCtx, testModule, globalArgs)
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(modulePath, "requirements.txt"))
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(modulePath, "src", "main.py"))
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("test generate stubs", func(t *testing.T) {
		setupDirectories(cCtx, testModule.ModuleName, globalArgs)
		_ = os.Mkdir(filepath.Join(modulePath, "src"), 0o755)
		_, err := os.Stat(filepath.Join(modulePath, "src"))
		test.That(t, err, test.ShouldBeNil)

		err = generateStubs(cCtx, testModule, globalArgs)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("test generate go stubs", func(t *testing.T) {
		testModule.Language = "go"
		testModule.SDKVersion = "0.44.0"
		setupDirectories(cCtx, testModule.ModuleName, globalArgs)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientCode := `
// Package arm contains a gRPC based arm client.
package arm

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/logging"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// client implements ArmServiceClient.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	name   string
	client pb.ArmServiceClient
	model  referenceframe.Model
	logger logging.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger logging.Logger,
) (Arm, error) {
	pbClient := pb.NewArmServiceClient(conn)
	c := &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.ShortName(),
		client: pbClient,
		logger: logger,
	}
	clientFrame, err := c.updateKinematics(ctx, nil)
	if err != nil {
		logger.CWarnw(
			ctx,
			"error getting model for arm; making the assumption that joints are revolute and that their positions are specified in degrees",
			"err",
			err,
		)
	} else {
		c.model = clientFrame
	}
	return c, nil
}

func (c *client) EndPosition(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetEndPosition(ctx, &pb.GetEndPositionRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return nil, err
	}
	return spatialmath.NewPoseFromProtobuf(resp.Pose), nil
}

func (c *client) MoveToPosition(ctx context.Context, pose spatialmath.Pose, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	if pose == nil {
		c.logger.Warnf("%s MoveToPosition: pose parameter is nil", c.name)
	}
	_, err = c.client.MoveToPosition(ctx, &pb.MoveToPositionRequest{
		Name:  c.name,
		To:    spatialmath.PoseToProtobuf(pose),
		Extra: ext,
	})
	return err
}

func (c *client) MoveToJointPositions(ctx context.Context, positions []referenceframe.Input, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	jp, err := referenceframe.JointPositionsFromInputs(c.model, positions)
	if err != nil {
		return err
	}
	_, err = c.client.MoveToJointPositions(ctx, &pb.MoveToJointPositionsRequest{
		Name:      c.name,
		Positions: jp,
		Extra:     ext,
	})
	return err
}

func (c *client) MoveThroughJointPositions(
	ctx context.Context,
	positions [][]referenceframe.Input,
	options *MoveOptions,
	extra map[string]interface{},
) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	if positions == nil {
		c.logger.Warnf("%s MoveThroughJointPositions: position argument is nil", c.name)
	}
	allJPs := make([]*pb.JointPositions, 0, len(positions))
	for _, position := range positions {
		jp, err := referenceframe.JointPositionsFromInputs(c.model, position)
		if err != nil {
			return err
		}
		allJPs = append(allJPs, jp)
	}
	req := &pb.MoveThroughJointPositionsRequest{
		Name:      c.name,
		Positions: allJPs,
		Extra:     ext,
	}
	if options != nil {
		req.Options = options.toProtobuf()
	}
	_, err = c.client.MoveThroughJointPositions(ctx, req)
	return err
}

func (c *client) JointPositions(ctx context.Context, extra map[string]interface{}) ([]referenceframe.Input, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetJointPositions(ctx, &pb.GetJointPositionsRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return nil, err
	}
	return referenceframe.InputsFromJointPositions(c.model, resp.Positions)
}

func (c *client) Stop(ctx context.Context, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.Stop(ctx, &pb.StopRequest{
		Name:  c.name,
		Extra: ext,
	})
	return err
}

func (c *client) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	return c.JointPositions(ctx, nil)
}

func (c *client) GoToInputs(ctx context.Context, inputSteps ...[]referenceframe.Input) error {
	return c.MoveThroughJointPositions(ctx, inputSteps, nil, nil)
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return rprotoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}

func (c *client) IsMoving(ctx context.Context) (bool, error) {
	resp, err := c.client.IsMoving(ctx, &pb.IsMovingRequest{Name: c.name})
	if err != nil {
		return false, err
	}
	return resp.IsMoving, nil
}

func (c *client) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetGeometries(ctx, &commonpb.GetGeometriesRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return nil, err
	}
	return spatialmath.NewGeometriesFromProto(resp.GetGeometries())
}

func (c *client) updateKinematics(ctx context.Context, extra map[string]interface{}) (referenceframe.Model, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetKinematics(ctx, &commonpb.GetKinematicsRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return nil, err
	}

	return parseKinematicsResponse(c.name, resp)
}
`
			w.Write([]byte(clientCode))
		}))
		defer server.Close()

		modgen.CreateGetClientCodeRequest = func(module modulegen.ModuleInputs) (*http.Request, error) {
			return http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
		}

		err := generateGolangStubs(testModule)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("test generate python stubs", func(t *testing.T) {
		testModule.Language = "python"
		setupDirectories(cCtx, testModule.ModuleName, globalArgs)
		_ = os.Mkdir(filepath.Join(modulePath, "src"), 0o755)
		_, err := os.Stat(filepath.Join(modulePath, "src"))
		test.That(t, err, test.ShouldBeNil)

		err = generatePythonStubs(testModule)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("test create module and manifest", func(t *testing.T) {
		cCtx, ac, _, _ := setup(&inject.AppServiceClient{}, nil, &inject.BuildServiceClient{
			StartBuildFunc: func(ctx context.Context, in *v1.StartBuildRequest, opts ...grpc.CallOption) (*v1.StartBuildResponse, error) {
				return &v1.StartBuildResponse{BuildId: "xyz123"}, nil
			},
		}, map[string]any{}, "token")
		err := createModuleAndManifest(cCtx, ac, testModule, globalArgs)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("test render manifest", func(t *testing.T) {
		setupDirectories(cCtx, testModule.ModuleName, globalArgs)
		err := renderManifest(cCtx, "moduleId", testModule, globalArgs)
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(testDir, testModule.ModuleName, "meta.json"))
		test.That(t, err, test.ShouldBeNil)

		manifestFile, err := os.Open(filepath.Join(testDir, testModule.ModuleName, "meta.json"))
		test.That(t, err, test.ShouldBeNil)
		defer manifestFile.Close()
		bytes, err := io.ReadAll(manifestFile)
		test.That(t, err, test.ShouldBeNil)
		var manifest moduleManifest
		err = json.Unmarshal(bytes, &manifest)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(manifest.Models), test.ShouldEqual, 1)
		test.That(t, manifest.Models[0].Model, test.ShouldEqual, testModule.ModelTriple)
		test.That(t, *manifest.Models[0].MarkdownLink, test.ShouldEqual, testModule.ModelReadmeLink)
	})
}
