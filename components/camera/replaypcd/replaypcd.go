// Package replaypcd implements a replay camera that can return point cloud data.
package replaypcd

import (
	"bytes"
	"context"
	"image"
	"net/http"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	datapb "go.viam.com/api/app/data/v1"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
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
	model = resource.DefaultModelFamily.WithModel("replay_pcd")

	// ErrEndOfDataset represents that the replay sensor has reached the end of the dataset.
	ErrEndOfDataset = errors.New("reached end of dataset")
)

func init() {
	resource.RegisterComponent(camera.API, model, resource.Registration[camera.Camera, *Config]{
		Constructor: newPCDCamera,
	})
}

// Config describes how to configure the replay camera component.
type Config struct {
	Source         string       `json:"source,omitempty"`
	RobotID        string       `json:"robot_id,omitempty"`
	LocationID     string       `json:"location_id,omitempty"`
	OrganizationID string       `json:"organization_id,omitempty"`
	Interval       TimeInterval `json:"time_interval,omitempty"`
	BatchSize      *uint64      `json:"batch_size,omitempty"`
	APIKey         string       `json:"api_key,omitempty"`
	APIKeyID       string       `json:"api_key_id,omitempty"`
}

// TimeInterval holds the start and end time used to filter data.
type TimeInterval struct {
	Start string `json:"start,omitempty"`
	End   string `json:"end,omitempty"`
}

// pointCloudCacheEntry stores data that was downloaded from a previous operation but has not yet been passed
// to the caller.
type pointCloudCacheEntry struct {
	pc            pointcloud.PointCloud
	timeRequested *timestamppb.Timestamp
	timeReceived  *timestamppb.Timestamp
	uri           string
	err           error
}

type imageCacheEntry struct {
	image         camera.NamedImage
	imageSource   string
	mimeType      string
	fileExt       string
	timeRequested *timestamppb.Timestamp
	timeReceived  *timestamppb.Timestamp
	uri           string
	err           error
}

// Validate checks that the config attributes are valid for a replay camera.
func (cfg *Config) Validate(path string) ([]string, error) {
	if cfg.Source == "" {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "source")
	}

	if cfg.RobotID == "" {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "robot_id")
	}

	if cfg.LocationID == "" {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "location_id")
	}

	if cfg.OrganizationID == "" {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "organization_id")
	}
	if cfg.APIKey == "" {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "api_key")
	}
	if cfg.APIKeyID == "" {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "api_key_id")
	}

	var err error
	var startTime time.Time
	if cfg.Interval.Start != "" {
		startTime, err = time.Parse(timeFormat, cfg.Interval.Start)
		if err != nil {
			return nil, errors.New("invalid time format for start time (UTC), use RFC3339")
		}
	}

	var endTime time.Time
	if cfg.Interval.End != "" {
		endTime, err = time.Parse(timeFormat, cfg.Interval.End)
		if err != nil {
			return nil, errors.New("invalid time format for end time (UTC), use RFC3339")
		}
	}

	if cfg.Interval.Start != "" && cfg.Interval.End != "" && startTime.After(endTime) {
		return nil, errors.New("invalid config, end time (UTC) must be after start time (UTC)")
	}

	if cfg.BatchSize != nil && (*cfg.BatchSize > uint64(maxCacheSize) || *cfg.BatchSize == 0) {
		return nil, errors.Errorf("batch_size must be between 1 and %d", maxCacheSize)
	}

	return []string{cloud.InternalServiceName.String()}, nil
}

// pcdCamera is a camera model that plays back pre-captured point cloud data.
type pcdCamera struct {
	resource.Named
	logger logging.Logger

	APIKey       string
	APIKeyID     string
	cloudConnSvc cloud.ConnectionService
	cloudConn    rpc.ClientConn
	dataClient   datapb.DataServiceClient
	httpClient   *http.Client

	lastPointCloudData string
	lastImagesData     string
	limit              uint64
	filter             *datapb.Filter

	pointCloudCache []*pointCloudCacheEntry
	imageCache      []*imageCacheEntry

	mu     sync.RWMutex
	closed bool
}

