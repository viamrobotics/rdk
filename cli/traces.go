package cli

import (
	stderrors "errors"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"go.viam.com/utils/perf"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/services/shell"
)

var tracesPath = path.Join("~", ".viam", "trace")

type traceGetRemoteArgs struct {
	Organization string
	Location     string
	Machine      string
	Part         string
	Destination  string
}

type traceImportRemoteArgs struct {
	Organization string
	Location     string
	Machine      string
	Part         string
	Endpoint     string
}

type traceImportLocalArgs struct {
	Path     string
	Endpoint string
}

type tracePrintLocalArgs struct {
	Path string
}

func traceImportRemoteAction(ctx *cli.Context, args traceImportRemoteArgs) error {
	client, err := newViamClient(ctx)
	if err != nil {
		return err
	}

	globalArgs, err := getGlobalArgs(ctx)
	if err != nil {
		return err
	}
	logger := globalArgs.createLogger()
	tmp, err := os.MkdirTemp("", "viamtraceimport")
	if err != nil {
		return err
	}
	//nolint: errcheck
	defer os.RemoveAll(tmp)
	if err := client.tracesGetRemoteAction(
		ctx,
		traceGetRemoteArgs{
			Organization: args.Organization,
			Location:     args.Location,
			Machine:      args.Machine,
			Part:         args.Part,
			Destination:  tmp,
		},
		globalArgs.Debug,
		logger,
	); err != nil {
		return err
	}

	return traceImportLocalAction(ctx, traceImportLocalArgs{
		Path:     filepath.Join(tmp, "traces"),
		Endpoint: args.Endpoint,
	})
}

func (c *viamClient) tracesGetRemoteAction(
	ctx *cli.Context,
	flagArgs traceGetRemoteArgs,
	debug bool,
	logger logging.Logger,
) error {
	part, err := c.robotPart(flagArgs.Organization, flagArgs.Location, flagArgs.Machine, flagArgs.Part)
	if err != nil {
		return err
	}
	// Intentional use of path instead of filepath: Windows understands both / and
	// \ as path separators, and we don't want a cli running on Windows to send
	// a path using \ to a *NIX machine.
	src := path.Join(tracesPath, part.Id, "traces")
	gArgs, err := getGlobalArgs(ctx)
	quiet := err == nil && gArgs != nil && gArgs.Quiet
	var startTime time.Time
	if !quiet {
		startTime = time.Now()
		printf(ctx.App.Writer, "Saving to %s ...", path.Join(flagArgs.Destination, part.GetId()))
	}
	if err := c.copyFilesFromMachine(
		flagArgs.Organization,
		flagArgs.Location,
		flagArgs.Machine,
		flagArgs.Part,
		debug,
		true,
		false,
		[]string{src},
		flagArgs.Destination,
		logger,
	); err != nil {
		if statusErr := status.Convert(err); statusErr != nil &&
			statusErr.Code() == codes.InvalidArgument &&
			statusErr.Message() == shell.ErrMsgDirectoryCopyRequestNoRecursion {
			return errDirectoryCopyRequestNoRecursion
		}
		return err
	}
	if !quiet {
		printf(ctx.App.Writer, "Download finished in %s.", time.Since(startTime))
	}
	return nil
}

func tracePrintRemoteAction(
	ctx *cli.Context,
	args machinesPartGetFTDCArgs,
) error {
	client, err := newViamClient(ctx)
	if err != nil {
		return err
	}

	globalArgs, err := getGlobalArgs(ctx)
	if err != nil {
		return err
	}
	logger := globalArgs.createLogger()
	tmp, err := os.MkdirTemp("", "viamtraceimport")
	if err != nil {
		return err
	}
	//nolint: errcheck
	defer os.RemoveAll(tmp)
	if err := client.tracesGetRemoteAction(
		ctx,
		traceGetRemoteArgs{
			Organization: args.Organization,
			Location:     args.Location,
			Machine:      args.Machine,
			Part:         args.Part,
			Destination:  tmp,
		},
		globalArgs.Debug,
		logger,
	); err != nil {
		return err
	}
	return tracePrintLocalAction(ctx, tracePrintLocalArgs{Path: filepath.Join(tmp, "traces")})
}

func traceGetRemoteAction(ctx *cli.Context, args traceGetRemoteArgs) error {
	client, err := newViamClient(ctx)
	if err != nil {
		return err
	}

	globalArgs, err := getGlobalArgs(ctx)
	if err != nil {
		return err
	}
	logger := globalArgs.createLogger()

	return client.tracesGetRemoteAction(ctx, args, globalArgs.Debug, logger)
}

func tracePrintLocalAction(
	ctx *cli.Context,
	args tracePrintLocalArgs,
) error {
	traceFile, err := os.Open(args.Path)
	if err != nil {
		if os.IsNotExist(err) {
			printf(ctx.App.Writer, "No traces found")
			return nil
		}
		return errors.Wrap(err, "failed to open trace file")
	}
	traceReader := protoutils.NewDelimitedProtoReader[tracepb.ResourceSpans](traceFile)
	//nolint: errcheck
	defer traceReader.Close()

	devExporter := perf.NewOtelDevelopmentExporter()
	if err := devExporter.Start(); err != nil {
		return err
	}
	defer devExporter.Stop()
	var msg tracepb.ResourceSpans
	err = nil
	for resource := range traceReader.AllWithMemory(&msg) {
		for _, scope := range resource.ScopeSpans {
			err = stderrors.Join(err, devExporter.ExportOTLPSpans(ctx.Context, scope.Spans))
		}
	}
	return err
}

func traceImportLocalAction(
	ctx *cli.Context,
	args traceImportLocalArgs,
) error {
	traceFile, err := os.Open(args.Path)
	if err != nil {
		if os.IsNotExist(err) {
			printf(ctx.App.Writer, "No traces found")
			return nil
		}
		return errors.Wrap(err, "failed to open trace file")
	}
	traceReader := protoutils.NewDelimitedProtoReader[tracepb.ResourceSpans](traceFile)
	//nolint: errcheck
	defer traceReader.Close()
	endpoint := args.Endpoint
	if endpoint == "" {
		endpoint = "localhost:4317"
	}
	otlpClient := otlptracegrpc.NewClient(
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err := otlpClient.Start(ctx.Context); err != nil {
		return err
	}
	//nolint: errcheck
	defer otlpClient.Stop(ctx.Context)
	var msg tracepb.ResourceSpans
	for span := range traceReader.AllWithMemory(&msg) {
		err := otlpClient.UploadTraces(ctx.Context, []*tracepb.ResourceSpans{span})
		if err != nil {
			printf(ctx.App.Writer, "Error uploading trace: %v", err)
		}
	}
	return nil
}
