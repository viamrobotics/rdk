package cli

import (
	"context"
	"strings"
	"testing"

	apppb "go.viam.com/api/app/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"

	"go.viam.com/rdk/testutils/inject"
)

func TestOrganizationsCreateAction(t *testing.T) {
	const orgID = "11111111-1111-1111-1111-111111111111"

	newClient := func(
		createNs string,
		updateNs string,
		available bool,
	) *inject.AppServiceClient {
		return &inject.AppServiceClient{
			GetOrganizationNamespaceAvailabilityFunc: func(
				ctx context.Context, in *apppb.GetOrganizationNamespaceAvailabilityRequest, opts ...grpc.CallOption,
			) (*apppb.GetOrganizationNamespaceAvailabilityResponse, error) {
				test.That(t, in.GetPublicNamespace(), test.ShouldEqual, "otf")
				return &apppb.GetOrganizationNamespaceAvailabilityResponse{Available: available}, nil
			},
			CreateOrganizationFunc: func(
				ctx context.Context, in *apppb.CreateOrganizationRequest, opts ...grpc.CallOption,
			) (*apppb.CreateOrganizationResponse, error) {
				test.That(t, in.GetName(), test.ShouldEqual, "OTF")
				return &apppb.CreateOrganizationResponse{
					Organization: &apppb.Organization{Id: orgID, Name: "OTF", PublicNamespace: createNs},
				}, nil
			},
			UpdateOrganizationFunc: func(
				ctx context.Context, in *apppb.UpdateOrganizationRequest, opts ...grpc.CallOption,
			) (*apppb.UpdateOrganizationResponse, error) {
				test.That(t, in.GetOrganizationId(), test.ShouldEqual, orgID)
				test.That(t, in.GetPublicNamespace(), test.ShouldEqual, "otf")
				return &apppb.UpdateOrganizationResponse{
					Organization: &apppb.Organization{Id: orgID, Name: "OTF", PublicNamespace: updateNs},
				}, nil
			},
		}
	}

	t.Run("creates org and sets public namespace", func(t *testing.T) {
		cCtx, ac, out, errOut := setup(newClient("", "otf", true), nil, nil, nil, "token")
		err := ac.organizationsCreateAction(context.Background(), cCtx, organizationsCreateArgs{
			Name: "OTF", PublicNamespace: "otf",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(errOut.messages), test.ShouldEqual, 0)
		joined := strings.Join(out.messages, "")
		test.That(t, joined, test.ShouldContainSubstring, `Created organization "OTF"`)
		test.That(t, joined, test.ShouldContainSubstring, orgID)
		test.That(t, joined, test.ShouldContainSubstring, `Set public namespace "otf"`)
	})

	t.Run("skips update when create already set the namespace", func(t *testing.T) {
		asc := newClient("otf", "", true)
		asc.UpdateOrganizationFunc = func(
			ctx context.Context, in *apppb.UpdateOrganizationRequest, opts ...grpc.CallOption,
		) (*apppb.UpdateOrganizationResponse, error) {
			t.Fatal("UpdateOrganization should not be called")
			return nil, nil
		}
		cCtx, ac, out, _ := setup(asc, nil, nil, nil, "token")
		err := ac.organizationsCreateAction(context.Background(), cCtx, organizationsCreateArgs{
			Name: "OTF", PublicNamespace: "otf",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, strings.Join(out.messages, ""), test.ShouldContainSubstring, `Public namespace "otf" is ready`)
	})

	t.Run("rejects taken namespace before creating", func(t *testing.T) {
		asc := newClient("", "otf", false)
		asc.CreateOrganizationFunc = func(
			ctx context.Context, in *apppb.CreateOrganizationRequest, opts ...grpc.CallOption,
		) (*apppb.CreateOrganizationResponse, error) {
			t.Fatal("CreateOrganization should not be called")
			return nil, nil
		}
		cCtx, ac, _, _ := setup(asc, nil, nil, nil, "token")
		err := ac.organizationsCreateAction(context.Background(), cCtx, organizationsCreateArgs{
			Name: "OTF", PublicNamespace: "otf",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, `public namespace "otf" is already taken`)
	})

	t.Run("rejects invalid namespace", func(t *testing.T) {
		cCtx, ac, _, _ := setup(newClient("", "otf", true), nil, nil, nil, "token")
		err := ac.organizationsCreateAction(context.Background(), cCtx, organizationsCreateArgs{
			Name: "OTF", PublicNamespace: "OTF",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "lowercase letters")
	})

	t.Run("requires flags when not interactive", func(t *testing.T) {
		cCtx, ac, _, _ := setup(newClient("", "otf", true), nil, nil, nil, "token")
		err := ac.organizationsCreateAction(context.Background(), cCtx, organizationsCreateArgs{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "--name")
		test.That(t, err.Error(), test.ShouldContainSubstring, "--public-namespace")
	})
}

func TestOrgPublicNamespacePattern(t *testing.T) {
	t.Parallel()
	test.That(t, orgPublicNamespacePattern.MatchString("otf"), test.ShouldBeTrue)
	test.That(t, orgPublicNamespacePattern.MatchString("my-namespace"), test.ShouldBeTrue)
	test.That(t, orgPublicNamespacePattern.MatchString("1otf"), test.ShouldBeTrue)
	test.That(t, orgPublicNamespacePattern.MatchString("a--b"), test.ShouldBeTrue)
	test.That(t, orgPublicNamespacePattern.MatchString("a1"), test.ShouldBeTrue)
	test.That(t, orgPublicNamespacePattern.MatchString(strings.Repeat("c", 39)), test.ShouldBeTrue)
	test.That(t, orgPublicNamespacePattern.MatchString("a"), test.ShouldBeFalse)
	test.That(t, orgPublicNamespacePattern.MatchString("9"), test.ShouldBeFalse)
	test.That(t, orgPublicNamespacePattern.MatchString(strings.Repeat("c", 40)), test.ShouldBeFalse)
	test.That(t, orgPublicNamespacePattern.MatchString("OTF"), test.ShouldBeFalse)
	test.That(t, orgPublicNamespacePattern.MatchString("-otf"), test.ShouldBeFalse)
	test.That(t, orgPublicNamespacePattern.MatchString("otf-"), test.ShouldBeFalse)
	test.That(t, orgPublicNamespacePattern.MatchString("-"), test.ShouldBeFalse)
	test.That(t, orgPublicNamespacePattern.MatchString(""), test.ShouldBeFalse)
	test.That(t, orgPublicNamespacePattern.MatchString("foo_bar"), test.ShouldBeFalse)
	test.That(t, orgPublicNamespacePattern.MatchString("foo.bar"), test.ShouldBeFalse)
	test.That(t, orgPublicNamespacePattern.MatchString("foo bar"), test.ShouldBeFalse)
	test.That(t, orgPublicNamespacePattern.MatchString("foo/bar"), test.ShouldBeFalse)
}