// newPCDCamera creates a new replay camera based on the inputted config and dependencies.
func newPCDCamera(
	ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger,
) (camera.Camera, error) {
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
	// First acquire the lock, so that it's safe to populate the cache and/or retrieve and
	// remove the next data point from the cache. Note that if multiple threads call
	// NextPointCloud concurrently, they may get data out-of-order, since there's no guarantee
	// about who acquires the lock first.
	replay.mu.Lock()
	defer replay.mu.Unlock()
	if replay.closed {
		return nil, errors.New("session closed")
	}

	// Retrieve next cached data and remove from cache, if no data remains in the cache, download a
	// new batch
	if len(replay.pointCloudCache) != 0 {
		return replay.getPointCloudDataFromCache(ctx)
	}

	filter := replay.filter
	filter.MimeType = []string{utils.MimeTypePCD}
	filter.Method = "NextPointCloud"
	// Retrieve data from the cloud. If the batch size is > 1, only metadata is returned here, otherwise
	// IncludeBinary can be set to true and the data can be downloaded directly via BinaryDataByFilter
	resp, err := replay.dataClient.BinaryDataByFilter(ctx, &datapb.BinaryDataByFilterRequest{
		DataRequest: &datapb.DataRequest{
			Filter:    filter,
			Limit:     replay.limit,
			Last:      replay.lastPointCloudData,
			SortOrder: datapb.Order_ORDER_ASCENDING,
		},
		CountOnly:     false,
		IncludeBinary: replay.limit == 1,
	})
	if err != nil {
		return nil, err
	}

	if len(resp.GetData()) == 0 {
		return nil, ErrEndOfDataset
	}
	replay.lastPointCloudData = resp.GetLast()

	// If using a batch size of 1, we already received the data itself, so decode and return the
	// binary data directly
	if replay.limit == 1 {
		pc, err := decodePointCloudResponseData(resp.GetData())
		if err != nil {
			return nil, err
		}
		if err := addGRPCMetadata(ctx,
			resp.GetData()[0].GetMetadata().GetTimeRequested(),
			resp.GetData()[0].GetMetadata().GetTimeReceived()); err != nil {
			return nil, err
		}
		return pc, nil
	}

	// Otherwise if using a batch size > 1, use the metadata from BinaryDataByFilter to download
	// data in parallel and cache the results
	replay.pointCloudCache = make([]*pointCloudCacheEntry, len(resp.Data))
	for i, dataResponse := range resp.Data {
		md := dataResponse.GetMetadata()
		replay.pointCloudCache[i] = &pointCloudCacheEntry{
			uri:           md.GetUri(),
			timeRequested: md.GetTimeRequested(),
			timeReceived:  md.GetTimeReceived(),
		}
	}

	ctxTimeout, cancelTimeout := context.WithTimeout(ctx, downloadTimeout)
	defer cancelTimeout()
	replay.downloadPointCloudBatch(ctxTimeout)
	if ctxTimeout.Err() != nil {
		return nil, errors.Wrap(ctxTimeout.Err(), "failed to download batch")
	}

	return replay.getPointCloudDataFromCache(ctx)
}

// downloadPointCloudBatch iterates through the current cache, performing the download of the respective data in
// parallel and adds all of them to the cache before returning.
func (replay *pcdCamera) downloadPointCloudBatch(ctx context.Context) {
	// Parallelize download of data based on ids in cache
	var wg sync.WaitGroup
	wg.Add(len(replay.pointCloudCache))
	for _, dataToCache := range replay.pointCloudCache {
		data := dataToCache

		goutils.PanicCapturingGo(func() {
			defer wg.Done()
			data.pc, data.err = replay.getPointcloudFromHTTP(ctx, data.uri)
			if data.err != nil {
				return
			}
		})
	}
	wg.Wait()
}

