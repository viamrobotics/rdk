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
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils/contextutils"
)

const (
	timeFormat            = time.RFC3339
	grpcConnectionTimeout = 10 * time.Second
	downloadTimeout       = 30 * time.Second
	maxCacheSize          = 100
)

var (
	// model is the model of a replay camera.
	model           = resource.DefaultModelFamily.WithModel("replay_pcd")
	errEndOfDataset = errors.New("reached end of dataset")
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

// cacheEntry stores data that was downloaded from a previous operation but has not yet been passed
// to the caller.
type cacheEntry struct {
	id      string
	pc      pointcloud.PointCloud
	timeReq *timestamppb.Timestamp
	timeRec *timestamppb.Timestamp
	err     error
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

	if cfg.BatchSize != nil && *cfg.BatchSize > uint64(maxCacheSize) {
		return nil, errors.Errorf("batch_size must be less than or equal to %d", maxCacheSize)
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

	cache []*cacheEntry

	activeBackgroundWorkers sync.WaitGroup

	mu     sync.RWMutex
	closed bool
}

// newPCDCamera creates a new replay camera based on the inputted config and dependencies.
func newPCDCamera(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger) (camera.Camera, error) {
	cam := &pcdCamera{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
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

	// Retrieve next cached data and remove from cache, if no data remains in the cache, download a
	// new batch
	if len(replay.cache) != 0 {
		return replay.getDataFromCache(ctx)
	}

	// Retrieve data from the cloud. If the batch size is > 1, only metadata is returned here, otherwise
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

	// If using a batch size of 1, we already received the data itself, so decode and return the
	// binary data directly
	if replay.limit == 1 {
		pc, err := decodeResponseData(resp.GetData(), replay.logger)
		if err != nil {
			return nil, err
		}
		err = addGRPCMetadata(ctx, resp.GetData()[0].GetMetadata().GetTimeRequested(), resp.GetData()[0].GetMetadata().GetTimeReceived())
		if err != nil {
			return nil, err
		}
		return pc, nil
	}

	// Otherwise if using a batch size > 1, use the metadata from BinaryDataByFilter to download
	// data in parallel and cache the results
	replay.cache = make([]*cacheEntry, replay.limit)

	for i, dataResponse := range resp.Data {
		replay.cache[i] = &cacheEntry{id: dataResponse.GetMetadata().Id}
	}

	ctxTimeout, cancelTimeout := context.WithTimeout(ctx, downloadTimeout)
	defer cancelTimeout()
	ch := make(chan struct{})
	goutils.PanicCapturingGo(func() {
		defer close(ch)
		replay.downloadBatch(ctxTimeout)
	})

	select {
	case <-ch:
	case <-ctxTimeout.Done():
		return nil, errors.Errorf("context canceled after download deadline exceeded")
	}

	return replay.NextPointCloud(ctx)
}

// downloadBatch iterates through the current cache, performing the download of the respective data in
// parallel and adds all of them to the cache before returning.
func (replay *pcdCamera) downloadBatch(ctx context.Context) {
	// Parallelize download of data based on ids in cache
	var wg sync.WaitGroup
	wg.Add(len(replay.cache))
	for _, dataToCache := range replay.cache {
		dataToCache := dataToCache

		goutils.PanicCapturingGo(func() {
			defer wg.Done()

			var resp *datapb.BinaryDataByIDsResponse
			resp, dataToCache.err = replay.dataClient.BinaryDataByIDs(ctx, &datapb.BinaryDataByIDsRequest{
				FileIds:       []string{dataToCache.id},
				IncludeBinary: true,
			})
			if dataToCache.err != nil {
				return
			}

			// Decode response data
			dataToCache.pc, dataToCache.err = decodeResponseData(resp.GetData(), replay.logger)
			if dataToCache.err == nil {
				dataToCache.timeReq = resp.GetData()[0].GetMetadata().GetTimeRequested()
				dataToCache.timeRec = resp.GetData()[0].GetMetadata().GetTimeReceived()
			}
		})
	}
	wg.Wait()
}

// getDataFromCache retrieves the next cached data and removes it from the cache.
func (replay *pcdCamera) getDataFromCache(ctx context.Context) (pointcloud.PointCloud, error) {
	// Grab the next cached data and update the cache immediately, even if there's an error,
	// so we don't get stuck in a loop checking for and returning the same error.
	data := replay.cache[0]
	replay.cache = replay.cache[1:]
	if data.err != nil {
		return nil, errors.Wrap(data.err, "cache data contained an error")
	}

	err := addGRPCMetadata(ctx, data.timeReq, data.timeRec)
	if err != nil {
		return nil, err
	}

	return data.pc, nil
}

// addGRPCMetadata adds timestamps from the data response to the gRPC response header if one is
// found in the context.
func addGRPCMetadata(ctx context.Context, timeReq, timeRec *timestamppb.Timestamp) error {
	if stream := grpc.ServerTransportStreamFromContext(ctx); stream != nil {
		var grpcMetadata metadata.MD = make(map[string][]string)
		if timeReq != nil {
			grpcMetadata.Set(contextutils.TimeRequestedMetadataKey, timeReq.AsTime().Format(time.RFC3339Nano))
		}
		if timeRec != nil {
			grpcMetadata.Set(contextutils.TimeReceivedMetadataKey, timeRec.AsTime().Format(time.RFC3339Nano))
		}
		if err := grpc.SetHeader(ctx, grpcMetadata); err != nil {
			return err
		}
	}

	return nil
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
	replay.activeBackgroundWorkers.Wait()

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
	replay.cache = nil

	replay.filter = &datapb.Filter{
		ComponentName: replayCamConfig.Source,
		RobotId:       replayCamConfig.RobotID,
		MimeType:      []string{"pointcloud/pcd"},
		Interval:      &datapb.CaptureInterval{},
	}
	replay.lastData = ""

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
