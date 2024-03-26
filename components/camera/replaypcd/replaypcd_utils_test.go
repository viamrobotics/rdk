package replaypcd

import (
	"context"
	"fmt"
	"image"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/pkg/errors"
	datapb "go.viam.com/api/app/data/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/components/camera"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/internal/cloud"
	cloudinject "go.viam.com/rdk/internal/testutils/inject"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testTime           = "2000-01-01T12:00:%02dZ"
	orgID              = "slam_org_id"
	locationID         = "slam_location_id"
	testingHTTPPattern = "/myurl"
)

// mockDataServiceServer is a struct that includes unimplemented versions of all the Data Service endpoints. These
// can be overwritten to allow developers to trigger desired behaviors during testing.
type mockDataServiceServer struct {
	datapb.UnimplementedDataServiceServer
	httpMock   map[method][]*httptest.Server
	lastData   map[method]string
	cameraType cameraType
}

func switchCurrentRgbdFileType() {
	if currentRGBDFileType == jpeg {
		currentRGBDFileType = depth
	} else {
		currentRGBDFileType = jpeg
	}
}

// BinaryDataByFilter is a mocked version of the Data Service function of a similar name. It returns a response with
// data corresponding to a stored pcd or image artifact based on the filter and last file accessed.
func (mDServer *mockDataServiceServer) BinaryDataByFilter(ctx context.Context, req *datapb.BinaryDataByFilterRequest,
) (*datapb.BinaryDataByFilterResponse, error) {
	// Parse request
	filter := req.DataRequest.GetFilter()
	limit := req.DataRequest.GetLimit()
	includeBinary := req.IncludeBinary
	method := method(filter.Method)
	mDServer.lastData[method] = req.DataRequest.GetLast()

	newFileNum, err := getNextFileNumAfterFilter(filter, mDServer.lastData[method], mDServer.cameraType)
	if err != nil {
		return nil, err
	}
	fmt.Println("")
	fmt.Println("In BinaryDataByFilter, newFileNum: ", newFileNum)
	fmt.Println("")
	// Construct response
	var resp datapb.BinaryDataByFilterResponse
	if includeBinary {
		data, err := getCompressedBytesFromArtifact(method, newFileNum, mDServer.cameraType)
		if err != nil {
			return nil, err
		}

		timeReq, timeRec, err := timestampsFromFileNum(newFileNum, mDServer.cameraType)
		if err != nil {
			return nil, err
		}

		id, err := getDatasetDirectory(method, newFileNum, mDServer.cameraType)
		if err != nil {
			return nil, err
		}

		binaryData := datapb.BinaryData{
			Binary: data,
			Metadata: &datapb.BinaryMetadata{
				Id:            id,
				TimeRequested: timeReq,
				TimeReceived:  timeRec,
				CaptureMetadata: &datapb.CaptureMetadata{
					OrganizationId: orgID,
					LocationId:     locationID,
				},
				Uri: mDServer.httpMock[method][newFileNum].URL + testingHTTPPattern,
			},
		}

		resp.Data = []*datapb.BinaryData{&binaryData}
		if mDServer.cameraType == rgbdCamera {
			switchCurrentRgbdFileType()
		}
		resp.Last = fmt.Sprint(newFileNum)
	} else {
		for i := 0; i < int(limit); i++ {
			if newFileNum+i >= numFiles[method] {
				break
			}

			timeReq, timeRec, err := timestampsFromFileNum(newFileNum+i, mDServer.cameraType)
			if err != nil {
				return nil, err
			}

			id, err := getDatasetDirectory(method, newFileNum+i, mDServer.cameraType)
			if err != nil {
				return nil, err
			}

			binaryData := datapb.BinaryData{
				Metadata: &datapb.BinaryMetadata{
					Id:            id,
					TimeRequested: timeReq,
					TimeReceived:  timeRec,
					CaptureMetadata: &datapb.CaptureMetadata{
						OrganizationId: orgID,
						LocationId:     locationID,
					},
					Uri: mDServer.httpMock[method][newFileNum+i].URL + testingHTTPPattern,
				},
			}
			resp.Data = append(resp.Data, &binaryData)
			if mDServer.cameraType == rgbdCamera {
				switchCurrentRgbdFileType()
			}
		}
		resp.Last = fmt.Sprint(newFileNum + int(limit) - 1)
	}

	return &resp, nil
}

func timestampsFromFileNum(fileNum int, camType cameraType) (*timestamppb.Timestamp, *timestamppb.Timestamp, error) {
	// for rgbd camera, use the same timestamps for matching rgb & depth data
	if camType == rgbdCamera {
		fileNum /= 2
	}
	timeReq, err := time.Parse(time.RFC3339, fmt.Sprintf(testTime, fileNum))
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed parsing time")
	}
	timeRec := timeReq.Add(time.Second)
	return timestamppb.New(timeReq), timestamppb.New(timeRec), nil
}