// getPointcloudFromHTTP makes a request to an http endpoint app serves, which gets redirected to GCS.
func (replay *pcdCamera) getPointcloudFromHTTP(ctx context.Context, dataURL string) (pointcloud.PointCloud, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dataURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("key_id", replay.APIKeyID)
	req.Header.Add("key", replay.APIKey)

	res, err := replay.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	pc, err := pointcloud.ReadPCD(res.Body)
	if err != nil {
		return nil, multierr.Combine(err, res.Body.Close())
	}
	if res.StatusCode != http.StatusOK {
		return nil, multierr.Combine(errors.New(res.Status), res.Body.Close())
	}

	if err := res.Body.Close(); err != nil {
		return nil, err
	}

	return pc, nil
}

// getPointCloudDataFromCache retrieves the next cached data and removes it from the cache. It assumes the
// write lock is being held.
func (replay *pcdCamera) getPointCloudDataFromCache(ctx context.Context) (pointcloud.PointCloud, error) {
	// Grab the next cached data and update the cache immediately, even if there's an error,
	// so we don't get stuck in a loop checking for and returning the same error.
	data := replay.pointCloudCache[0]
	replay.pointCloudCache = replay.pointCloudCache[1:]
	if data.err != nil {
		return nil, errors.Wrap(data.err, "cache data contained an error")
	}

	if err := addGRPCMetadata(ctx, data.timeRequested, data.timeReceived); err != nil {
		return nil, err
	}

	return data.pc, nil
}

// addGRPCMetadata adds timestamps from the data response to the gRPC response header if one is
// found in the context.
func addGRPCMetadata(ctx context.Context, timeRequested, timeReceived *timestamppb.Timestamp) error {
	if stream := grpc.ServerTransportStreamFromContext(ctx); stream != nil {
		var grpcMetadata metadata.MD = make(map[string][]string)
		if timeRequested != nil {
			grpcMetadata.Set(contextutils.TimeRequestedMetadataKey, timeRequested.AsTime().Format(time.RFC3339Nano))
		}
		if timeReceived != nil {
			grpcMetadata.Set(contextutils.TimeReceivedMetadataKey, timeReceived.AsTime().Format(time.RFC3339Nano))
		}
		if err := grpc.SetHeader(ctx, grpcMetadata); err != nil {
			return err
		}
	}

	return nil
}

