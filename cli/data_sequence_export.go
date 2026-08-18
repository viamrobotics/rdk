package cli

import (
	"context"
	"io"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v3"
	datapb "go.viam.com/api/app/data/v1"

	"go.viam.com/rdk/data"
)

const (
	sequenceTabularDir      = "tabular"
	sequenceBinaryExportDir = "binary"
)

var unsafeFileNameChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

type dataExportSequenceArgs struct {
	Destination string
	SequenceID  string
	Parallel    uint
	Timeout     uint
	OnlyTabular bool
	OnlyBinary  bool
}

// DataExportSequenceAction is the corresponding action for 'data export sequence'.
func DataExportSequenceAction(ctx context.Context, cmd *cli.Command, args dataExportSequenceArgs) error {
	client, err := newViamClient(ctx, cmd)
	if err != nil {
		return err
	}

	return client.dataExportSequenceAction(ctx, args)
}

// dataExportSequenceAction exports a sequence's binary and tabular data.
func (c *viamClient) dataExportSequenceAction(ctx context.Context, args dataExportSequenceArgs) error {
	if args.OnlyTabular && args.OnlyBinary {
		return errors.Errorf("--%s and --%s cannot both be provided", dataFlagOnlyTabular, dataFlagOnlyBinary)
	}

	resp, err := c.dataClient.GetSequence(ctx, &datapb.GetSequenceRequest{Id: args.SequenceID})
	if err != nil {
		return errors.Wrapf(err, "failed to look up sequence %s", args.SequenceID)
	}
	sequence := resp.GetSequence()
	if sequence == nil {
		return errors.Errorf("sequence %s not found", args.SequenceID)
	}

	if err := makeDestinationDirs(args.Destination); err != nil {
		return errors.Wrap(err, "could not create destination directory")
	}

	printf(c.c.Root().Writer, "Exporting sequence %s to %s", sequence.GetId(), args.Destination)

	tabular, binary := partitionResourcesByCaptureType(sequence.GetResources())
	if !args.OnlyBinary {
		if err := c.exportSequenceTabular(ctx, sequence, tabular, args.Destination); err != nil {
			return err
		}
	}
	if !args.OnlyTabular {
		if err := c.exportSequenceBinary(ctx, sequence.GetId(), binary, args.Destination, args.Parallel, args.Timeout); err != nil {
			return err
		}
	}

	return nil
}

// exportSequenceTabular runs one tabular export per resource the sequence references, scoped to
// the sequence's part and capture interval, writing each to its own NDJSON file under dst/tabular/.
func (c *viamClient) exportSequenceTabular(
	ctx context.Context, sequence *datapb.Sequence, resources []*datapb.SequenceResourceFilter, dst string,
) error {
	printf(c.c.Root().Writer, "")
	printf(c.c.Root().Writer, "Tabular data (%s/):", sequenceTabularDir)

	if len(resources) == 0 {
		printf(c.c.Root().Writer, "  none")
		return nil
	}

	tabularDir := filepath.Join(dst, sequenceTabularDir)
	if err := makeDestinationDirs(tabularDir); err != nil {
		return errors.Wrapf(err, "could not create %s", tabularDir)
	}

	interval := &datapb.CaptureInterval{Start: sequence.GetStartTime(), End: sequence.GetEndTime()}
	names := sequenceTabularFileNames(resources)
	for i, resource := range resources {
		subtype, err := c.resolveResourceSubtype(ctx, sequence.GetPartId(), resource, interval)
		if err != nil {
			return err
		}
		if subtype == "" {
			printf(c.c.Root().Writer, "  %s %s", resource.GetResourceName(), resource.GetMethodName())
			printf(c.c.Root().Writer, "    no tabular data in the sequence's interval, skipping")
			continue
		}

		request := &datapb.ExportTabularDataRequest{
			PartId:          sequence.GetPartId(),
			ResourceName:    resource.GetResourceName(),
			ResourceSubtype: subtype,
			MethodName:      resource.GetMethodName(),
			Interval:        interval,
		}

		// Name the resource before the export starts so it is on screen while the work runs, then
		// add the row count below it. io.Discard because that writer's only output is a dot per
		// retry attempt, which the count supersedes.
		printf(c.c.Root().Writer, "  %s %s", resource.GetResourceName(), resource.GetMethodName())
		rows, err := c.tabularDataToFile(filepath.Join(tabularDir, names[i]), request, io.Discard)
		if err != nil {
			return errors.Wrapf(err, "failed to export tabular data for resource %q method %q",
				resource.GetResourceName(), resource.GetMethodName())
		}
		printf(c.c.Root().Writer, "    %d %s", rows, pluralize(rows, "row"))
	}
	return nil
}

// partitionResourcesByCaptureType splits a sequence's resources by what their capture method
// produces. MethodToCaptureType defaults anything it does not recognise to tabular.
func partitionResourcesByCaptureType(
	resources []*datapb.SequenceResourceFilter,
) (tabular, binary []*datapb.SequenceResourceFilter) {
	for _, resource := range resources {
		if data.MethodToCaptureType(resource.GetMethodName()) == data.CaptureTypeBinary {
			binary = append(binary, resource)
			continue
		}
		tabular = append(tabular, resource)
	}
	return tabular, binary
}

