package cli

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v3"
	"go.uber.org/multierr"
	apppb "go.viam.com/api/app/v1"
	"go.viam.com/utils"
)

const (
	visualizeFlagAppPath    = "app-path"
	visualizeFlagAppVersion = "app-version"
	visualizeFlagUpdate     = "update"
	// visualizerPathEnv names the installed desktop app when --app-path is not given.
	visualizerPathEnv = "VIAM_VISUALIZER_PATH"
	// launchFileVersion is the launch file contract the desktop app accepts.
	launchFileVersion = 1
)

type visualizeArgs struct {
	Organization string
	Location     string
	Machine      string
	Part         string
	AppPath      string
	AppVersion   string
	Update       bool
}

// errNoVisualizerPath says neither --app-path nor VIAM_VISUALIZER_PATH was given, so the CLI's own
// install cache is the next place to look.
var errNoVisualizerPath = errors.New("no visualizer path given")

// launchCredentials mirrors the credentials union the desktop app passes to the TypeScript SDK.
// Type is "access-token" (Payload only) or "api-key" (AuthEntity is the key id).
type launchCredentials struct {
	Type       string `json:"type"`
	AuthEntity string `json:"authEntity,omitempty"`
	Payload    string `json:"payload"`
}

// launchConfig is the version 1 launch file the desktop app reads from --launch-file.
type launchConfig struct {
	Version          int               `json:"version"`
	Host             string            `json:"host"`
	PartID           string            `json:"partId"`
	PartName         string            `json:"partName"`
	MachineName      string            `json:"machineName"`
	SignalingAddress string            `json:"signalingAddress"`
	ServiceHost      string            `json:"serviceHost"`
	Credentials      launchCredentials `json:"credentials"`
	ExpiresAt        string            `json:"expiresAt,omitempty"`
}

// visualizerBundleBinary is the executable inside a macOS .app bundle.
const visualizerBundleBinary = "viam-visualizer"

// buildLaunchConfig maps a resolved part and the CLI's stored auth to the launch file contents.
// It performs no I/O.
func buildLaunchConfig(part *apppb.RobotPart, machineName string, conf *Config) (launchConfig, error) {
	signaling := conf.BaseURL
	if signaling == "" {
		signaling = defaultBaseURL
	}
	serviceHost := signaling
	if u, err := url.Parse(signaling); err == nil && u.Host != "" {
		u.Host = u.Hostname()
		serviceHost = u.String()
	}

	cfg := launchConfig{
		Version:          launchFileVersion,
		Host:             part.Fqdn,
		PartID:           part.Id,
		PartName:         part.Name,
		MachineName:      machineName,
		SignalingAddress: signaling,
		ServiceHost:      serviceHost,
	}
	switch auth := conf.Auth.(type) {
	case nil:
		return launchConfig{}, errLoggedOut
	case *token:
		cfg.Credentials = launchCredentials{Type: "access-token", Payload: auth.AccessToken}
		cfg.ExpiresAt = auth.ExpiresAt.Format(time.RFC3339)
	case *apiKey:
		cfg.Credentials = launchCredentials{Type: "api-key", AuthEntity: auth.KeyID, Payload: auth.KeyCrypto}
	default:
		return launchConfig{}, errors.Errorf("unsupported auth type %T", conf.Auth)
	}
	return cfg, nil
}

// resolveVisualizerBinary picks the desktop app binary from the --app-path flag, then the
// VIAM_VISUALIZER_PATH environment variable, and returns errNoVisualizerPath when neither is set.
// A macOS .app bundle resolves to Contents/MacOS/<binary>. statFn is os.Stat in production.
func resolveVisualizerBinary(flagPath, envPath string, statFn func(string) (os.FileInfo, error)) (string, error) {
	path := flagPath
	if path == "" {
		path = envPath
	}
	if path == "" {
		return "", errNoVisualizerPath
	}
	info, err := statFn(path)
	if err != nil {
		return "", errors.Wrapf(err, "visualizer app not found at %q", path)
	}
	if info.IsDir() && strings.HasSuffix(path, ".app") {
		return filepath.Join(path, "Contents", "MacOS", visualizerBundleBinary), nil
	}
	return path, nil
}

// writeLaunchFile writes cfg as JSON to a new 0600 file in dir and returns its path.
func writeLaunchFile(dir string, cfg launchConfig) (string, error) {
	data, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, "viam-visualizer-*.json")
	if err != nil {
		return "", err
	}
	path := f.Name()
	if _, err := f.Write(data); err != nil {
		return "", multierr.Combine(err, f.Close(), os.Remove(path))
	}
	if err := f.Close(); err != nil {
		return "", multierr.Combine(err, os.Remove(path))
	}
	return path, nil
}

// runVisualizer starts binary with --launch-file path and waits for it. Ctrl+C cancels ctx and
// closes the app.
func runVisualizer(ctx context.Context, cmd *cli.Command, binary, launchFilePath string) error {
	// The app deletes the file on read, so a missing file here is the expected outcome.
	defer func() {
		if err := os.Remove(launchFilePath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			utils.UncheckedError(err)
		}
	}()

	//nolint:gosec
	child := exec.CommandContext(ctx, binary, "--launch-file", launchFilePath)
	child.Stdout = cmd.Root().Writer
	child.Stderr = cmd.Root().ErrWriter
	if err := child.Run(); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return errors.Wrapf(err, "running %s", binary)
	}
	return nil
}

// machinesPartVisualizeAction is the action for 'machines part visualize'.
func machinesPartVisualizeAction(ctx context.Context, cmd *cli.Command, args visualizeArgs) error {
	client, err := newViamClient(ctx, cmd)
	if err != nil {
		return err
	}
	return client.visualizePart(ctx, cmd, args)
}

func (c *viamClient) visualizePart(ctx context.Context, cmd *cli.Command, args visualizeArgs) error {
	if _, err := getGlobalArgs(cmd); err != nil {
		return err
	}

	// An explicit path wins and is checked before any network call. Otherwise the CLI's own cache
	// is used, downloading a release when it has none.
	binary, err := resolveVisualizerBinary(args.AppPath, os.Getenv(visualizerPathEnv), os.Stat)
	if errors.Is(err, errNoVisualizerPath) {
		binary, err = ensureVisualizer(ctx, defaultVisualizerInstallOptions(cmd.Root().ErrWriter, args.AppVersion, args.Update))
	}
	if err != nil {
		return err
	}

	if err := c.ensureLoggedIn(ctx); err != nil {
		return err
	}
	if err := c.selectOrganization(ctx, args.Organization); err != nil {
		return err
	}
	if err := c.selectLocation(ctx, args.Location); err != nil {
		return err
	}
	part, err := c.robotPart(ctx, c.selectedOrg.Id, c.selectedLoc.Id, args.Machine, args.Part)
	if err != nil {
		return err
	}
	if _, err := c.refreshAuthIfExpired(ctx); err != nil {
		return err
	}

	machineName := part.Robot
	if resp, err := c.client.GetRobot(ctx, &apppb.GetRobotRequest{Id: part.Robot}); err == nil && resp.GetRobot() != nil {
		machineName = resp.GetRobot().Name
	}

	cfg, err := buildLaunchConfig(part, machineName, c.conf)
	if err != nil {
		return err
	}
	path, err := writeLaunchFile(os.TempDir(), cfg)
	if err != nil {
		return err
	}

	printf(cmd.Root().Writer, "Launching visualizer for part %s on machine %s (%s)", part.Name, machineName, binary)
	return runVisualizer(ctx, cmd, binary, path)
}
