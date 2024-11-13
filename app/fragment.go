package app

import (
	"fmt"

	pb "go.viam.com/api/app/v1"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Fragment struct {
	ID                string
	Name              string
	Fragment          *map[string]interface{}
	OrganizationOwner string
	Public            bool
	CreatedOn         *timestamppb.Timestamp
	OrganizationName  string
	RobotPartCount    int32
	OrganizationCount int32
	OnlyUsedByOwner   bool
	Visibility        FragmentVisibility
	LastUpdated       *timestamppb.Timestamp
}

func ProtoToFragment(fragment *pb.Fragment) (*Fragment, error) {
	f := fragment.Fragment.AsMap()
	visibility, err := ProtoToFragmentVisibility(fragment.Visibility)
	if err != nil {
		return nil, err
	}
	return &Fragment{
		ID:                fragment.Id,
		Name:              fragment.Name,
		Fragment:          &f,
		OrganizationOwner: fragment.OrganizationOwner,
		Public:            fragment.Public,
		CreatedOn:         fragment.CreatedOn,
		OrganizationName:  fragment.OrganizationName,
		RobotPartCount:    fragment.RobotPartCount,
		OrganizationCount: fragment.OrganizationCount,
		OnlyUsedByOwner:   fragment.OnlyUsedByOwner,
		Visibility:        visibility,
		LastUpdated:       fragment.LastUpdated,
	}, nil
}

func FragmentToProto(fragment *Fragment) (*pb.Fragment, error) {
	f, err := protoutils.StructToStructPb(fragment.Fragment)
	if err != nil {
		return nil, err
	}
	visibility, err := FragmentVisibilityToProto(fragment.Visibility)
	if err != nil {
		return nil, err
	}
	return &pb.Fragment{
		Id:                fragment.ID,
		Name:              fragment.Name,
		Fragment:          f,
		OrganizationOwner: fragment.OrganizationOwner,
		Public:            fragment.Public,
		CreatedOn:         fragment.CreatedOn,
		OrganizationName:  fragment.OrganizationName,
		RobotPartCount:    fragment.RobotPartCount,
		OrganizationCount: fragment.OrganizationCount,
		OnlyUsedByOwner:   fragment.OnlyUsedByOwner,
		Visibility:        visibility,
		LastUpdated:       fragment.LastUpdated,
	}, nil
}

type FragmentVisibility int32

const (
	FragmentVisibilityUnspecified    FragmentVisibility = 0
	FragmentVisibilityPrivate        FragmentVisibility = 1
	FragmentVisibilityPublic         FragmentVisibility = 2
	FragmentVisibilityPublicUnlisted FragmentVisibility = 3
)

func ProtoToFragmentVisibility(visibility pb.FragmentVisibility) (FragmentVisibility, error) {
	switch visibility {
	case pb.FragmentVisibility_FRAGMENT_VISIBILITY_UNSPECIFIED:
		return FragmentVisibilityUnspecified, nil
	case pb.FragmentVisibility_FRAGMENT_VISIBILITY_PRIVATE:
		return FragmentVisibilityPrivate, nil
	case pb.FragmentVisibility_FRAGMENT_VISIBILITY_PUBLIC:
		return FragmentVisibilityPublic, nil
	case pb.FragmentVisibility_FRAGMENT_VISIBILITY_PUBLIC_UNLISTED:
		return FragmentVisibilityPublicUnlisted, nil
	default:
		return 0, fmt.Errorf("uknown fragment visibililty: %v", visibility)
	}
}

func FragmentVisibilityToProto(visibility FragmentVisibility) (pb.FragmentVisibility, error) {
	switch visibility {
	case FragmentVisibilityUnspecified:
		return pb.FragmentVisibility_FRAGMENT_VISIBILITY_UNSPECIFIED, nil
	case FragmentVisibilityPrivate:
		return pb.FragmentVisibility_FRAGMENT_VISIBILITY_PRIVATE, nil
	case FragmentVisibilityPublic:
		return pb.FragmentVisibility_FRAGMENT_VISIBILITY_PUBLIC, nil
	case FragmentVisibilityPublicUnlisted:
		return pb.FragmentVisibility_FRAGMENT_VISIBILITY_PUBLIC_UNLISTED, nil
	default:
		return 0, fmt.Errorf("unknown fragment visibility: %v", visibility)
	}
}

type FragmentHistoryEntry struct {
	Fragment string
	EditedOn *timestamppb.Timestamp
	Old      *Fragment
	EditedBy *AuthenticatorInfo
}

func ProtoToFragmentHistoryEntry(entry *pb.FragmentHistoryEntry) (*FragmentHistoryEntry, error) {
	old, err := ProtoToFragment(entry.Old)
	if err != nil {
		return nil, err
	}
	editedBy, err := ProtoToAuthenticatorInfo(entry.EditedBy)
	if err != nil {
		return nil, err
	}
	return &FragmentHistoryEntry{
		Fragment: entry.Fragment,
		EditedOn: entry.EditedOn,
		Old:      old,
		EditedBy: editedBy,
	}, nil
}

func FragmentHistoryEntryToProto(entry *FragmentHistoryEntry) (*pb.FragmentHistoryEntry, error) {
	old, err := FragmentToProto(entry.Old)
	if err != nil {
		return nil, err
	}
	editedBy, err := AuthenticatorInfoToProto(entry.EditedBy)
	if err != nil {
		return nil, err
	}
	return &pb.FragmentHistoryEntry{
		Fragment: entry.Fragment,
		EditedOn: entry.EditedOn,
		Old:      old,
		EditedBy: editedBy,
	}, nil
}
