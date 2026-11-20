package packages

import (
	"testing"
	"time"

	packagespb "go.viam.com/api/app/packages/v1"
	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/config"
)

func TestPackageStatusProtoRoundTrip(t *testing.T) {
	for _, state := range []PackageState{
		PackageStateUnknown,
		PackageStateDownloading,
		PackageStateLoading,
		PackageStateFirstRun,
		PackageStateReady,
		PackageStateFailed,
	} {
		status := PackageStatus{
			Name:                "package",
			Type:                config.PackageTypeModule,
			State:               state,
			Error:               "error",
			LastUpdated:         time.Unix(1700000000, 123).UTC(),
			Version:             "1.2.3",
			BytesDownloaded:     1024,
			TotalBytes:          4096,
			ConsecutiveFailures: 2,
		}

		protoStatus, err := PackageStatusToProto(status)
		test.That(t, err, test.ShouldBeNil)
		converted, err := PackageStatusFromProto(protoStatus)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, converted, test.ShouldResemble, status)
	}
}

func TestPackageStatusFromProtoEdgeCases(t *testing.T) {
	t.Run("nil status", func(t *testing.T) {
		status, err := PackageStatusFromProto(nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, status, test.ShouldResemble, PackageStatus{})
	})

	t.Run("missing timestamp", func(t *testing.T) {
		status, err := PackageStatusFromProto(&robotpb.PackageStatus{
			Name:  "package",
			Type:  packagespb.PackageType_PACKAGE_TYPE_MODULE,
			State: robotpb.PackageStatus_STATE_READY,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status.LastUpdated.IsZero(), test.ShouldBeTrue)
	})

	t.Run("unsupported enums and invalid timestamp", func(t *testing.T) {
		status, err := PackageStatusFromProto(&robotpb.PackageStatus{
			Name:        "package",
			Type:        packagespb.PackageType(100),
			State:       robotpb.PackageStatus_State(100),
			LastUpdated: &timestamppb.Timestamp{Seconds: 253402300800},
			Version:     "1.2.3",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, status, test.ShouldResemble, PackageStatus{
			Name:    "package",
			Version: "1.2.3",
		})
	})
}

func TestPackageStatusToProtoInvalidValues(t *testing.T) {
	protoStatus, err := PackageStatusToProto(PackageStatus{
		Name:        "package",
		Type:        config.PackageType("unsupported"),
		State:       PackageState(100),
		LastUpdated: time.Date(10000, time.January, 1, 0, 0, 0, 0, time.UTC),
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, protoStatus.GetType(), test.ShouldEqual, packagespb.PackageType_PACKAGE_TYPE_UNSPECIFIED)
	test.That(t, protoStatus.GetState(), test.ShouldEqual, robotpb.PackageStatus_STATE_UNSPECIFIED)
	test.That(t, protoStatus.GetLastUpdated().CheckValid(), test.ShouldNotBeNil)
}

func TestPackageStatusFirstRunFailures(t *testing.T) {
	v1 := config.PackageConfig{Name: "package", Type: config.PackageTypeModule, Version: "v1"}
	status := newPackageStatus(v1, PackageStateReady, "", nil)

	status.setFirstRunState(PackageStateFirstRun, "")
	status.setFirstRunState(PackageStateFailed, "boom")
	test.That(t, status.ConsecutiveFailures, test.ShouldEqual, 1)

	// The error survives the retry, including a resync of the same version.
	status = newPackageStatus(v1, PackageStateLoading, "", status)
	status.setFirstRunState(PackageStateFirstRun, "")
	test.That(t, status.Error, test.ShouldEqual, "boom")
	test.That(t, status.ConsecutiveFailures, test.ShouldEqual, 1)

	status.setFirstRunState(PackageStateFailed, "boom again")
	test.That(t, status.Error, test.ShouldEqual, "boom again")
	test.That(t, status.ConsecutiveFailures, test.ShouldEqual, 2)

	t.Run("new version resets", func(t *testing.T) {
		v2 := v1
		v2.Version = "v2"
		fresh := newPackageStatus(v2, PackageStateDownloading, "", status)
		test.That(t, fresh.Error, test.ShouldBeEmpty)
		test.That(t, fresh.ConsecutiveFailures, test.ShouldEqual, 0)
	})

	t.Run("ready resets", func(t *testing.T) {
		ready := *status
		ready.setFirstRunState(PackageStateReady, "")
		test.That(t, ready.Error, test.ShouldBeEmpty)
		test.That(t, ready.ConsecutiveFailures, test.ShouldEqual, 0)
	})
}
