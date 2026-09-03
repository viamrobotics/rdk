package cli

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	apppb "go.viam.com/api/app/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"

	"go.viam.com/rdk/testutils/inject"
)

func TestSetOrg(t *testing.T) {
	orgID := uuid.New().String()
	organizations := []*apppb.Organization{
		{
			Id: orgID,
		},
	}
	listOrgsFunc := func(ctx context.Context, in *apppb.ListOrganizationsRequest,
		opts ...grpc.CallOption,
	) (*apppb.ListOrganizationsResponse, error) {
		return &apppb.ListOrganizationsResponse{Organizations: organizations}, nil
	}

	asc := &inject.AppServiceClient{
		ListOrganizationsFunc: listOrgsFunc,
	}

	tests := []struct {
		name       string
		orgID      string
		shouldPass bool
	}{
		{
			name:       "matching org ID",
			orgID:      orgID,
			shouldPass: true,
		},
		{
			name:       "non-matching org ID",
			orgID:      "some-other-org-id",
			shouldPass: false,
		},
		{
			name:       "empty org-id for clearing org",
			shouldPass: true,
		},
	}
	cCtx, vc, _, _ := setup(asc, nil, nil, map[string]any{"org-id": orgID}, "token")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{}
			_, err := vc.setDefaultOrg(context.Background(), cCtx, &config, tt.orgID)
			test.That(t, err == nil, test.ShouldEqual, tt.shouldPass)
			if tt.shouldPass {
				test.That(t, config.DefaultOrg, test.ShouldResemble, tt.orgID)
			}
		})
	}
}

func TestSetLocation(t *testing.T) {
	orgID := uuid.New().String()
	organizations := []*apppb.Organization{
		{
			Id: orgID,
		},
	}
	listOrgsFunc := func(ctx context.Context, in *apppb.ListOrganizationsRequest,
		opts ...grpc.CallOption,
	) (*apppb.ListOrganizationsResponse, error) {
		return &apppb.ListOrganizationsResponse{Organizations: organizations}, nil
	}
	locations := []*apppb.Location{
		{
			Id: "my-loc-id",
		},
	}
	listLocationsFunc := func(ctx context.Context, in *apppb.ListLocationsRequest,
		opts ...grpc.CallOption,
	) (*apppb.ListLocationsResponse, error) {
		return &apppb.ListLocationsResponse{Locations: locations}, nil
	}

	asc := &inject.AppServiceClient{
		ListLocationsFunc:     listLocationsFunc,
		ListOrganizationsFunc: listOrgsFunc,
	}

	tests := []struct {
		name       string
		locationID string
		shouldPass bool
	}{
		{
			name:       "matching location ID",
			locationID: "my-loc-id",
			shouldPass: true,
		},
		{
			name:       "non-matching location ID",
			locationID: "some-other-loc-id",
			shouldPass: false,
		},
		{
			name:       "empty loc-id for clearing location",
			shouldPass: true,
		},
	}
	cCtx, vc, _, errOut := setup(asc, nil, nil, map[string]any{"location-id": "my-loc-id"}, "token")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{}
			_, err := vc.setDefaultLocation(context.Background(), cCtx, &config, tt.locationID)
			test.That(t, err == nil, test.ShouldEqual, tt.shouldPass)
			if tt.shouldPass {
				test.That(t, config.DefaultLocation, test.ShouldResemble, tt.locationID)
			} else {
				test.That(t, len(errOut.messages), test.ShouldBeGreaterThan, 0)
				test.That(t, errOut.messages[0], test.ShouldEqual, "Warning: ")
				test.That(t,
					errOut.messages[1],
					test.ShouldContainSubstring,
					"attempting to set a default location argument when no default org argument is set",
				)
				test.That(t, err.Error(), test.ShouldEqual, "no location found matching ID some-other-loc-id")
			}
		})
	}
}

