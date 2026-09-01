package cli

import (
	"context"
	"regexp"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v3"
	apppb "go.viam.com/api/app/v1"
)

// orgPublicNamespacePattern is the org public-namespace format used by the app:
// 2–39 characters; lowercase letters, numbers, and hyphens; must start and end
// with a letter or number.
var orgPublicNamespacePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,37}[a-z0-9]$`)

const orgPublicNamespaceErr = "public namespace must be 2-39 characters: lowercase letters, numbers, and hyphens, and must start and end with a letter or number"

type organizationsCreateArgs struct {
	Name            string
	PublicNamespace string
}

// OrganizationsCreateAction corresponds to `organizations create`.
func OrganizationsCreateAction(ctx context.Context, cmd *cli.Command, args organizationsCreateArgs) error {
	c, err := newViamClient(ctx, cmd)
	if err != nil {
		return err
	}
	return c.organizationsCreateAction(ctx, cmd, args)
}

func (c *viamClient) organizationsCreateAction(ctx context.Context, cmd *cli.Command, args organizationsCreateArgs) error {
	if err := fillOrganizationsCreateArgs(&args); err != nil {
		return err
	}
	args.Name = strings.TrimSpace(args.Name)
	ns := strings.TrimSpace(args.PublicNamespace)
	if args.Name == "" {
		return errors.New("organization name must not be empty")
	}
	if !orgPublicNamespacePattern.MatchString(ns) {
		return errors.New(orgPublicNamespaceErr)
	}

	avail, err := c.client.GetOrganizationNamespaceAvailability(ctx, &apppb.GetOrganizationNamespaceAvailabilityRequest{
		PublicNamespace: ns,
	})
	if err != nil {
		return errors.Wrapf(err, "could not check availability of public namespace %q", ns)
	}
	if !avail.GetAvailable() {
		return errors.Errorf("public namespace %q is already taken", ns)
	}

	created, err := c.client.CreateOrganization(ctx, &apppb.CreateOrganizationRequest{Name: args.Name})
	if err != nil {
		return errors.Wrapf(err, "failed to create organization %q", args.Name)
	}
	org := created.GetOrganization()
	if org == nil {
		return errors.Errorf("failed to create organization %q: empty response", args.Name)
	}
	printf(cmd.Root().Writer, "Created organization %q (id: %s)", org.GetName(), org.GetId())

	if org.GetPublicNamespace() == ns {
		printf(cmd.Root().Writer, "Public namespace %q is ready for module generate", ns)
		return nil
	}

	resp, err := c.client.UpdateOrganization(ctx, &apppb.UpdateOrganizationRequest{
		OrganizationId:  org.GetId(),
		PublicNamespace: &ns,
	})
	if err != nil {
		return errors.Wrapf(err,
			"created organization %q (id: %s) but failed to set public namespace %q; set it in the Viam app: organization dropdown → Settings",
			org.GetName(), org.GetId(), ns)
	}
	updated := resp.GetOrganization()
	if updated != nil && updated.GetPublicNamespace() != "" {
		ns = updated.GetPublicNamespace()
	}
	printf(cmd.Root().Writer, "Set public namespace %q", ns)
	return nil
}

func fillOrganizationsCreateArgs(args *organizationsCreateArgs) error {
	if args.Name != "" && args.PublicNamespace != "" {
		return nil
	}
	if !isInteractive() {
		return errors.New("missing required flags for non-interactive mode; provide --name and --public-namespace")
	}
	return promptOrganizationsCreate(args)
}

func promptOrganizationsCreate(args *organizationsCreateArgs) error {
	fields := []huh.Field{}
	if args.Name == "" {
		fields = append(fields, huh.NewInput().
			Title("Organization name").
			Description("Display name for the organization.").
			Placeholder("My Org").
			Value(&args.Name).
			Validate(func(s string) error {
				if s == "" {
					return errors.New("organization name must not be empty")
				}
				return nil
			}))
	}
	if args.PublicNamespace == "" {
		fields = append(fields, huh.NewInput().
			Title("Public namespace").
			Description("Used in module IDs (namespace:module-name).\n2-39 characters: lowercase letters, numbers, and hyphens; must start and end with a letter or number. Set this before viam module generate.").
			Placeholder("my-namespace").
			Value(&args.PublicNamespace).
			Validate(func(s string) error {
				if !orgPublicNamespacePattern.MatchString(s) {
					return errors.New(orgPublicNamespaceErr)
				}
				return nil
			}))
	}
	form := huh.NewForm(huh.NewGroup(fields...)).WithHeight(25).WithWidth(88)
	if err := form.Run(); err != nil {
		return errors.Wrap(err, "encountered an error creating an organization")
	}
	return nil
}
