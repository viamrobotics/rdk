package cli

import (
	"context"
	stderrors "errors"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v3"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"go.viam.com/utils/perf"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"braces.dev/errtrace"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/services/shell"
)

// traces are stored at VIAM_HOME/trace/[part-id]/traces
const tracesRelativePath = "trace"

var (
	tracesRootDir     = path.Join("~", ".viam")
	defaultTracesPath = path.Join(tracesRootDir, tracesRelativePath)
)

type traceGetRemoteArgs struct {
	Organization string
	Location     string
	Machine      string
	Part         string
	ViamHomeDir  string
}

type traceImportRemoteArgs struct {
	Organization string
	Location     string
	Machine      string
	Part         string
	Endpoint     string
}

type traceImportLocalArgs struct {
	Endpoint string
}

func traceImportRemoteAction(ctx context.Context, cmd *cli.Command, args traceImportRemoteArgs) error {
	client, err := newViamClient(ctx, cmd)
	if err != nil {
		return errtrace.Wrap(err)
	}

	globalArgs, err := getGlobalArgs(cmd)
	if err != nil {
		return errtrace.Wrap(err)
	}
	logger := globalArgs.createLogger()
	tmp, err := os.MkdirTemp("", "viamtraceimport")
	if err != nil {
		return errtrace.Wrap(err)
	}
	//nolint: errcheck
	defer os.RemoveAll(tmp)
	if err := client.tracesGetRemoteAction(
		ctx,
		cmd,
		traceGetRemoteArgs{
			Organization: args.Organization,
			Location:     args.Location,
			Machine:      args.Machine,
			Part:         args.Part,
		},
		tmp,
		false,
		globalArgs.Debug,
		logger,
	); err != nil {
		return errtrace.Wrap(err)
	}

	return errtrace.Wrap(traceImportLocal(ctx, cmd, traceImportLocalArgs{
		Endpoint: args.Endpoint,
	},
		filepath.Join(tmp, "traces"),
	))
}

func (c *viamClient) tracesGetRemoteAction(
	goCtx context.Context,
	ctx *cli.Command,
	flagArgs traceGetRemoteArgs,
	target string,
	getAll bool,
	debug bool,
	logger logging.Logger,
) error {
	part, err := c.robotPart(goCtx, flagArgs.Organization, flagArgs.Location, flagArgs.Machine, flagArgs.Part)
	if err != nil {
		return errtrace.Wrap(err)
	}
	// Intentional use of path instead of filepath: Windows understands both / and
	// \ as path separators, and we don't want a cli running on Windows to send
	// a path using \ to a *NIX machine.
	src := path.Join(defaultTracesPath, part.Id)
	// if target part has a non-default VIAM_HOME, the caller can specify it.
	if flagArgs.ViamHomeDir != "" {
		src = path.Join(flagArgs.ViamHomeDir, tracesRelativePath, part.Id)
	}
	// if getAll is set then download the entire directory, including rotated
	// files. Otherwise just get the current file.
	if !getAll {
		src = path.Join(src, "traces")
	}
	gArgs, err := getGlobalArgs(ctx)
	quiet := err == nil && gArgs != nil && gArgs.Quiet
	var startTime time.Time
	if !quiet {
		startTime = time.Now()
		printf(ctx.Root().Writer, "Saving to %s ...", path.Join(target))
	}
	if err := c.copyFilesFromMachine(
		goCtx,
		flagArgs.Organization,
		flagArgs.Location,
		flagArgs.Machine,
		flagArgs.Part,
		debug,
		true,
		false,
		[]string{src},
		target,
		logger,
	); err != nil {
		if statusErr := status.Convert(err); statusErr != nil &&
			statusErr.Code() == codes.InvalidArgument &&
			statusErr.Message() == shell.ErrMsgDirectoryCopyRequestNoRecursion {
			return errtrace.Wrap(errDirectoryCopyRequestNoRecursion)
		}
		return errtrace.Wrap(err)
	}
	if !quiet {
		printf(ctx.Root().Writer, "Download finished in %s.", time.Since(startTime))
	}
	return nil
}

func tracePrintRemoteAction(ctx context.Context, cmd *cli.Command, args traceGetRemoteArgs) error {
	client, err := newViamClient(ctx, cmd)
	if err != nil {
		return errtrace.Wrap(err)
	}

	globalArgs, err := getGlobalArgs(cmd)
	if err != nil {
		return errtrace.Wrap(err)
	}
	logger := globalArgs.createLogger()
	tmp, err := os.MkdirTemp("", "viamtraceimport")
	if err != nil {
		return errtrace.Wrap(err)
	}
	//nolint: errcheck
	defer os.RemoveAll(tmp)
	if err := client.tracesGetRemoteAction(
		ctx,
		cmd,
		args,
		tmp,
		false,
		globalArgs.Debug,
		logger,
	); err != nil {
		return errtrace.Wrap(err)
	}
	return errtrace.Wrap(tracePrintLocal(ctx, cmd, filepath.Join(tmp, "traces")))
}

