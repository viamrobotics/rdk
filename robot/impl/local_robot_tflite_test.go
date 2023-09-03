//go:build !no_tflite
package robotimpl_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"math"
	"net"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	// registers all components.
	commonpb "go.viam.com/api/common/v1"
	armpb "go.viam.com/api/component/arm/v1"
	pb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/testutils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/arm/fake"
	"go.viam.com/rdk/components/audioinput"
	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/components/motor"
	fakemotor "go.viam.com/rdk/components/motor/fake"
	"go.viam.com/rdk/components/movementsensor"
	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/examples/customresources/apis/summationapi"
	rgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/robot/framesystem"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/robot/packages"
	putils "go.viam.com/rdk/robot/packages/testutils"
	"go.viam.com/rdk/robot/server"
	weboptions "go.viam.com/rdk/robot/web/options"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/datamanager/builtin"
	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/services/mlmodel/tflitecpu"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/navigation"
	_ "go.viam.com/rdk/services/register"
	"go.viam.com/rdk/services/sensors"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
	rtestutils "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/testutils/robottestutils"
	rutils "go.viam.com/rdk/utils"
)

func TestConfigPackageReferenceReplacement(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	fakePackageServer, err := putils.NewFakePackageServer(ctx, logger)
	test.That(t, err, test.ShouldBeNil)
	defer utils.UncheckedErrorFunc(fakePackageServer.Shutdown)

	packageDir := t.TempDir()
	labelPath := "${packages.orgID/some-name-2}/labels.txt"

	robotConfig := &config.Config{
		Packages: []config.PackageConfig{
			{
				Name:    "some-name-1",
				Package: "package-1",
				Version: "v1",
			},
			{
				Name:    "orgID/some-name-2",
				Package: "package-2",
				Version: "latest",
			},
			{
				Name:    "my-module",
				Package: "orgID/my-module",
				Type:    config.PackageTypeModule,
				Version: "1.2",
			},
			{
				Name:    "my-ml-model",
				Package: "orgID/my-ml-model",
				Type:    config.PackageTypeMlModel,
				Version: "latest",
			},
		},
		PackagePath: packageDir,
		Services: []resource.Config{
			{
				Name:  "ml-model-service",
				API:   mlmodel.API,
				Model: resource.DefaultModelFamily.WithModel("tflite_cpu"),
				ConvertedAttributes: &tflitecpu.TFLiteConfig{
					ModelPath:  "${packages.some-name-1}/model.tflite",
					LabelPath:  labelPath,
					NumThreads: 1,
				},
			},
			{
				Name:  "my-ml-model",
				API:   mlmodel.API,
				Model: resource.DefaultModelFamily.WithModel("tflite_cpu"),
				ConvertedAttributes: &tflitecpu.TFLiteConfig{
					ModelPath:  "${packages.ml_models.my-ml-model}/model.tflite",
					LabelPath:  labelPath,
					NumThreads: 2,
				},
			},
		},
		Modules: []config.Module{
			{
				Name:    "my-module",
				ExePath: "${packages.modules.my-module}/exec.sh",
			},
		},
	}

	fakePackageServer.StorePackage(robotConfig.Packages...)

	r, err := robotimpl.New(ctx, robotConfig, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
}
