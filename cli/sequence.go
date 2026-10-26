package cli

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v3"
	datapb "go.viam.com/api/app/data/v1"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

const (
	sequenceFlagSequenceID  = "sequence-id"
	sequenceFlagSequenceIDs = "sequence-ids"
	sequenceFlagResources   = "resources"
)

// parseSequenceResources parses "<resource-name>:<method>" values into resource filters.
func parseSequenceResources(values []string) ([]*datapb.SequenceResourceFilter, error) {
	resources := make([]*datapb.SequenceResourceFilter, 0, len(values))
	for _, v := range values {
		name, method, found := strings.Cut(v, ":")
		if !found || name == "" || method == "" {
			return nil, errors.Errorf("invalid --%s value %q (expected <resource-name>:<method>)", sequenceFlagResources, v)
		}
		resources = append(resources, &datapb.SequenceResourceFilter{ResourceName: name, MethodName: method})
	}
	return resources, nil
}

// nonBlankTags drops empty entries so that `--tags ""` yields no tags (clearing them on update)
// rather than a single blank tag.
func nonBlankTags(tags []string) []string {
	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		if tag != "" {
			result = append(result, tag)
		}
	}
	return result
}

func formatSequenceResources(resources []*datapb.SequenceResourceFilter) string {
	formatted := make([]string, 0, len(resources))
	for _, r := range resources {
		formatted = append(formatted, r.GetResourceName()+":"+r.GetMethodName())
	}
	return strings.Join(formatted, ",")
}

func (c *viamClient) printSequence(seq *datapb.Sequence) {
	w := c.c.Root().Writer
	printf(w, "ID: %s", seq.GetId())
	printf(w, "\tPart ID: %s", seq.GetPartId())
	printf(w, "\tResources: %s", formatSequenceResources(seq.GetResources()))
	printf(w, "\tTags: %s", strings.Join(seq.GetSequenceTags(), ","))
	printf(w, "\tStart: %s", formatTimestamp(seq.GetStartTime()))
	printf(w, "\tEnd: %s", formatTimestamp(seq.GetEndTime()))
	printf(w, "\tCreated: %s", formatTimestamp(seq.GetCreatedAt()))
	printf(w, "\tUpdated: %s", formatTimestamp(seq.GetUpdatedAt()))
}

type dataSequenceCreateArgs struct {
	PartID    string
	Resources []string
	Tags      []string
	Start     string
	End       string
}

// DataSequenceCreateAction is the corresponding action for 'data sequence create'.
func DataSequenceCreateAction(ctx context.Context, cmd *cli.Command, args dataSequenceCreateArgs) error {
	client, err := newViamClient(ctx, cmd)
	if err != nil {
		return err
	}
	return client.createSequence(ctx, args)
}

func (c *viamClient) createSequence(ctx context.Context, args dataSequenceCreateArgs) error {
	resources, err := parseSequenceResources(args.Resources)
	if err != nil {
		return err
	}
	if args.Start == "" || args.End == "" {
		return errors.Errorf("must provide both --%s and --%s", generalFlagStart, generalFlagEnd)
	}
	start, err := parseTimeString(args.Start)
	if err != nil {
		return err
	}
	end, err := parseTimeString(args.End)
	if err != nil {
		return err
	}

	resp, err := c.dataClient.CreateSequence(ctx, &datapb.CreateSequenceRequest{
		PartId:       args.PartID,
		Resources:    resources,
		SequenceTags: nonBlankTags(args.Tags),
		StartTime:    start,
		EndTime:      end,
	})
	if err != nil {
		return errors.Wrap(err, serverErrorMessage)
	}
	printf(c.c.Root().Writer, "Created sequence with ID: %s", resp.GetId())
	return nil
}

type dataSequenceGetArgs struct {
	SequenceID string
}

// DataSequenceGetAction is the corresponding action for 'data sequence get'.
func DataSequenceGetAction(ctx context.Context, cmd *cli.Command, args dataSequenceGetArgs) error {
	client, err := newViamClient(ctx, cmd)
	if err != nil {
		return err
	}
	return client.getSequence(ctx, args.SequenceID)
}

func (c *viamClient) getSequence(ctx context.Context, sequenceID string) error {
	resp, err := c.dataClient.GetSequence(ctx, &datapb.GetSequenceRequest{Id: sequenceID})
	if err != nil {
		return errors.Wrap(err, serverErrorMessage)
	}
	c.printSequence(resp.GetSequence())
	return nil
}

type dataSequenceUpdateArgs struct {
	SequenceID string
	Resources  []string
	Tags       []string
	Start      string
	End        string
}

// DataSequenceUpdateAction is the corresponding action for 'data sequence update'.
func DataSequenceUpdateAction(ctx context.Context, cmd *cli.Command, args dataSequenceUpdateArgs) error {
	client, err := newViamClient(ctx, cmd)
	if err != nil {
		return err
	}
	return client.updateSequence(ctx, args)
}