func getSingularArg(ctx *cli.Command) (string, error) {
	cliArgs := ctx.Args().Slice()
	var result string
	switch numArgs := len(cliArgs); numArgs {
	case 1:
		result = cliArgs[0]
	default:
		return "", errtrace.Wrap(wrongNumArgsError{have: numArgs, min: 1})
	}
	return result, nil
}

func traceGetRemoteAction(ctx context.Context, cmd *cli.Command, args traceGetRemoteArgs) error {
	cliArgs := cmd.Args().Slice()
	var targetPath string
	switch numArgs := len(cliArgs); numArgs {
	case 0:
		var err error
		targetPath, err = os.Getwd()
		if err != nil {
			return errtrace.Wrap(err)
		}
	case 1:
		targetPath = cliArgs[0]
	default:
		return errtrace.Wrap(wrongNumArgsError{numArgs, 0, 1})
	}

	client, err := newViamClient(ctx, cmd)
	if err != nil {
		return errtrace.Wrap(err)
	}

	globalArgs, err := getGlobalArgs(cmd)
	if err != nil {
		return errtrace.Wrap(err)
	}
	logger := globalArgs.createLogger()

	return errtrace.Wrap(client.tracesGetRemoteAction(ctx, cmd, args, targetPath, true, globalArgs.Debug, logger))
}

func tracePrintLocalAction(ctx context.Context, cmd *cli.Command, _ struct{}) error {
	target, err := getSingularArg(cmd)
	if err != nil {
		return errtrace.Wrap(err)
	}
	return errtrace.Wrap(tracePrintLocal(ctx, cmd, target))
}

func tracePrintLocal(
	ctx context.Context,
	cmd *cli.Command,
	source string,
) error {
	//nolint: gosec
	traceFile, err := os.Open(source)
	if err != nil {
		if os.IsNotExist(err) {
			printf(cmd.Root().Writer, "No traces found")
			return nil
		}
		return errtrace.Wrap(errors.Wrap(err, "failed to open trace file"))
	}
	traceReader := protoutils.NewDelimitedProtoReader[tracepb.ResourceSpans](traceFile)
	//nolint: errcheck
	defer traceReader.Close()

	devExporter := perf.NewOtelDevelopmentExporter()
	var msg tracepb.ResourceSpans
	err = nil
	for resource := range traceReader.AllWithMemory(&msg) {
		for _, scope := range resource.ScopeSpans {
			err = stderrors.Join(err, devExporter.ExportOTLPSpans(ctx, scope.Spans))
		}
	}
	return errtrace.Wrap(err)
}

func traceImportLocalAction(ctx context.Context, cmd *cli.Command, args traceImportLocalArgs) error {
	target, err := getSingularArg(cmd)
	if err != nil {
		return errtrace.Wrap(err)
	}
	return errtrace.Wrap(traceImportLocal(ctx, cmd, args, target))
}

func traceImportLocal(
	ctx context.Context,
	cmd *cli.Command,
	args traceImportLocalArgs,
	source string,
) error {
	//nolint: gosec
	traceFile, err := os.Open(source)
	if err != nil {
		if os.IsNotExist(err) {
			printf(cmd.Root().Writer, "No traces found")
			return nil
		}
		return errtrace.Wrap(errors.Wrap(err, "failed to open trace file"))
	}
	traceReader := protoutils.NewDelimitedProtoReader[tracepb.ResourceSpans](traceFile)
	//nolint: errcheck
	defer traceReader.Close()
	endpoint := args.Endpoint
	if endpoint == "" {
		endpoint = "localhost:4317"
	}
	opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(endpoint)}
	if strings.HasPrefix(endpoint, "localhost:") {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}
	otlpClient := otlptracegrpc.NewClient(opts...)
	if err := otlpClient.Start(ctx); err != nil {
		return errtrace.Wrap(err)
	}
	//nolint: errcheck
	defer otlpClient.Stop(ctx)
	var msg tracepb.ResourceSpans
	msgSuccess := 0
	msgTotal := 0
	printf(cmd.Root().Writer, "Importing spans to %v...", endpoint)
	for span := range traceReader.AllWithMemory(&msg) {
		msgTotal++
		err := otlpClient.UploadTraces(ctx, []*tracepb.ResourceSpans{span})
		if err != nil {
			printf(cmd.Root().Writer, "Error uploading trace: %v", err)
		} else {
			msgSuccess++
		}
	}
	printf(cmd.Root().Writer, "Imported %d/%d messages", msgSuccess, msgTotal)
	return nil
}