// Images is a part of the camera interface but is not implemented for replay.
func (replay *pcdCamera) Images(ctx context.Context) ([]camera.NamedImage, resource.ResponseMetadata, error) {
	// First acquire the lock, so that it's safe to populate the cache and/or retrieve and
	// remove the next data point from the cache. Note that if multiple threads call
	// Images concurrently, they may get data out-of-order, since there's no guarantee
	// about who acquires the lock first.
	replay.mu.Lock()
	defer replay.mu.Unlock()
	if replay.closed {
		return nil, resource.ResponseMetadata{}, errors.New("session closed")
	}

	// Retrieve next cached data and remove from cache, if no data remains in the cache, download a
	// new batch
	if len(replay.imageCache) != 0 {
		return replay.getImagesDataFromCache(ctx)
	}

	filter := replay.filter
	filter.MimeType = []string{}
	filter.Method = "GetImages"
	// Retrieve data from the cloud. If the batch size is > 1, only metadata is returned here, otherwise
	// IncludeBinary can be set to true and the data can be downloaded directly via BinaryDataByFilter
	resp, err := replay.dataClient.BinaryDataByFilter(ctx, &datapb.BinaryDataByFilterRequest{
		DataRequest: &datapb.DataRequest{
			Filter:    filter,
			Limit:     replay.limit,
			Last:      replay.lastImagesData,
			SortOrder: datapb.Order_ORDER_ASCENDING,
		},
		CountOnly: false,
		// IncludeBinary: replay.limit == 1,
	})
	if err != nil {
		return nil, resource.ResponseMetadata{}, err
	}

	if len(resp.GetData()) == 0 {
		return nil, resource.ResponseMetadata{}, ErrEndOfDataset
	}
	replay.lastImagesData = resp.GetLast()

	// If using a batch size of 1, we already received the data itself, so decode and return the
	// binary data directly
	// if replay.limit == 1 {
	// 	image, err := decodeImagesResponseData(ctx, resp.GetData())
	// 	if err != nil {
	// 		return nil, resource.ResponseMetadata{}, err
	// 	}
	// 	if err := addGRPCMetadata(ctx,
	// 		resp.GetData()[0].GetMetadata().GetTimeRequested(),
	// 		resp.GetData()[0].GetMetadata().GetTimeReceived()); err != nil {
	// 		return nil, resource.ResponseMetadata{}, err
	// 	}
	// 	namedImage := camera.NamedImage{Image: image, SourceName: replay.filter.ComponentName}
	// 	return []camera.NamedImage{namedImage}, resource.ResponseMetadata{}, nil
	// }

	// Otherwise if using a batch size > 1, use the metadata from BinaryDataByFilter to download
	// data in parallel and cache the results
	replay.imageCache = make([]*imageCacheEntry, len(resp.Data))
	for i, dataResponse := range resp.Data {
		md := dataResponse.GetMetadata()
		replay.imageCache[i] = &imageCacheEntry{
			imageSource:   md.CaptureMetadata.ComponentName,
			mimeType:      md.CaptureMetadata.MimeType,
			fileExt:       md.FileExt,
			uri:           md.GetUri(),
			timeRequested: md.GetTimeRequested(),
			timeReceived:  md.GetTimeReceived(),
		}
	}

	ctxTimeout, cancelTimeout := context.WithTimeout(ctx, downloadTimeout)
	defer cancelTimeout()
	replay.downloadImagesBatch(ctxTimeout)
	if ctxTimeout.Err() != nil {
		return nil, resource.ResponseMetadata{}, errors.Wrap(ctxTimeout.Err(), "failed to download batch")
	}

	if len(replay.imageCache) == 0 {
		return []camera.NamedImage{}, resource.ResponseMetadata{}, errors.New("No image in the cache!")
	}

	return replay.getImagesDataFromCache(ctx)
}

// Properties is a part of the camera interface and returns the camera.Properties struct with SupportsPCD set to true.
func (replay *pcdCamera) Properties(ctx context.Context) (camera.Properties, error) {
	props := camera.Properties{
		SupportsPCD: true,
	}
	return props, nil
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
	replay.mu.Lock()
	defer replay.mu.Unlock()

	replay.closed = true
	// Close cloud connection
	replay.closeCloudConnection(ctx)
	return nil
}

// Reconfigure finishes the bring up of the replay camera by evaluating given arguments and setting up the required cloud
// connection.
func (replay *pcdCamera) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	replay.mu.Lock()
	defer replay.mu.Unlock()
	if replay.closed {
		return errors.New("session closed")
	}

	replayCamConfig, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}
	replay.APIKey = replayCamConfig.APIKey
	replay.APIKeyID = replayCamConfig.APIKeyID

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
	replay.pointCloudCache = nil

	replay.filter = &datapb.Filter{
		ComponentName:   replayCamConfig.Source,
		RobotId:         replayCamConfig.RobotID,
		LocationIds:     []string{replayCamConfig.LocationID},
		OrganizationIds: []string{replayCamConfig.OrganizationID},
		Interval:        &datapb.CaptureInterval{},
	}
	replay.lastPointCloudData = ""
	replay.lastImagesData = ""

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

	_, conn, err := replay.cloudConnSvc.AcquireConnectionAPIKey(ctx, replay.APIKey, replay.APIKeyID)
	if err != nil {
		return err
	}
	dataServiceClient := datapb.NewDataServiceClient(conn)

	replay.cloudConn = conn
	replay.dataClient = dataServiceClient
	replay.httpClient = &http.Client{}
	return nil
}

