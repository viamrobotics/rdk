package protoutils

import (
	"math"
	"strconv"
	"testing"

	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"go.viam.com/rdk/cloud"
)

func TestMetadataFromProto(t *testing.T) {
	expected := cloud.Metadata{}
	observed := MetadataFromProto(nil)
	test.That(t, observed, test.ShouldResemble, expected)

	partID := "123"
	machineID := "abc"
	orgID := "456"
	locID := "def"

	samePartID := &robotpb.GetCloudMetadataResponse{
		RobotPartId:   partID,
		MachinePartId: partID,
		MachineId:     machineID,
		PrimaryOrgId:  orgID,
		LocationId:    locID,
	}
	expected = cloud.Metadata{
		MachinePartID: partID,
		MachineID:     machineID,
		PrimaryOrgID:  orgID,
		LocationID:    locID,
	}
	observed = MetadataFromProto(samePartID)
	test.That(t, observed, test.ShouldResemble, expected)

	robotPartID := "789"
	diffPartID := &robotpb.GetCloudMetadataResponse{
		RobotPartId:   robotPartID,
		MachinePartId: partID,
		MachineId:     machineID,
		PrimaryOrgId:  orgID,
		LocationId:    locID,
	}
	observed = MetadataFromProto(diffPartID)
	test.That(t, observed, test.ShouldResemble, expected)
}

func TestMetadataToProto(t *testing.T) {
	partID := "123"
	machineID := "abc"
	orgID := "456"
	locID := "def"

	metadata := cloud.Metadata{
		MachinePartID: partID,
		MachineID:     machineID,
		PrimaryOrgID:  orgID,
		LocationID:    locID,
	}
	expected := &robotpb.GetCloudMetadataResponse{
		RobotPartId:   partID,
		MachinePartId: partID,
		MachineId:     machineID,
		PrimaryOrgId:  orgID,
		LocationId:    locID,
	}
	observed := MetadataToProto(metadata)
	test.That(t, observed, test.ShouldResembleProto, expected)
}

func TestStringToAnyPB(t *testing.T) {
	anyVal, err := ConvertStringToAnyPB("12")
	test.That(t, err, test.ShouldBeNil)
	wrappedVal := wrapperspb.Int64(int64(12))
	test.That(t, anyVal.MessageIs(wrappedVal), test.ShouldBeTrue)

	anyVal, err = ConvertStringToAnyPB(strconv.Itoa(math.MaxInt))
	test.That(t, err, test.ShouldBeNil)
	wrappedVal = wrapperspb.Int64(math.MaxInt64)
	test.That(t, anyVal.MessageIs(wrappedVal), test.ShouldBeTrue)

	anyVal, err = ConvertStringToAnyPB("123.456")
	test.That(t, err, test.ShouldBeNil)
	wrappedVal1 := wrapperspb.Double(float64(123.456))
	test.That(t, anyVal.MessageIs(wrappedVal1), test.ShouldBeTrue)

	anyVal, err = ConvertStringToAnyPB(strconv.FormatUint(math.MaxUint64, 10))
	test.That(t, err, test.ShouldBeNil)
	wrappedVal2 := wrapperspb.UInt64(uint64(math.MaxUint64))
	test.That(t, anyVal.MessageIs(wrappedVal2), test.ShouldBeTrue)

	anyVal, err = ConvertStringToAnyPB("true")
	test.That(t, err, test.ShouldBeNil)
	wrappedVal3 := wrapperspb.Bool(true)
	test.That(t, anyVal.MessageIs(wrappedVal3), test.ShouldBeTrue)

	anyVal, err = ConvertStringToAnyPB("abcd")
	test.That(t, err, test.ShouldBeNil)
	wrappedVal4 := wrapperspb.String("abcd")
	test.That(t, anyVal.MessageIs(wrappedVal4), test.ShouldBeTrue)
}
