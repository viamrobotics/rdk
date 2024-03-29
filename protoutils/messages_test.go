package protoutils

import (
	"math"
	"strconv"
	"testing"

	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/wrapperspb"
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
	partId := "abcde"
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
			resource.NewNameWithPartID(api, name, partId),
			&commonpb.ResourceName{
				Namespace:     string(api.Type.Namespace),
				Type:          api.Type.Name,
				Subtype:       api.SubtypeName,
				Name:          name,
				MachinePartId: &partId,
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
			resource.NewNameWithPartID(api, name, partId).PrependRemote(remoteName),
			&commonpb.ResourceName{
				Namespace:     string(api.Type.Namespace),
				Type:          api.Type.Name,
				Subtype:       api.SubtypeName,
				Name:          resource.NewNameWithPartID(api, name, partId).PrependRemote(remoteName).ShortName(),
				MachinePartId: &partId,
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
	partId := "abcde"
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
				MachinePartId: &partId,
			},
			resource.NewNameWithPartID(api, name, partId),
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
				Name:          resource.NewNameWithPartID(api, name, partId).PrependRemote(remoteName).ShortName(),
				MachinePartId: &partId,
			},
			resource.NewNameWithPartID(api, name, partId).PrependRemote(remoteName),
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := ResourceNameFromProto(tc.Name)
			test.That(t, observed, test.ShouldResemble, tc.Expected)
		})
	}
}
