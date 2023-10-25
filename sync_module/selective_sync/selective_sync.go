// Package selectivesync implements a datasync manager.
package selectivesync

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/vision"
)

// Model is the full model definition.
var Model = resource.NewModel("selectivesync", "demo", "vision")

func init() {
	registration := resource.Registration[resource.Resource, *Config]{
		Constructor: newSelectiveSyncer,
	}
	resource.RegisterComponent(generic.API, Model, registration)
}

// Config contains the name to the underlying camera and the name of the vision service to be used.
type Config struct {
	DataManager   string `json:"data_manager"`
	Camera        string `json:"camera"`
	VisionService string `json:"vision_service"`
}

// Validate validates the config and returns implicit dependencies.
func (cfg *Config) Validate(path string) ([]string, error) {
	if cfg.DataManager == "" {
		return nil, fmt.Errorf(`expected "data_manager" attribute in %q`, path)
	}
	if cfg.Camera == "" {
		return nil, fmt.Errorf(`expected "camera" attribute in %q`, path)
	}
	if cfg.VisionService == "" {
		return nil, fmt.Errorf(`expected "vision_service" attribute in %q`, path)
	}

	return []string{cfg.DataManager, cfg.Camera, cfg.VisionService}, nil
}

type visionSyncer struct {
	resource.Named
	actualDM      datamanager.Service
	camera        camera.Camera
	visionService vision.Service

	cancelCtx  context.Context
	cancelFunc func()
	wg         sync.WaitGroup
	logger     golog.Logger
}

func newSelectiveSyncer(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger,
) (resource.Resource, error) {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	v := &visionSyncer{
		Named:      conf.ResourceName().AsNamed(),
		logger:     logger,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
	}
	if err := v.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	v.wg.Add(1)
	utils.PanicCapturingGo(func() {
		defer v.wg.Done()
		v.pollForSyncTrigger()
	})
	return v, nil
}

func (s *visionSyncer) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	dmConfig, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	s.actualDM, err = datamanager.FromDependencies(deps, dmConfig.DataManager)
	if err != nil {
		return errors.Wrapf(err, "unable to get dm %v for visionSyncer", dmConfig.DataManager)
	}
	// If sync_disabled is false for the data manager, there are no guarantees that the selective sync
	// behavior will function as intended. It is possible that data will be synced irregardless of the trigger.
	s.logger.Warnw("Behavior is undetermined when scheduled sync is enabled and using selective sync", "data_manager", dmConfig.DataManager)
	s.camera, err = camera.FromDependencies(deps, dmConfig.Camera)
	if err != nil {
		return errors.Wrapf(err, "unable to get camera %v for visionSyncer", dmConfig.VisionService)
	}
	s.visionService, err = vision.FromDependencies(deps, dmConfig.VisionService)
	if err != nil {
		return errors.Wrapf(err, "unable to get vision service %v for visionSyncer", dmConfig.VisionService)
	}

	return nil
}

// DoCommand simply echos whatever was sent.
func (s *visionSyncer) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}

func (s *visionSyncer) pollForSyncTrigger() {
	stream, err := s.camera.Stream(s.cancelCtx)
	defer utils.UncheckedErrorFunc(func() error {
		return stream.Close(s.cancelCtx)
	})
	if err != nil {
		s.logger.Error("could not get camera stream")
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			// Check for stuff, if true Sync
			img, release, err := stream.Next(s.cancelCtx)
			if err != nil {
				s.logger.Error("could not get next image")
				release()
				return
			}
			detections, err := s.visionService.Detections(s.cancelCtx, img, map[string]interface{}{})
			release()
			if err != nil {
				s.logger.Error("could not get detections")
				return
			}
			if len(detections) != 0 {
				s.logger.Info("time to sync")
				if err := s.actualDM.Sync(s.cancelCtx, nil); err != nil {
					s.logger.Error("could not sync images")
					return
				}
			}
		case <-s.cancelCtx.Done():
			s.logger.Info("canceled selective syncing")
			return
		}
	}
}

// Close closes the underlying generic.
func (s *visionSyncer) Close(ctx context.Context) error {
	s.cancelFunc()
	s.wg.Wait()
	return nil
}
