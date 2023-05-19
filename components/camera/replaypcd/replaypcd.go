// Package replaypcd implements a replay camera that can return point cloud data.
package replaypcd

import (
	"bytes"
	"compress/gzip"
	"context"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
	datapb "go.viam.com/api/app/data/v1"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage/transform"
)

// Model is the model of a replay camera.
var (
	model                 = resource.DefaultModelFamily.WithModel("replay_pcd")
	errEndOfDataset       = errors.New("reached end of dataset")
	timeFormat            = time.RFC3339
	grpcConnectionTimeout = 10 * time.Second
	downloadTimeout       = 30 * time.Second
)

func init() {
	resource.RegisterComponent(camera.API, model, resource.Registration[camera.Camera, *Config]{
		Constructor: newPCDCamera,
	})
}

// Config describes how to configure the replay camera component.
type Config struct {
	Source    string       `json:"source,omitempty"`
	RobotID   string       `json:"robot_id,omitempty"`
	Interval  TimeInterval `json:"time_interval,omitempty"`
	BatchSize *uint64      `json:"batch_size,omitempty"`
}

// TimeInterval holds the start and end time used to filter data.
type TimeInterval struct {
	Start string `json:"start,omitempty"`
	End   string `json:"end,omitempty"`
}

// cacheEntry stored data that was downloaded from a previous operation but has not yet been past
// to the caller. cacheEntries are initially instantiated with only an id to preserve download
// order; once the download has been completed, ready is set to true and any resulting point clouds
// and errors are emplaced.
type cacheEntry struct {
	id    string
	pc    pointcloud.PointCloud
	err   error
	ready bool
}

// Validate checks that the config attributes are valid for a replay camera.
func (cfg *Config) Validate(path string) ([]string, error) {
	if cfg.Source == "" {
		return nil, goutils.NewConfigValidationFieldRequiredError(path, "source")
	}

	var err error
	var startTime time.Time
	if cfg.Interval.Start != "" {
		startTime, err = time.Parse(timeFormat, cfg.Interval.Start)
		if err != nil {
			return nil, errors.New("invalid time format for start time (UTC), use RFC3339")
		}
		if startTime.After(time.Now()) {
			return nil, errors.New("invalid config, start time (UTC) must be in the past")
		}
	}

	var endTime time.Time
	if cfg.Interval.End != "" {
		endTime, err = time.Parse(timeFormat, cfg.Interval.End)
		if err != nil {
			return nil, errors.New("invalid time format for end time (UTC), use RFC3339")
		}
		if endTime.After(time.Now()) {
			return nil, errors.New("invalid config, end time (UTC) must be in the past")
		}
	}

	if cfg.Interval.Start != "" && cfg.Interval.End != "" && startTime.After(endTime) {
		return nil, errors.New("invalid config, end time (UTC) must be after start time (UTC)")
	}

	return []string{cloud.InternalServiceName.String()}, nil
}

// pcdCamera is a camera model that plays back pre-captured point cloud data.
type pcdCamera struct {
	resource.Named
	logger golog.Logger

	cloudConnSvc cloud.ConnectionService
	cloudConn    rpc.ClientConn
	dataClient   datapb.DataServiceClient

	lastData string
	limit    uint64
	filter   *datapb.Filter

	cache   []cacheEntry
	cacheCh chan cacheEntry

	mu     sync.RWMutex
	closed bool
}

// newPCDCamera creates a new replay camera based on the inputted config and dependencies.
func newPCDCamera(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger) (camera.Camera, error) {
	cam := &pcdCamera{
		Named:   conf.ResourceName().AsNamed(),
		logger:  logger,
		cacheCh: make(chan cacheEntry),
	}

	if err := cam.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}

	return cam, nil
}

