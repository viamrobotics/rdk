// Package metadata contains a metadata type that can be used to hold information about a robot's components and services.
package metadata

import (
	"sync"

	"github.com/go-errors/errors"

	"go.viam.com/core/resource"
	"go.viam.com/core/robot"

	"github.com/google/uuid"
)

// Metadata keeps track of all resources associated with a robot.
type Metadata struct {
	mu        sync.Mutex
	resources []resource.ResourceName
}

// NewFromRobot Creates and populate Metadata given a robot.
func NewFromRobot(r robot.Robot) (*Metadata, error) {
	res := New()

	if err := res.Populate(r); err != nil {
		return nil, err
	}
	return &res, nil
}

// New creates a new Metadata struct and initializes the resource list with a metadata service.
func New() Metadata {
	resources := []resource.ResourceName{
		{
			UUID:      uuid.NewString(),
			Namespace: resource.ResourceNamespaceCore,
			Type:      resource.ResourceTypeService,
			Subtype:   resource.ResourceSubtypeMetadata,
			Name:      "",
		},
	}

	return Metadata{resources: resources}
}

// Populate populates Metadata given a robot.
func (m *Metadata) Populate(robot robot.Robot) error {
	// TODO: Currently just a placeholder implementation, this should be rewritten once robot/parts have more metadata about themselves
	for _, name := range robot.ArmNames() {
		err := m.Add(
			resource.ResourceName{
				UUID:      uuid.NewString(),
				Namespace: resource.ResourceNamespaceCore, // can be non-core as well
				Type:      resource.ResourceTypeComponent,
				Subtype:   resource.ResourceSubtypeArm,
				Name:      name,
			},
		)
		if err != nil {
			return err
		}

	}
	for _, name := range robot.BaseNames() {
		err := m.Add(
			resource.ResourceName{
				UUID:      uuid.NewString(),
				Namespace: resource.ResourceNamespaceCore,
				Type:      resource.ResourceTypeComponent,
				Subtype:   resource.ResourceSubtypeBase,
				Name:      name,
			},
		)
		if err != nil {
			return err
		}
	}
	for _, name := range robot.BoardNames() {
		err := m.Add(
			resource.ResourceName{
				UUID:      uuid.NewString(),
				Namespace: resource.ResourceNamespaceCore,
				Type:      resource.ResourceTypeComponent,
				Subtype:   resource.ResourceSubtypeBoard,
				Name:      name,
			},
		)
		if err != nil {
			return err
		}
	}
	for _, name := range robot.CameraNames() {
		err := m.Add(
			resource.ResourceName{
				UUID:      uuid.NewString(),
				Namespace: resource.ResourceNamespaceCore,
				Type:      resource.ResourceTypeComponent,
				Subtype:   resource.ResourceSubtypeCamera,
				Name:      name,
			},
		)
		if err != nil {
			return err
		}
	}
	for _, name := range robot.FunctionNames() {
		err := m.Add(
			resource.ResourceName{
				UUID:      uuid.NewString(),
				Namespace: resource.ResourceNamespaceCore,
				Type:      resource.ResourceTypeService,
				Subtype:   resource.ResourceSubtypeFunction,
				Name:      name,
			},
		)
		if err != nil {
			return err
		}
	}
	for _, name := range robot.GripperNames() {
		err := m.Add(
			resource.ResourceName{
				UUID:      uuid.NewString(),
				Namespace: resource.ResourceNamespaceCore,
				Type:      resource.ResourceTypeComponent,
				Subtype:   resource.ResourceSubtypeGripper,
				Name:      name,
			},
		)
		if err != nil {
			return err
		}
	}
	for _, name := range robot.LidarNames() {
		err := m.Add(
			resource.ResourceName{
				UUID:      uuid.NewString(),
				Namespace: resource.ResourceNamespaceCore,
				Type:      resource.ResourceTypeComponent,
				Subtype:   resource.ResourceSubtypeLidar,
				Name:      name,
			},
		)
		if err != nil {
			return err
		}
	}
	for _, name := range robot.RemoteNames() {
		err := m.Add(
			resource.ResourceName{
				UUID:      uuid.NewString(),
				Namespace: resource.ResourceNamespaceCore,
				Type:      resource.ResourceTypeComponent,
				Subtype:   resource.ResourceSubtypeRemote,
				Name:      name,
			},
		)
		if err != nil {
			return err
		}
	}
	for _, name := range robot.SensorNames() {
		err := m.Add(
			resource.ResourceName{
				UUID:      uuid.NewString(),
				Namespace: resource.ResourceNamespaceCore,
				Type:      resource.ResourceTypeComponent,
				Subtype:   resource.ResourceSubtypeSensor,
				Name:      name,
			},
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// All returns the list of resources.
func (m *Metadata) All() []resource.ResourceName {
	return m.resources
}

// Add adds an additional resource to the list.
func (m *Metadata) Add(res resource.ResourceName) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := res.Validate(); err != nil {
		return errors.Errorf("unable to add resource: %s", err.Error())
	}

	idx := -1
	for i := range m.resources {
		if m.resources[i].UUID == res.UUID {
			idx = i
			break
		}
	}
	if idx != -1 {
		return errors.Errorf("resource with uuid %s already exists and cannot be added again", res.UUID)
	}

	m.resources = append(m.resources, res)
	return nil
}

// Remove removes resource from the list.
func (m *Metadata) Remove(res resource.ResourceName) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := res.Validate(); err != nil {
		return errors.Errorf("invalid resource to find and remove: %s", err.Error())
	}

	idx := -1
	for i := range m.resources {
		if m.resources[i].UUID == res.UUID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return errors.Errorf("unable to find and remove resource with uuid %s", res.UUID)
	}
	if res.Subtype == resource.ResourceSubtypeMetadata {
		return errors.New("unable to remove resource with a metadata subtype")
	}

	m.resources = append(m.resources[:idx], m.resources[idx+1:]...)
	return nil
}
