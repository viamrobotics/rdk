package resource

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

// Name represents a known component/service representation of a robot.
type Name struct {
	API    API
	Remote string
	Name   string
}

// NewName creates a new resource Name.
func NewName(api API, name string) Name {
	r := strings.Split(name, ":")
	remote := strings.Join(r[0:len(r)-1], ":")
	nameIdent := r[len(r)-1]
	return Name{
		API:    api,
		Name:   nameIdent,
		Remote: remote,
	}
}

// UnmarshalJSON unmarshals a resource name from a string.
func (n *Name) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	newN, err := NewFromString(s)
	if err != nil {
		return err
	}
	*n = newN
	return nil
}

// newRemoteName creates a new Name for a resource attached to a remote.
func newRemoteName(remoteName string, api API, name string) Name {
	n := NewName(api, name)
	n.Remote = remoteName
	return n
}

// NewFromString creates a new Name based on a fully qualified resource name string passed in.
func NewFromString(resourceName string) (Name, error) {
	if !resRegexValidator.MatchString(resourceName) {
		return Name{}, errors.Errorf("string %q is not a valid resource name", resourceName)
	}
	matches := resRegexValidator.FindStringSubmatch(resourceName)
	rAPIParts := strings.Split(matches[1], ":")
	remoteName := matches[2]
	if len(remoteName) > 0 {
		remoteName = remoteName[:len(remoteName)-1]
	}
	api := APINamespace(rAPIParts[0]).WithType(rAPIParts[1]).WithSubtype(rAPIParts[2])
	return newRemoteName(remoteName, api, matches[3]), nil
}

// PrependRemote returns a Name with a remote prepended.
func (n Name) PrependRemote(remoteName string) Name {
	if remoteName == "" {
		return n
	}
	if len(n.Remote) > 0 && strings.Count(remoteName, ":") == 0 && strings.Count(n.String(), ":") == 0 {
		remoteName = strings.Join([]string{remoteName, n.Remote}, ":")
	}
	return newRemoteName(
		remoteName,
		n.API,
		n.Name)
}

// PopRemote pop the first remote from a Name (if any) and returns the new Name.
func (n Name) PopRemote() Name {
	if n.Remote == "" {
		return n
	}
	remotes := strings.Split(n.Remote, ":")
	return newRemoteName(
		strings.Join(remotes[1:], ":"),
		n.API,
		n.Name)
}

// ContainsRemoteNames return true if the resource is a remote resource.
func (n Name) ContainsRemoteNames() bool {
	return len(n.Remote) > 0
}

// AsNamed is a helper to let this name return itself as a basic resource that does
// nothing.
func (n Name) AsNamed() Named {
	return selfNamed{n}
}

// RemoveRemoteName returns a new name with remote removed.
func RemoveRemoteName(n Name) Name {
	tempName := NewName(n.API, n.Name)
	tempName.Remote = ""
	return tempName
}

// ShortName returns the short name on Name n in the form of <remote>:<name>.
func (n Name) ShortName() string {
	nameR := n.Name
	if n.Remote != "" {
		nameR = fmt.Sprintf("%s:%s", n.Remote, nameR)
	}
	return nameR
}

// Validate ensures that important fields exist and are valid.
func (n Name) Validate() error {
	if n.Name == "" {
		return errors.New("name field for resource is empty")
	}
	if err := n.API.Validate(); err != nil {
		return err
	}
	if err := ContainsReservedCharacter(n.Name); err != nil {
		return err
	}
	return nil
}

// String returns the fully qualified name for the resource.
func (n Name) String() string {
	name := n.API.String()
	if n.Remote != "" {
		name = fmt.Sprintf("%s/%s:%s", name, n.Remote, n.Name)
	} else {
		name = fmt.Sprintf("%s/%s", name, n.Name)
	}
	return name
}