// resolveResourceSubtype discovers a resource's subtype, which ExportTabularData requires but a
// SequenceResourceFilter does not record. It asks for a single captured row matching the same
// part, resource, method, and interval the export will use, and reads the subtype off that row's
// CaptureMetadata. Returns "" when the resource has no tabular data in the interval.
func (c *viamClient) resolveResourceSubtype(
	ctx context.Context, partID string, resource *datapb.SequenceResourceFilter, interval *datapb.CaptureInterval,
) (string, error) {
	//nolint:staticcheck // TabularDataByFilter is deprecated, but we are using this to get the missing subtype.
	// We can safely remove if/when we update sequence resources to record subtypes.
	resp, err := c.dataClient.TabularDataByFilter(ctx, &datapb.TabularDataByFilterRequest{
		DataRequest: &datapb.DataRequest{
			Filter: &datapb.Filter{
				PartId:        partID,
				ComponentName: resource.GetResourceName(),
				Method:        resource.GetMethodName(),
				Interval:      interval,
			},
			Limit: 1,
		},
	})
	if err != nil {
		return "", errors.Wrapf(err, "failed to look up the subtype of resource %q method %q",
			resource.GetResourceName(), resource.GetMethodName())
	}
	for _, meta := range resp.GetMetadata() {
		if subtype := meta.GetComponentType(); subtype != "" {
			return subtype, nil
		}
	}
	return "", nil
}

// sequenceTabularFileNames builds one NDJSON file name per resource, positionally matching
// resources. Names are derived from the resource and method name; resources that sanitize to the
// same string get a numeric suffix so no export silently overwrites another.
func sequenceTabularFileNames(resources []*datapb.SequenceResourceFilter) []string {
	names := make([]string, 0, len(resources))
	seen := map[string]int{}
	for _, resource := range resources {
		var parts []string

		// Sanitize name just in case. Our FE has allowed character set, but edge case - they used API to update config.
		if name := strings.Trim(unsafeFileNameChars.ReplaceAllString(resource.GetResourceName(), "_"), "_."); name != "" {
			parts = append(parts, name)
		}
		if method := resource.GetMethodName(); method != "" {
			parts = append(parts, method)
		}
		if len(parts) == 0 {
			parts = append(parts, "resource")
		}

		base := strings.Join(parts, "-")
		seen[base]++
		// Edge case - two sanitized names are the same => append a number to the end.
		if n := seen[base]; n > 1 {
			base += "-" + strconv.Itoa(n)
		}
		names = append(names, base+".ndjson")
	}

	return names
}

// exportSequenceBinary downloads every binary datum the sequence references into
// <destination>/binary, using the same layout and parallel-download machinery as
// `data export binary`, and reports the result per resource so the section reads like the
// tabular one above it.
func (c *viamClient) exportSequenceBinary(
	ctx context.Context, sequenceID string, resources []*datapb.SequenceResourceFilter,
	dst string, parallel, timeout uint,
) error {
	binaryDst := filepath.Join(dst, sequenceBinaryExportDir)

	// resourceOf is populated while paging and read by the download workers, so both sides take
	// progressMu. Downloads run in parallel across resources, so a per-resource line cannot update
	// live; a single running total does that, and the per-resource split is reported at the end.
	var progressMu sync.Mutex
	resourceOf := map[string]string{}
	countByResource := map[string]int{}
	for _, resource := range resources {
		countByResource[resourceLabel(resource.GetResourceName(), resource.GetMethodName())] = 0
	}
	var downloaded atomic.Int32

	fetchIDsInto := func(ctx context.Context, ids chan<- string) error {
		defer close(ids)
		return forEachSequenceBinaryData(ctx, c.dataClient, sequenceID, func(bd *datapb.BinaryData) error {
			id := bd.GetMetadata().GetBinaryDataId()

			capture := bd.GetMetadata().GetCaptureMetadata()

			progressMu.Lock()
			resourceOf[id] = resourceLabel(capture.GetComponentName(), capture.GetMethodName())
			progressMu.Unlock()

			select {
			case ids <- id:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}

	download := func(ctx context.Context, id string) error {
		if err := c.downloadBinary(ctx, binaryDst, timeout, id); err != nil {
			return err
		}
		downloaded.Add(1)

		progressMu.Lock()
		defer progressMu.Unlock()
		countByResource[resourceOf[id]]++
		return nil
	}

	printf(c.c.Root().Writer, "")
	printf(c.c.Root().Writer, "Binary data (%s/):", sequenceBinaryExportDir)
	if err := c.performActionOnBinaryDataIDs(ctx, fetchIDsInto, download, parallel, func(int32) {}); err != nil {
		return err
	}
	if len(countByResource) == 0 {
		printf(c.c.Root().Writer, "  none")
		return nil
	}

	for _, resource := range slices.Sorted(maps.Keys(countByResource)) {
		printf(c.c.Root().Writer, "  %s", resource)
		printf(c.c.Root().Writer, "    %d %s", countByResource[resource], pluralize(countByResource[resource], "file"))
	}
	return nil
}

func resourceLabel(name, method string) string {
	return name + " " + method
}
