package protoutils

import (
	"math"
	"strconv"
	"testing"

	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"go.viam.com/rdk/resource"
)

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

func TestResourceNameToProto(t *testing.T) {
	api := resource.NewAPI("foo", "bar", "baz")
	name := "hello"
	remoteName := "remote1"
	partID := "abcde"
	for _, tc := range []struct {
		TestName string
		Name     resource.Name
		Expected *commonpb.ResourceName
	}{
		{
			"name",
			resource.NewName(api, name),
			&commonpb.ResourceName{
				Namespace: string(api.Type.Namespace),
				Type:      api.Type.Name,
				Subtype:   api.SubtypeName,
				Name:      name,
			},
		},
		{
			"name with part id",
			resource.NewNameWithPartID(api, name, partID),
			&commonpb.ResourceName{
				Namespace:     string(api.Type.Namespace),
				Type:          api.Type.Name,
				Subtype:       api.SubtypeName,
				Name:          name,
				MachinePartId: &partID,
			},
		},
		{
			"remote name",
			resource.NewName(api, name).PrependRemote(remoteName),
			&commonpb.ResourceName{
				Namespace: string(api.Type.Namespace),
				Type:      api.Type.Name,
				Subtype:   api.SubtypeName,
				Name:      resource.NewName(api, name).PrependRemote(remoteName).ShortName(),
			},
		},
		{
			"remote name with part id",
			resource.NewNameWithPartID(api, name, partID).PrependRemote(remoteName),
			&commonpb.ResourceName{
				Namespace:     string(api.Type.Namespace),
				Type:          api.Type.Name,
				Subtype:       api.SubtypeName,
				Name:          resource.NewNameWithPartID(api, name, partID).PrependRemote(remoteName).ShortName(),
				MachinePartId: &partID,
			},
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := ResourceNameToProto(tc.Name)
			test.That(t, observed, test.ShouldResemble, tc.Expected)
		})
	}
}

func TestResourceNameFromProto(t *testing.T) {
	api := resource.NewAPI("foo", "bar", "baz")
	name := "hello"
	remoteName := "remote1"
	partID := "abcde"
	for _, tc := range []struct {
		TestName string
		Name     *commonpb.ResourceName
		Expected resource.Name
	}{
		{
			"name",
			&commonpb.ResourceName{
				Namespace: string(api.Type.Namespace),
				Type:      api.Type.Name,
				Subtype:   api.SubtypeName,
				Name:      name,
			},
			resource.NewName(api, name),
		},
		{
			"name with part id",
			&commonpb.ResourceName{
				Namespace:     string(api.Type.Namespace),
				Type:          api.Type.Name,
				Subtype:       api.SubtypeName,
				Name:          name,
				MachinePartId: &partID,
			},
			resource.NewNameWithPartID(api, name, partID),
		},
		{
			"remote name",
			&commonpb.ResourceName{
				Namespace: string(api.Type.Namespace),
				Type:      api.Type.Name,
				Subtype:   api.SubtypeName,
				Name:      resource.NewName(api, name).PrependRemote(remoteName).ShortName(),
			},
			resource.NewName(api, name).PrependRemote(remoteName),
		},
		{
			"remote name with part id",
			&commonpb.ResourceName{
				Namespace:     string(api.Type.Namespace),
				Type:          api.Type.Name,
				Subtype:       api.SubtypeName,
				Name:          resource.NewNameWithPartID(api, name, partID).PrependRemote(remoteName).ShortName(),
				MachinePartId: &partID,
			},
			resource.NewNameWithPartID(api, name, partID).PrependRemote(remoteName),
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := ResourceNameFromProto(tc.Name)
			test.That(t, observed, test.ShouldResemble, tc.Expected)
		})
	}
}