// downloadImagesBatch iterates through the current cache, performing the download of the respective data in
// parallel and adds all of them to the cache before returning.
func (replay *pcdCamera) downloadImagesBatch(ctx context.Context) {
	// Parallelize download of data based on ids in cache
	var wg sync.WaitGroup
	wg.Add(len(replay.imageCache))
	for _, dataToCache := range replay.imageCache {
		data := dataToCache

		goutils.PanicCapturingGo(func() {
			defer wg.Done()
			data.image, data.err = replay.getImageFromHTTP(ctx, data)
			if data.err != nil {
				return
			}
		})
	}
	wg.Wait()
}

// getImageFromHTTP makes a request to an http endpoint app serves, which gets redirected to GCS.
func (replay *pcdCamera) getImageFromHTTP(ctx context.Context, data *imageCacheEntry) (camera.NamedImage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, data.uri, nil)
	if err != nil {
		return camera.NamedImage{}, err
	}
	req.Header.Add("key_id", replay.APIKeyID)
	req.Header.Add("key", replay.APIKey)

	res, err := replay.httpClient.Do(req)
	if err != nil {
		return camera.NamedImage{}, err
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(res.Body)

	mimeType := data.mimeType
	var myImage image.Image
	if data.fileExt == ".dep" {
		mimeType = utils.MimeTypeRawDepth
		myImage = rimage.NewLazyEncodedImage(buf.Bytes(), utils.MimeTypeRawDepth)
	} else {
		myImage, err = rimage.DecodeImage(ctx, buf.Bytes(), mimeType)
		if err != nil {
			return camera.NamedImage{}, multierr.Combine(err, res.Body.Close())
		}
	}
	if res.StatusCode != http.StatusOK {
		return camera.NamedImage{}, multierr.Combine(errors.New(res.Status), res.Body.Close())
	}

	if err := res.Body.Close(); err != nil {
		return camera.NamedImage{}, err
	}

	namedImage := camera.NamedImage{
		Image:      myImage,
		SourceName: data.imageSource,
	}

	return namedImage, nil
}

// getImagesDataFromCache retrieves the next cached data and removes it from the cache. It assumes the
// write lock is being held.
func (replay *pcdCamera) getImagesDataFromCache(ctx context.Context) ([]camera.NamedImage, resource.ResponseMetadata, error) {
	data := replay.imageCache[0]
	if data.err != nil {
		// If there's an error, update the cache immediately,
		// so we don't get stuck in a loop checking for and returning the same error.
		replay.imageCache = replay.imageCache[1:]
		return nil, resource.ResponseMetadata{}, errors.Wrap(data.err, "cache data contained an error")
	}

	images := []camera.NamedImage{data.image}
	i := 0
	deleteFromIndex := i

	// TODO[kat]: Implement getting new data if we run out of data. Need to extract function out
	// Loop:
	// 	for {
	for i, nextData := range replay.imageCache[1:] {
		if nextData.err != nil {
			// If there's an error, update the cache immediately,
			// so we don't get stuck in a loop checking for and returning the same error.
			// Discard all data before the error occurred.
			replay.imageCache = replay.imageCache[i+2:]
			return nil, resource.ResponseMetadata{}, errors.Wrap(nextData.err, "cache data contained an error")
		}
		if nextData.timeRequested.GetSeconds() == data.timeRequested.GetSeconds() &&
			nextData.timeRequested.GetNanos() == data.timeRequested.GetNanos() {
			images = append(images, nextData.image)
			deleteFromIndex++
		} else {
			break
		}
	}
	// }
	replay.imageCache = replay.imageCache[deleteFromIndex+1:]

	if err := addGRPCMetadata(ctx, data.timeRequested, data.timeReceived); err != nil {
		return nil, resource.ResponseMetadata{}, err
	}

	return images, resource.ResponseMetadata{CapturedAt: data.timeReceived.AsTime()}, nil
}

// decodePointCloudResponseData decodes the pcd file byte array.
func decodePointCloudResponseData(respData []*datapb.BinaryData) (pointcloud.PointCloud, error) {
	if len(respData) == 0 {
		return nil, errors.New("no response data; this should never happen")
	}

	pc, err := pointcloud.ReadPCD(bytes.NewBuffer(respData[0].GetBinary()))
	if err != nil {
		return nil, err
	}

	return pc, nil
}