// createMockCloudDependencies creates a mockDataServiceServer and rpc client connection to it which is then
// stored in a mockCloudConnectionService.
func createMockCloudDependencies(ctx context.Context, t *testing.T, logger logging.Logger, b bool, camType cameraType) (resource.Dependencies, func() error) {
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger.AsZap(), rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	// This creates a mock server for each file used in testing
	srv := newHTTPMock(testingHTTPPattern, http.StatusOK, camType)
	test.That(t, rpcServer.RegisterServiceServer(
		ctx,
		&datapb.DataService_ServiceDesc,
		&mockDataServiceServer{
			httpMock: srv,
			lastData: map[method]string{
				nextPointCloud: "",
				getImages:      "",
			},
			cameraType: camType,
		},
		datapb.RegisterDataServiceHandlerFromEndpoint,
	), test.ShouldBeNil)

	go rpcServer.Serve(listener)

	conn, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	mockCloudConnectionService := &cloudinject.CloudConnectionService{
		Named: cloud.InternalServiceName.AsNamed(),
		Conn:  conn,
	}
	if !b {
		mockCloudConnectionService.AcquireConnectionErr = errors.New("cloud connection error")
	}

	r := &inject.Robot{}
	rs := map[resource.Name]resource.Resource{}
	rs[cloud.InternalServiceName] = mockCloudConnectionService
	r.MockResourcesFromMap(rs)

	return resourcesFromDeps(t, r, []string{cloud.InternalServiceName.String()}), rpcServer.Stop
}

// createNewReplayCamera will create a new replay_pcd camera based on the provided config with either
// a valid or invalid data client.
func createNewReplayCamera(ctx context.Context, t *testing.T, replayCamCfg *Config, validDeps bool, camType cameraType,
) (camera.Camera, resource.Dependencies, func() error, error) {
	logger := logging.NewTestLogger(t)

	resources, closeRPCFunc := createMockCloudDependencies(ctx, t, logger, validDeps, camType)

	cfg := resource.Config{ConvertedAttributes: replayCamCfg}
	cam, err := newReplayCamera(ctx, resources, cfg, logger)

	return cam, resources, closeRPCFunc, err
}

// resourcesFromDeps returns a list of dependencies from the provided robot.
func resourcesFromDeps(t *testing.T, r robot.Robot, deps []string) resource.Dependencies {
	t.Helper()
	resources := resource.Dependencies{}
	for _, dep := range deps {
		resName, err := resource.NewFromString(dep)
		test.That(t, err, test.ShouldBeNil)
		res, err := r.ResourceByName(resName)
		if err == nil {
			// some resources are weakly linked
			resources[resName] = res
		}
	}
	return resources
}

// getNextFileNumAfterFilter returns the artifact index of the next data file to be return based on
// the provided filter and last returned artifact.
func getNextFileNumAfterFilter(filter *datapb.Filter, last string, camType cameraType) (int, error) {
	// Basic component part (source) filter
	if filter.ComponentName != "" && filter.ComponentName != validSource {
		return 0, ErrEndOfDataset
	}

	// Basic robot_id filter
	if filter.RobotId != "" && filter.RobotId != validRobotID {
		return 0, ErrEndOfDataset
	}

	// Basic location_id filter
	if len(filter.LocationIds) == 0 {
		return 0, errors.New("LocationIds in filter is empty")
	}
	if filter.LocationIds[0] != "" && filter.LocationIds[0] != validLocationID {
		return 0, ErrEndOfDataset
	}

	// Basic organization_id filter
	if len(filter.OrganizationIds) == 0 {
		return 0, errors.New("OrganizationIds in filter is empty")
	}
	if filter.OrganizationIds[0] != "" && filter.OrganizationIds[0] != validOrganizationID {
		return 0, ErrEndOfDataset
	}

	// Apply the time-based filter based on the seconds value in the start and end fields. Because artifacts
	// do not have timestamps associated with them but are numerically ordered we can approximate the filtering
	// by sorting for the files which occur after the start second count and before the end second count.
	// For example, if there are 15 files in the artifact directory, the start time is 2000-01-01T12:00:10Z
	// and the end time is 2000-01-01T12:00:24Z, we will return files 10-14.
	start := 0
	end := numFiles[method(filter.Method)]
	if filter.Interval.Start != nil {
		start = filter.Interval.Start.AsTime().Second()
	}
	if filter.Interval.End != nil {
		end = int(math.Min(float64(filter.Interval.End.AsTime().Second()), float64(end)))
	}

	if camType == rgbdCamera {
		end *= 2
	}

	if last == "" {
		return getFile(start, end)
	}
	lastFileNum, err := strconv.Atoi(last)
	if err != nil {
		return 0, err
	}
	return getFile(lastFileNum+1, end)
}

