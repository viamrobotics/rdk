package cli

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
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

const (
	// visualizerBundleBinary is the executable inside a macOS .app bundle. electron-builder names it
	// after the product, the same as the bundle.
	visualizerBundleBinary = "Viam Visualizer"
	// visualizerCachedBinary is the file name stem the install cache uses on linux and windows.
	visualizerCachedBinary = "viam-visualizer"
	// visualizerNoAutoUpdateEnv stops the app from updating itself, so a cached version directory
	// keeps holding the version it is named for.
	visualizerNoAutoUpdateEnv = "VIAM_VISUALIZER_NO_AUTO_UPDATE"
	// visualizerMinTokenLifetime is how long a user token must have left when it is handed to the
	// app. The launch file carries no refresh token, so a shorter one is refreshed first.
	visualizerMinTokenLifetime = 5 * time.Minute
)

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

// visualizerEnv is the app's environment. A binary from the CLI's install cache gets
// VIAM_VISUALIZER_NO_AUTO_UPDATE so it never overwrites itself; nil means inherit the CLI's.
func visualizerEnv(cached bool) []string {
	if !cached {
		return nil
	}
	return append(os.Environ(), visualizerNoAutoUpdateEnv+"=1")
}

// runVisualizer starts binary with --launch-file path and waits for it. ctx must already be
// cancelled by Ctrl+C, which closes the app. cached says the binary came from the CLI's install
// cache.
func runVisualizer(ctx context.Context, cmd *cli.Command, binary, launchFilePath string, cached bool) error {
	// The app deletes the file on read, so a missing file here is the expected outcome.
	defer func() {
		if err := os.Remove(launchFilePath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			utils.UncheckedError(err)
		}
	}()

	//nolint:gosec
	child := exec.CommandContext(ctx, binary, "--launch-file", launchFilePath)
	child.Env = visualizerEnv(cached)
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

	binary, err := resolveVisualizerBinary(args.AppPath, os.Getenv(visualizerPathEnv), os.Stat)
	cached := errors.Is(err, errNoVisualizerPath)
	if err != nil && !cached {
		return err
	}
	if cached && args.AppVersion != "" {
		if args.AppVersion, err = normalizeVisualizerVersion(args.AppVersion); err != nil {
			return err
		}
	}

	if err := c.ensureLoggedIn(ctx); err != nil {
		return err
	}
	part, err := c.robotPart(ctx, args.Organization, args.Location, args.Machine, args.Part)
	if err != nil {
		return err
	}
	if cached {
		opts := defaultVisualizerInstallOptions(cmd.Root().ErrWriter, args.AppVersion, args.Update)
		if binary, err = ensureVisualizer(ctx, opts); err != nil {
			return err
		}
	}
	// Refreshed after the install, which can take a while, so the app gets the freshest token.
	if err := c.refreshAuthForLaunch(ctx); err != nil {
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
	// Without this, Ctrl+C kills the CLI before runVisualizer can delete the credentials file.
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	path, err := writeLaunchFile(os.TempDir(), cfg)
	if err != nil {
		return err
	}

	printf(cmd.Root().Writer, "Launching visualizer for part %s on machine %s (%s)", part.Name, machineName, binary)
	return runVisualizer(ctx, cmd, binary, path, cached)
}

// refreshAuthForLaunch refreshes a user token that has less than visualizerMinTokenLifetime left,
// beyond the CLI's usual just-expired check, and persists it. API keys are left as they are.
func (c *viamClient) refreshAuthForLaunch(ctx context.Context) error {
	if _, err := c.refreshAuthIfExpired(ctx); err != nil {
		return err
	}
	tok, ok := c.conf.Auth.(*token)
	if !ok || time.Until(tok.ExpiresAt) >= visualizerMinTokenLifetime || !tok.canRefresh() {
		return nil
	}
	refreshed, err := c.authFlow.refreshToken(ctx, tok)
	if err != nil {
		return errors.Wrap(err, "refreshing login for the visualizer")
	}
	c.conf.Auth = refreshed
	return errors.Wrap(storeConfigToCache(c.conf), "caching refreshed token")
}
