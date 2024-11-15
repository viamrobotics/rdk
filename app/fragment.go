package app

import (
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

func fragmentFromProto(fragment *pb.Fragment) (*Fragment) {
	f := fragment.Fragment.AsMap()
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
		Visibility:        fragmentVisibilityFromProto(fragment.Visibility),
		LastUpdated:       fragment.LastUpdated,
	}
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

func fragmentVisibilityFromProto(visibility pb.FragmentVisibility) FragmentVisibility {
	switch visibility {
	case pb.FragmentVisibility_FRAGMENT_VISIBILITY_PRIVATE:
		return FragmentVisibilityPrivate
	case pb.FragmentVisibility_FRAGMENT_VISIBILITY_PUBLIC:
		return FragmentVisibilityPublic
	case pb.FragmentVisibility_FRAGMENT_VISIBILITY_PUBLIC_UNLISTED:
		return FragmentVisibilityPublicUnlisted
	default:
		return FragmentVisibilityUnspecified
	}
}

func fragmentVisibilityToProto(visibility FragmentVisibility) (pb.FragmentVisibility) {
	switch visibility {
	case FragmentVisibilityPrivate:
		return pb.FragmentVisibility_FRAGMENT_VISIBILITY_PRIVATE
	case FragmentVisibilityPublic:
		return pb.FragmentVisibility_FRAGMENT_VISIBILITY_PUBLIC
	case FragmentVisibilityPublicUnlisted:
		return pb.FragmentVisibility_FRAGMENT_VISIBILITY_PUBLIC_UNLISTED
	default:
		return pb.FragmentVisibility_FRAGMENT_VISIBILITY_UNSPECIFIED
	}
}

// FragmentHistoryEntry is an entry of a fragment's history.
type FragmentHistoryEntry struct {
	Fragment string
	EditedOn *timestamppb.Timestamp
	Old      *Fragment
	EditedBy *AuthenticatorInfo
}

func fragmentHistoryEntryFromProto(entry *pb.FragmentHistoryEntry) (*FragmentHistoryEntry) {
	return &FragmentHistoryEntry{
		Fragment: entry.Fragment,
		EditedOn: entry.EditedOn,
		Old:      fragmentFromProto(entry.Old),
		EditedBy: authenticatorInfoFromProto(entry.EditedBy),
	}
}
