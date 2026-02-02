package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/fullstorydev/grpcurl"
	"github.com/google/uuid"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/nathan-fiscaletti/consolesize-go"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	buildpb "go.viam.com/api/app/build/v1"
	datapb "go.viam.com/api/app/data/v1"
	datapipelinespb "go.viam.com/api/app/datapipelines/v1"
	datasetpb "go.viam.com/api/app/dataset/v1"
	mlinferencepb "go.viam.com/api/app/mlinference/v1"
	mltrainingpb "go.viam.com/api/app/mltraining/v1"
	packagepb "go.viam.com/api/app/packages/v1"
	apppb "go.viam.com/api/app/v1"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	reflectpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	"go.viam.com/rdk/app"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/utils/contextutils"
)

// ... existing code ...

type organizationsFirebaseConfigUploadArgs struct {
	OrgID              string `flag:"org-id"`
	BundleID           string `flag:"bundle-id"`
	FirebaseConfigPath string `flag:"firebase-config-path"`
}

// OrganizationsFirebaseConfigUploadAction corresponds to `organizations firebase-config upload`.
func OrganizationsFirebaseConfigUploadAction(cCtx *cli.Context, args organizationsFirebaseConfigUploadArgs) error {
	return c.organizationsFirebaseConfigUploadAction(cCtx, args.OrgID, args.BundleID, args.FirebaseConfigPath)
}

func (c *viamClient) organizationsFirebaseConfigUploadAction(cCtx *cli.Context, orgID, bundleID, configPath string) error {
	bytes, err := os.ReadFile(configPath)
	if err != nil {
		return errors.Wrap(err, "failed to read firebase config file")
	}

	// Validate JSON
	var js map[string]interface{}
	if err := json.Unmarshal(bytes, &js); err != nil {
		return errors.Wrap(err, "invalid json in firebase config file")
	}

	_, err = c.client.UploadFirebaseConfig(c.c.Context, &apppb.UploadFirebaseConfigRequest{
		OrgId:      orgID,
		BundleId:   bundleID,
		ConfigJson: string(bytes),
	})
	if err != nil {
		return errors.Wrap(err, "failed to upload firebase config")
	}

	fmt.Println("Firebase config uploaded successfully")
	return nil
}