// getFile will return the next file to be returned after checking it satisfies the end condition.
func getFile(i, end int) (int, error) {
	if i < end {
		return i, nil
	}
	return 0, ErrEndOfDataset
}

// getCompressedBytesFromArtifact will return an array of bytes from the
// provided artifact path.
func getCompressedBytesFromArtifact(method method, newFileNum int, camType cameraType) ([]byte, error) {
	inputPath, err := getDatasetDirectory(method, newFileNum, camType)
	if err != nil {
		return nil, err
	}

	artifactPath, err := artifact.Path(inputPath)
	if err != nil {
		return nil, ErrEndOfDataset
	}
	path := filepath.Clean(artifactPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, ErrEndOfDataset
	}

	return data, nil
}

// getPointCloudFromArtifact will return a point cloud based on the provided artifact path.
func getPointCloudFromArtifact(i int, camType cameraType) (pointcloud.PointCloud, error) {
	datasetDirectory, err := getDatasetDirectory(nextPointCloud, i, camType)
	if err != nil {
		return nil, err
	}
	path := filepath.Clean(artifact.MustPath(datasetDirectory))
	pcdFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer utils.UncheckedErrorFunc(pcdFile.Close)

	pcExpected, err := pointcloud.ReadPCD(pcdFile)
	if err != nil {
		return nil, err
	}

	return pcExpected, nil
}

type ctrl struct {
	statusCode int
	fileNumber int
	method     method
	camType    cameraType
}

// mockHandler will return the pcd file attached to the mock server.
func (c *ctrl) mockHandler(w http.ResponseWriter, r *http.Request) {
	pcdFile, _ := getCompressedBytesFromArtifact(c.method, c.fileNumber, c.camType)

	w.WriteHeader(c.statusCode)
	w.Write(pcdFile)
}

// newHTTPMock creates a set of mock http servers based on the number of PCD or image files used for testing.
func newHTTPMock(pattern string, statusCode int, camType cameraType) map[method][]*httptest.Server {
	httpServers := map[method][]*httptest.Server{}
	for _, method := range methodList {
		numberFiles := numFilesOriginal[method]
		if camType == rgbdCamera {
			numberFiles *= 2
		}
		for i := 0; i < numberFiles; i++ {
			c := &ctrl{statusCode, i, method, camType}
			handler := http.NewServeMux()
			handler.HandleFunc(pattern, c.mockHandler)
			httpServers[method] = append(httpServers[method], httptest.NewServer(handler))
		}
	}

	return httpServers
}

func getImageFromArtifact(rgbdFileType fileType, camType cameraType, i int) (camera.NamedImage, error) {
	currentRGBDFileType = rgbdFileType
	datasetDirectory, err := getDatasetDirectory(getImages, i, camType)
	if err != nil {
		return camera.NamedImage{}, err
	}
	path := filepath.Clean(artifact.MustPath(datasetDirectory))
	file, err := os.Open(path)
	if err != nil {
		return camera.NamedImage{}, err
	}
	defer utils.UncheckedErrorFunc(file.Close)

	img, _, err := image.Decode(file)
	if err != nil {
		return camera.NamedImage{}, err
	}
	namedImage := camera.NamedImage{
		Image:      img,
		SourceName: "mock-rgbd",
	}

	return namedImage, nil
}

// getImagesFromArtifact will return an array of RGBD images based on the provided artifact path.
func getImagesFromArtifact(i int, camType cameraType) ([]camera.NamedImage, error) {
	tmpCurrentRGBDFileType := currentRGBDFileType

	namedImages := []camera.NamedImage{}
	for _, rgbdFileType := range []fileType{jpeg, depth} {
		fmt.Println("")
		fmt.Println("in getImagesFromArtifact. Calling getImageFromArtifact")
		image, err := getImageFromArtifact(rgbdFileType, camType, i)
		if err != nil {
			return nil, err
		}
		fmt.Println("getImagesFromArtifact error: ", err)
		fmt.Println("")
		namedImages = append(namedImages, image)
	}

	currentRGBDFileType = tmpCurrentRGBDFileType

	return namedImages, nil
}

func getDatasetDirectory(method method, fileNum int, camType cameraType) (string, error) {
	switch method {
	case nextPointCloud:
		return fmt.Sprintf(datasetDirectories[method][pcd], fileNum), nil
	case getImages:
		var fType fileType
		if camType == rgbdCamera {
			fileNum /= 2
			fType = currentRGBDFileType
		} else {
			fType = jpeg
		}
		switch fType {
		case jpeg:
			return fmt.Sprintf(datasetDirectories[method][jpeg], fileNum), nil
		case depth:
			return fmt.Sprintf(datasetDirectories[method][depth], fileNum), nil
		default:
			return "", errors.New("filetype not implemented; this should never happen")
		}
	default:
		return "", errors.New("method not implemented; this should never happen")
	}
}
