// Package service contains a service type that can be used to hold information about a robot's components and services.
package service

import (
	"context"
	"sync"

	"github.com/go-errors/errors"

	"go.viam.com/core/resource"
	"go.viam.com/core/robot"

	"github.com/google/uuid"
)

// Service keeps track of all resources associated with a robot.
type Service struct {
	mu        sync.Mutex
	resources []resource.Name
}

// NewFromRobot Creates and populate Service given a robot.
func NewFromRobot(r robot.Robot) (*Service, error) {
	res := New()

	if err := res.Populate(r); err != nil {
		return nil, err
	}
	return &res, nil
}

// New creates a new Service struct and initializes the resource list with a metadata service.
func New() Service {
	resources := []resource.Name{
		{
			UUID:      uuid.NewString(),
			Namespace: resource.ResourceNamespaceCore,
			Type:      resource.ResourceTypeService,
			Subtype:   resource.ResourceSubtypeMetadata,
			Name:      "",
		},
	}

	return Service{resources: resources}
}

// Populate populates Service given a robot.
func (s *Service) Populate(robot robot.Robot) error {
	// TODO: Currently just a placeholder implementation, this should be rewritten once robot/parts have more metadata about themselves
	for _, name := range robot.ArmNames() {
		err := s.Add(
			resource.Name{
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
		err := s.Add(
			resource.Name{
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
		err := s.Add(
			resource.Name{
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
		err := s.Add(
			resource.Name{
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
		err := s.Add(
			resource.Name{
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
		err := s.Add(
			resource.Name{
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
		err := s.Add(
			resource.Name{
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
		err := s.Add(
			resource.Name{
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
		err := s.Add(
			resource.Name{
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
func (s *Service) All() []resource.Name {
	return s.resources
}

// Add adds an additional resource to the list.
func (s *Service) Add(res resource.Name) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := res.Validate(); err != nil {
		return errors.Errorf("unable to add resource: %s", err.Error())
	}

	idx := -1
	for i := range s.resources {
		if s.resources[i].UUID == res.UUID {
			idx = i
			break
		}
	}
	if idx != -1 {
		return errors.Errorf("resource with uuid %s already exists and cannot be added again", res.UUID)
	}

	s.resources = append(s.resources, res)
	return nil
}

// Remove removes resource from the list.
func (s *Service) Remove(res resource.Name) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := res.Validate(); err != nil {
		return errors.Errorf("invalid resource to find and remove: %s", err.Error())
	}

	idx := -1
	for i := range s.resources {
		if s.resources[i].UUID == res.UUID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return errors.Errorf("unable to find and remove resource with uuid %s", res.UUID)
	}

	s.resources = append(s.resources[:idx], s.resources[idx+1:]...)
	return nil
}

type ctxMetadataKey int

const (
	ctxKeyMetadata = ctxMetadataKey(iota)
)

// ContextWithService attaches a metadata Service to the given context.
func ContextWithService(ctx context.Context, s *Service) context.Context {
	return context.WithValue(ctx, ctxKeyMetadata, s)
}

// ContextService returns a metadata Service struct. It may be nil if the value was never set.
func ContextService(ctx context.Context) *Service {
	s := ctx.Value(ctxKeyMetadata)
	if s == nil {
		return nil
	}
	return s.(*Service)
}