// NextPointCloud returns the next point cloud retrieved from cloud storage based on the applied filter.
func (replay *pcdCamera) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	replay.mu.RLock()
	defer replay.mu.RUnlock()
	if replay.closed {
		return nil, errors.New("session closed")
	}

	// Retrieve next cached data and remove from cache, if no data remains in the cache, download a new batch
	if len(replay.cache) != 0 {
		return replay.getDataFromCache()
	}

	// Retrieve data from the cloud. If the batch size is one, only metadata is returned here, otherwise
	// IncludeBinary can be set to true and the data can be downloaded directly via BinaryDataByFilter
	resp, err := replay.dataClient.BinaryDataByFilter(ctx, &datapb.BinaryDataByFilterRequest{
		DataRequest: &datapb.DataRequest{
			Filter:    replay.filter,
			Limit:     replay.limit,
			Last:      replay.lastData,
			SortOrder: datapb.Order_ORDER_ASCENDING,
		},
		CountOnly:     false,
		IncludeBinary: replay.limit == 1,
	})
	if err != nil {
		return nil, err
	}

	replay.lastData = resp.GetLast()

	// If batching use metadata to download data in parallel, caching results
	if replay.limit > 1 {
		for _, dataResponse := range resp.Data {
			newCacheEntry := cacheEntry{id: dataResponse.GetMetadata().Id}
			replay.cache = append(replay.cache, newCacheEntry)
		}
		ctxTimeout, cancelTimeout := context.WithTimeout(ctx, downloadTimeout)
		defer cancelTimeout()

		replay.downloadBatch(ctxTimeout)
		replay.waitForDataAndUpdateCache(ctxTimeout)
		return replay.NextPointCloud(ctx)
	}

	// If not batching, decode the returned binary directly
	pc, err := decodeResponseData(resp.GetData(), replay.logger)
	if err != nil {
		return nil, err
	}

	return pc, nil
}

// downloadBatch iterates through the current cache, performing the download of the respective data in
// parallel using go routines.
func (replay *pcdCamera) downloadBatch(ctx context.Context) {
	var wg sync.WaitGroup

	// Parallelize download of data based on given ids
	for _, cEntry := range replay.cache {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			// A new cacheEntry is created here with the associated id and ready set to true as, once the
			// go routine has completed, it will always have either an error or point cloud associated with it.
			cData := cacheEntry{id: id, ready: true}

			var resp *datapb.BinaryDataByIDsResponse
			resp, cData.err = replay.dataClient.BinaryDataByIDs(ctx, &datapb.BinaryDataByIDsRequest{
				FileIds:       []string{id},
				IncludeBinary: true,
			})
			if cData.err != nil {
				replay.cacheCh <- cData
				return
			}

			// Decode response data
			cData.pc, cData.err = decodeResponseData(resp.GetData(), replay.logger)

			// Send data to cache channel to be added to the cache
			replay.cacheCh <- cData
		}(cEntry.id)
	}
}

// waitForDataAndUpdateCache monitors the cache channel until all data has been returned or the timeout has
// been triggered.
func (replay *pcdCamera) waitForDataAndUpdateCache(ctx context.Context) {
	for {
		// Check if all data has been returned
		done := true
		for _, c := range replay.cache {
			done = done && c.ready
		}
		if done {
			return
		}
		// Cache data from channel
		select {
		case <-ctx.Done():
			return
		case cData := <-replay.cacheCh:
			for i, c := range replay.cache {
				if c.id == cData.id {
					replay.cache[i] = cData
				}
			}
		}
	}
}

// getDataFromCache retrieves the next cached data and removes it from the cache.
func (replay *pcdCamera) getDataFromCache() (pointcloud.PointCloud, error) {
	data := replay.cache[0]

	if !data.ready {
		return nil, errors.New("data from cache not returned")
	}
	if data.err != nil {
		return nil, errors.Wrapf(data.err, "cache data contained an error")
	}

	replay.cache = replay.cache[1:]

	return data.pc, nil
}

// Properties is a part of the camera interface but is not implemented for replay.
func (replay *pcdCamera) Properties(ctx context.Context) (camera.Properties, error) {
	var props camera.Properties
	return props, errors.New("Properties is unimplemented")
}

// Projector is a part of the camera interface but is not implemented for replay.
func (replay *pcdCamera) Projector(ctx context.Context) (transform.Projector, error) {
	var proj transform.Projector
	return proj, errors.New("Projector is unimplemented")
}