// updateSequence only sends fields whose flags were explicitly passed, so unset flags leave
// the stored values untouched rather than clearing them.
func (c *viamClient) updateSequence(ctx context.Context, args dataSequenceUpdateArgs) error {
	req := &datapb.UpdateSequenceRequest{Id: args.SequenceID}
	var paths []string

	if c.c.IsSet(sequenceFlagResources) {
		resources, err := parseSequenceResources(args.Resources)
		if err != nil {
			return err
		}
		req.Resources = resources
		paths = append(paths, "resources")
	}
	if c.c.IsSet(generalFlagTags) {
		req.SequenceTags = nonBlankTags(args.Tags)
		paths = append(paths, "sequence_tags")
	}
	if c.c.IsSet(generalFlagStart) {
		if args.Start == "" {
			return errors.Errorf("--%s cannot be empty", generalFlagStart)
		}
		start, err := parseTimeString(args.Start)
		if err != nil {
			return err
		}
		req.StartTime = start
		paths = append(paths, "start_time")
	}
	if c.c.IsSet(generalFlagEnd) {
		if args.End == "" {
			return errors.Errorf("--%s cannot be empty", generalFlagEnd)
		}
		end, err := parseTimeString(args.End)
		if err != nil {
			return err
		}
		req.EndTime = end
		paths = append(paths, "end_time")
	}
	if len(paths) == 0 {
		return errors.Errorf("must provide at least one of --%s, --%s, --%s, or --%s",
			sequenceFlagResources, generalFlagTags, generalFlagStart, generalFlagEnd)
	}
	req.FieldMask = &fieldmaskpb.FieldMask{Paths: paths}

	if _, err := c.dataClient.UpdateSequence(ctx, req); err != nil {
		return errors.Wrap(err, serverErrorMessage)
	}
	printf(c.c.Root().Writer, "Updated sequence %s (%s)", args.SequenceID, strings.Join(paths, ", "))
	return nil
}

type dataSequenceDeleteArgs struct {
	SequenceIDs []string
}

// DataSequenceDeleteAction is the corresponding action for 'data sequence delete'.
func DataSequenceDeleteAction(ctx context.Context, cmd *cli.Command, args dataSequenceDeleteArgs) error {
	client, err := newViamClient(ctx, cmd)
	if err != nil {
		return err
	}
	return client.deleteSequences(ctx, args.SequenceIDs)
}

func (c *viamClient) deleteSequences(ctx context.Context, sequenceIDs []string) error {
	for _, id := range sequenceIDs {
		if _, err := c.dataClient.DeleteSequence(ctx, &datapb.DeleteSequenceRequest{Id: id}); err != nil {
			return errors.Wrapf(err, "%s while deleting sequence %s", serverErrorMessage, id)
		}
		printf(c.c.Root().Writer, "Deleted sequence %s", id)
	}
	return nil
}

type datasetSequenceArgs struct {
	DatasetID   string
	SequenceIDs []string
}

// DatasetSequenceAddAction is the corresponding action for 'dataset sequence add'.
func DatasetSequenceAddAction(ctx context.Context, cmd *cli.Command, args datasetSequenceArgs) error {
	client, err := newViamClient(ctx, cmd)
	if err != nil {
		return err
	}
	return client.addSequencesToDataset(ctx, args.DatasetID, args.SequenceIDs)
}

func (c *viamClient) addSequencesToDataset(ctx context.Context, datasetID string, sequenceIDs []string) error {
	if _, err := c.dataClient.AddSequencesToDataset(ctx, &datapb.AddSequencesToDatasetRequest{
		DatasetId:   datasetID,
		SequenceIds: sequenceIDs,
	}); err != nil {
		return errors.Wrap(err, serverErrorMessage)
	}
	printf(c.c.Root().Writer, "Added %d %s to dataset %s", len(sequenceIDs), pluralize(len(sequenceIDs), "sequence"), datasetID)
	return nil
}

// DatasetSequenceRemoveAction is the corresponding action for 'dataset sequence remove'.
func DatasetSequenceRemoveAction(ctx context.Context, cmd *cli.Command, args datasetSequenceArgs) error {
	client, err := newViamClient(ctx, cmd)
	if err != nil {
		return err
	}
	return client.removeSequencesFromDataset(ctx, args.DatasetID, args.SequenceIDs)
}

func (c *viamClient) removeSequencesFromDataset(ctx context.Context, datasetID string, sequenceIDs []string) error {
	if _, err := c.dataClient.RemoveSequencesFromDataset(ctx, &datapb.RemoveSequencesFromDatasetRequest{
		DatasetId:   datasetID,
		SequenceIds: sequenceIDs,
	}); err != nil {
		return errors.Wrap(err, serverErrorMessage)
	}
	printf(c.c.Root().Writer, "Removed %d %s from dataset %s", len(sequenceIDs), pluralize(len(sequenceIDs), "sequence"), datasetID)
	return nil
}