func TestViewOrg(t *testing.T) {
	orgID := uuid.New().String()
	asc := &inject.AppServiceClient{
		ListOrganizationsFunc: func(ctx context.Context, in *apppb.ListOrganizationsRequest,
			opts ...grpc.CallOption,
		) (*apppb.ListOrganizationsResponse, error) {
			return &apppb.ListOrganizationsResponse{Organizations: []*apppb.Organization{
				{Id: orgID, Name: "otf"},
			}}, nil
		},
	}

	t.Run("unset", func(t *testing.T) {
		cCtx, vc, out, errOut := setup(asc, nil, nil, nil, "token")
		test.That(t, vc.viewDefaultOrgAction(context.Background(), cCtx), test.ShouldBeNil)
		test.That(t, len(errOut.messages), test.ShouldEqual, 0)
		test.That(t, len(out.messages), test.ShouldEqual, 1)
		test.That(t, out.messages[0], test.ShouldEqual, "No default organization set\n")
	})

	t.Run("set with name", func(t *testing.T) {
		cCtx, vc, out, errOut := setup(asc, nil, nil, nil, "token")
		vc.conf.DefaultOrg = orgID
		test.That(t, vc.viewDefaultOrgAction(context.Background(), cCtx), test.ShouldBeNil)
		test.That(t, len(errOut.messages), test.ShouldEqual, 0)
		test.That(t, len(out.messages), test.ShouldEqual, 1)
		test.That(t, out.messages[0], test.ShouldEqual, fmt.Sprintf("otf (id: %s)\n", orgID))
	})

	t.Run("set but org not found", func(t *testing.T) {
		cCtx, vc, out, errOut := setup(asc, nil, nil, nil, "token")
		missingID := uuid.New().String()
		vc.conf.DefaultOrg = missingID
		test.That(t, vc.viewDefaultOrgAction(context.Background(), cCtx), test.ShouldBeNil)
		test.That(t, len(errOut.messages), test.ShouldBeGreaterThan, 1)
		test.That(t, errOut.messages[0], test.ShouldEqual, "Warning: ")
		test.That(t, errOut.messages[1], test.ShouldContainSubstring, "could not resolve default org")
		test.That(t, errOut.messages[1], test.ShouldContainSubstring, missingID)
		test.That(t, errOut.messages[1], test.ShouldContainSubstring, "no organization found")
		test.That(t, len(out.messages), test.ShouldEqual, 1)
		test.That(t, out.messages[0], test.ShouldEqual, missingID+"\n")
	})
}

func TestViewLocation(t *testing.T) {
	orgID := uuid.New().String()
	locID := uuid.New().String()
	asc := &inject.AppServiceClient{
		ListOrganizationsFunc: func(ctx context.Context, in *apppb.ListOrganizationsRequest,
			opts ...grpc.CallOption,
		) (*apppb.ListOrganizationsResponse, error) {
			return &apppb.ListOrganizationsResponse{Organizations: []*apppb.Organization{
				{Id: orgID, Name: "otf"},
			}}, nil
		},
		ListLocationsFunc: func(ctx context.Context, in *apppb.ListLocationsRequest,
			opts ...grpc.CallOption,
		) (*apppb.ListLocationsResponse, error) {
			return &apppb.ListLocationsResponse{Locations: []*apppb.Location{
				{Id: locID, Name: "lab"},
			}}, nil
		},
	}

	t.Run("unset", func(t *testing.T) {
		cCtx, vc, out, errOut := setup(asc, nil, nil, nil, "token")
		test.That(t, vc.viewDefaultLocationAction(context.Background(), cCtx), test.ShouldBeNil)
		test.That(t, len(errOut.messages), test.ShouldEqual, 0)
		test.That(t, len(out.messages), test.ShouldEqual, 1)
		test.That(t, out.messages[0], test.ShouldEqual, "No default location set\n")
	})

	t.Run("set with name", func(t *testing.T) {
		cCtx, vc, out, errOut := setup(asc, nil, nil, nil, "token")
		vc.conf.DefaultOrg = orgID
		vc.conf.DefaultLocation = locID
		test.That(t, vc.viewDefaultLocationAction(context.Background(), cCtx), test.ShouldBeNil)
		test.That(t, len(errOut.messages), test.ShouldEqual, 0)
		test.That(t, len(out.messages), test.ShouldEqual, 1)
		test.That(t, out.messages[0], test.ShouldEqual, fmt.Sprintf("lab (id: %s)\n", locID))
	})

	t.Run("set without default org", func(t *testing.T) {
		cCtx, vc, out, errOut := setup(asc, nil, nil, nil, "token")
		vc.conf.DefaultLocation = locID
		test.That(t, vc.viewDefaultLocationAction(context.Background(), cCtx), test.ShouldBeNil)
		test.That(t, len(errOut.messages), test.ShouldEqual, 0)
		test.That(t, len(out.messages), test.ShouldEqual, 1)
		test.That(t, out.messages[0], test.ShouldEqual, locID+"\n")
	})
}
