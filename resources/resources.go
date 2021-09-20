// Package resources contains a Metadata type that can be used to hold information about a robot's components and services.
package resources

import (
	"sync"

	"github.com/go-errors/errors"
	"go.viam.com/core/robot"

	"github.com/google/uuid"
)

// Define a few known constants
const (
	ResourceNamespaceCore   = "core"
	ResourceTypeComponent   = "component"
	ResourceTypeService     = "service"
	ResourceSubtypeArm      = "arm"
	ResourceSubtypeBase     = "base"
	ResourceSubtypeBoard    = "board"
	ResourceSubtypeCamera   = "camera"
	ResourceSubtypeFunction = "function"
	ResourceSubtypeGripper  = "gripper"
	ResourceSubtypeLidar    = "lidar"
	ResourceSubtypeMetadata = "metadata"
	ResourceSubtypeRemote   = "remote"
	ResourceSubtypeSensor   = "sensor"
)

type Resource struct {
	Uuid      string
	Namespace string
	Type      string
	Subtype   string
	Name      string
}

// Validate ensures that important fields exist and are valid
func (r Resource) Validate() error {
	if _, err := uuid.Parse(r.Uuid); err != nil {
		return errors.New("uuid field for resource missing or invalid.")
	}
	if r.Namespace == "" {
		return errors.New("namespace field for resource missing or invalid.")
	}
	if r.Type == "" {
		return errors.New("type field for resource missing or invalid.")
	}
	if r.Subtype == "" {
		return errors.New("subtype field for resource missing or invalid.")
	}
	return nil
}

type Resources struct {
	mu        sync.Mutex
	resources []Resource
}

// Creates and populate Resources given a robot.
func Init(r robot.Robot) (*Resources, error) {
	res := New()

	if err := res.Populate(r); err != nil {
		return nil, err
	}
	return &res, nil
}

// New creates a new Resources struct and initializes the resource list with a metadata service.
func New() Resources {
	resources := []Resource{
		{
			Uuid:      uuid.NewString(),
			Namespace: ResourceNamespaceCore,
			Type:      ResourceTypeService,
			Subtype:   ResourceSubtypeMetadata,
			Name:      "",
		},
	}

	return Resources{resources: resources}
}

// Populate Resources given a robot.
func (r *Resources) Populate(robot robot.Robot) error {
	// TODO: Currently just a placeholder implementation, this should be rewritten once robot/parts have more metadata about themselves
	for _, name := range robot.ArmNames() {
		err := r.AddResource(
			Resource{
				Uuid:      uuid.NewString(),
				Namespace: ResourceNamespaceCore, // can be non-core as well
				Type:      ResourceTypeComponent,
				Subtype:   ResourceSubtypeArm,
				Name:      name,
			},
		)
		if err != nil {
			return err
		}

	}
	for _, name := range robot.BaseNames() {
		err := r.AddResource(
			Resource{
				Uuid:      uuid.NewString(),
				Namespace: ResourceNamespaceCore,
				Type:      ResourceTypeComponent,
				Subtype:   ResourceSubtypeBase,
				Name:      name,
			},
		)
		if err != nil {
			return err
		}
	}
	for _, name := range robot.BoardNames() {
		err := r.AddResource(
			Resource{
				Uuid:      uuid.NewString(),
				Namespace: ResourceNamespaceCore,
				Type:      ResourceTypeComponent,
				Subtype:   ResourceSubtypeBoard,
				Name:      name,
			},
		)
		if err != nil {
			return err
		}
	}
	for _, name := range robot.CameraNames() {
		err := r.AddResource(
			Resource{
				Uuid:      uuid.NewString(),
				Namespace: ResourceNamespaceCore,
				Type:      ResourceTypeComponent,
				Subtype:   ResourceSubtypeCamera,
				Name:      name,
			},
		)
		if err != nil {
			return err
		}
	}
	for _, name := range robot.FunctionNames() {
		err := r.AddResource(
			Resource{
				Uuid:      uuid.NewString(),
				Namespace: ResourceNamespaceCore,
				Type:      ResourceTypeService,
				Subtype:   ResourceSubtypeFunction,
				Name:      name,
			},
		)
		if err != nil {
			return err
		}
	}
	for _, name := range robot.GripperNames() {
		err := r.AddResource(
			Resource{
				Uuid:      uuid.NewString(),
				Namespace: ResourceNamespaceCore,
				Type:      ResourceTypeComponent,
				Subtype:   ResourceSubtypeGripper,
				Name:      name,
			},
		)
		if err != nil {
			return err
		}
	}
	for _, name := range robot.LidarNames() {
		err := r.AddResource(
			Resource{
				Uuid:      uuid.NewString(),
				Namespace: ResourceNamespaceCore,
				Type:      ResourceTypeComponent,
				Subtype:   ResourceSubtypeLidar,
				Name:      name,
			},
		)
		if err != nil {
			return err
		}
	}
	for _, name := range robot.RemoteNames() {
		err := r.AddResource(
			Resource{
				Uuid:      uuid.NewString(),
				Namespace: ResourceNamespaceCore,
				Type:      ResourceTypeComponent,
				Subtype:   ResourceSubtypeRemote,
				Name:      name,
			},
		)
		if err != nil {
			return err
		}
	}
	for _, name := range robot.SensorNames() {
		err := r.AddResource(
			Resource{
				Uuid:      uuid.NewString(),
				Namespace: ResourceNamespaceCore,
				Type:      ResourceTypeComponent,
				Subtype:   ResourceSubtypeSensor,
				Name:      name,
			},
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// Resources returns the list of resources.
func (r *Resources) GetResources() []Resource {
	return r.resources
}

// AddResource adds an additional resource to the list. Cannot add another metadata service
func (r *Resources) AddResource(resource Resource) error {
	if err := resource.Validate(); err != nil {
		return errors.Errorf("Unable to add resource: %s", err.Error())
	}

	idx := -1
	for i := range r.resources {
		if r.resources[i].Uuid == resource.Uuid {
			idx = i
			break
		}
	}
	if idx != -1 {
		return errors.Errorf("Resource with uuid %s already exists and cannot be added again.", resource.Uuid)
	}
	if resource.Subtype == ResourceSubtypeMetadata {
		return errors.New("Unable to add a resource with a metadata subtype.")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.resources = append(r.resources, resource)
	return nil
}

// RemoveResource removes resource from the list.
func (r *Resources) RemoveResource(resource Resource) error {
	if err := resource.Validate(); err != nil {
		return errors.Errorf("Invalid resource to find and remove: %s", err.Error())
	}

	idx := -1
	for i := range r.resources {
		if r.resources[i].Uuid == resource.Uuid {
			idx = i
			break
		}
	}
	if idx == -1 {
		return errors.Errorf("Unable to find and remove resource with uuid %s.", resource.Uuid)
	}
	if resource.Subtype == ResourceSubtypeMetadata {
		return errors.New("Unable to remove resource with a metadata subtype.")
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	r.resources = append(r.resources[:idx], r.resources[idx+1:]...)
	return nil
}

// ClearResources clears all resources except the metadata service from the resource list
func (r *Resources) ClearResources() error {
	idx := -1
	for i := range r.resources {
		if r.resources[i].Subtype == ResourceSubtypeMetadata {
			idx = i
			break
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if idx == -1 {
		r.resources = []Resource{
			{
				Uuid:      uuid.NewString(),
				Namespace: ResourceNamespaceCore,
				Type:      ResourceTypeService,
				Subtype:   ResourceSubtypeMetadata,
				Name:      "",
			},
		}
	} else {
		r.resources = r.resources[idx : idx+1]
	}
	return nil
}
