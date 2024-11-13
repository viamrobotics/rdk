package app

import (
	"fmt"

	pb "go.viam.com/api/app/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Fragment stores the information of a fragment.
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

func fragmentFromProto(fragment *pb.Fragment) (*Fragment, error) {
	f := fragment.Fragment.AsMap()
	visibility, err := fragmentVisibilityFromProto(fragment.Visibility)
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

// FragmentVisibility specifies the kind of visibility a fragment has.
type FragmentVisibility int32

const (
	// FragmentVisibilityUnspecified is an unspecified visibility.
	FragmentVisibilityUnspecified FragmentVisibility = 0
	// FragmentVisibilityPrivate restricts access to a fragment to its organization.
	FragmentVisibilityPrivate FragmentVisibility = 1
	// FragmentVisibilityPublic allows the fragment to be accessible to everyone.
	FragmentVisibilityPublic FragmentVisibility = 2
	// FragmentVisibilityPublicUnlisted allows the fragment to be accessible to everyone but is hidden from public listings like it is private.
	FragmentVisibilityPublicUnlisted FragmentVisibility = 3
)

func fragmentVisibilityFromProto(visibility pb.FragmentVisibility) (FragmentVisibility, error) {
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

func fragmentVisibilityToProto(visibility FragmentVisibility) (pb.FragmentVisibility, error) {
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

// FragmentHistoryEntry is an entry of a fragment's history.
type FragmentHistoryEntry struct {
	Fragment string
	EditedOn *timestamppb.Timestamp
	Old      *Fragment
	EditedBy *AuthenticatorInfo
}

func fragmentHistoryEntryFromProto(entry *pb.FragmentHistoryEntry) (*FragmentHistoryEntry, error) {
	old, err := fragmentFromProto(entry.Old)
	if err != nil {
		return nil, err
	}
	editedBy, err := authenticatorInfoFromProto(entry.EditedBy)
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
