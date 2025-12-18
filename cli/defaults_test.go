package cli

import (
	"context"
	"testing"

	"github.com/google/uuid"
	apppb "go.viam.com/api/app/v1"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/test"
	"google.golang.org/grpc"
)

func TestSetOrg(t *testing.T) {
	orgID := uuid.New().String()
	organizations := []*apppb.Organization{
		{
			Id: orgID,
		},
	}
	listOrgsFunc := func(ctx context.Context, in *apppb.ListOrganizationsRequest, opts ...grpc.CallOption) (*apppb.ListOrganizationsResponse, error) {
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
			_, err := vc.setDefaultOrg(cCtx, &config, tt.orgID)
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
	listOrgsFunc := func(ctx context.Context, in *apppb.ListOrganizationsRequest, opts ...grpc.CallOption) (*apppb.ListOrganizationsResponse, error) {
		return &apppb.ListOrganizationsResponse{Organizations: organizations}, nil
	}
	locations := []*apppb.Location{
		{
			Id: "my-loc-id",
		},
	}
	listLocationsFunc := func(ctx context.Context, in *apppb.ListLocationsRequest, opts ...grpc.CallOption) (*apppb.ListLocationsResponse, error) {
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
			_, err := vc.setDefaultLocation(cCtx, &config, tt.locationID)
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