// Stream is a part of the camera interface but is not implemented for replay.
func (replay *pcdCamera) Stream(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
	var stream gostream.VideoStream
	return stream, errors.New("Stream is unimplemented")
}

// Close stops replay camera, closes the channels and its connections to the cloud.
func (replay *pcdCamera) Close(ctx context.Context) error {
	// Close cache channel
	close(replay.cacheCh)

	// Close cloud connection
	replay.closeCloudConnection(ctx)

	replay.mu.Lock()
	defer replay.mu.Unlock()
	replay.closed = true
	return nil
}

// Reconfigure finishes the bring up of the replay camera by evaluating given arguments and setting up the required cloud
// connection.
func (replay *pcdCamera) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	replay.mu.RLock()
	defer replay.mu.RUnlock()
	if replay.closed {
		return errors.New("session closed")
	}

	replayCamConfig, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	cloudConnSvc, err := resource.FromDependencies[cloud.ConnectionService](deps, cloud.InternalServiceName)
	if err != nil {
		return err
	}

	// Update cloud connection if needed
	if replay.cloudConnSvc != cloudConnSvc {
		replay.closeCloudConnection(ctx)
		replay.cloudConnSvc = cloudConnSvc

		if err := replay.initCloudConnection(ctx); err != nil {
			replay.closeCloudConnection(ctx)
			return errors.Wrap(err, "failure to connect to the cloud")
		}
	}

	if replayCamConfig.BatchSize == nil {
		replay.limit = 1
	} else {
		replay.limit = *replayCamConfig.BatchSize
	}

	replay.filter = &datapb.Filter{
		ComponentName: replayCamConfig.Source,
		RobotId:       replayCamConfig.RobotID,
		MimeType:      []string{"pointcloud/pcd"},
		Interval:      &datapb.CaptureInterval{},
	}

	if replayCamConfig.Interval.Start != "" {
		startTime, err := time.Parse(timeFormat, replayCamConfig.Interval.Start)
		if err != nil {
			replay.closeCloudConnection(ctx)
			return errors.New("invalid time format for start time, missed during config validation")
		}
		replay.filter.Interval.Start = timestamppb.New(startTime)
	}

	if replayCamConfig.Interval.End != "" {
		endTime, err := time.Parse(timeFormat, replayCamConfig.Interval.End)
		if err != nil {
			replay.closeCloudConnection(ctx)
			return errors.New("invalid time format for end time, missed during config validation")
		}
		replay.filter.Interval.End = timestamppb.New(endTime)
	}
	return nil
}

// closeCloudConnection closes all parts of the cloud connection used by the replay camera.
func (replay *pcdCamera) closeCloudConnection(ctx context.Context) {
	if replay.cloudConn != nil {
		goutils.UncheckedError(replay.cloudConn.Close())
	}

	if replay.cloudConnSvc != nil {
		goutils.UncheckedError(replay.cloudConnSvc.Close(ctx))
	}
}

// initCloudConnection creates a rpc client connection and data service.
func (replay *pcdCamera) initCloudConnection(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, grpcConnectionTimeout)
	defer cancel()

	_, conn, err := replay.cloudConnSvc.AcquireConnection(ctx)
	if err != nil {
		return err
	}
	dataServiceClient := datapb.NewDataServiceClient(conn)

	replay.cloudConn = conn
	replay.dataClient = dataServiceClient
	return nil
}

// decodeResponseData decompresses the gzipped byte array.
func decodeResponseData(respData []*datapb.BinaryData, logger golog.Logger) (pointcloud.PointCloud, error) {
	// If no data is returned, return an error indicating we've reached the end of the dataset.
	if len(respData) == 0 {
		return nil, errEndOfDataset
	}

	r, err := gzip.NewReader(bytes.NewBuffer(respData[0].GetBinary()))
	if err != nil {
		return nil, err
	}

	defer func() {
		if err = r.Close(); err != nil {
			logger.Warnw("Failed to close gzip reader", "warn", err)
		}
	}()

	pc, err := pointcloud.ReadPCD(r)
	if err != nil {
		return nil, err
	}

	return pc, nil
}
