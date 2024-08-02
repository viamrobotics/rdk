package sync

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"testing"

	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"go.viam.com/rdk/logging"
)

// func TestFileDeletion(t *testing.T) {
// 	tests := []struct {
// 		name                    string
// 		syncEnabled             bool
// 		shouldCancelContext     bool
// 		expectedDeleteFilenames []string
// 		fileList                []string
// 		syncerInProgressFiles   []string
// 	}{
// 		{
// 			name:                    "if sync disabled, file deleter should delete every 5th file",
// 			fileList:                []string{"0shouldDelete.capture", "1.capture", "2.capture", "3.capture", "4.capture", "5shouldDelete.capture"},
// 			expectedDeleteFilenames: []string{"0shouldDelete.capture", "5shouldDelete.capture"},
// 		},
// 		{
// 			name:                    "if sync enabled and all files marked as in progress, file deleter should not delete any files",
// 			syncEnabled:             true,
// 			fileList:                []string{"0.capture", "1.capture", "2.capture", "3.capture", "4.capture", "5.capture"},
// 			syncerInProgressFiles:   []string{"0.capture", "1.capture", "2.capture", "3.capture", "4.capture", "5.capture"},
// 			expectedDeleteFilenames: []string{},
// 		},
// 		{
// 			name:                    "if sync enabled and some files marked as inprogress, file deleter should delete less files",
// 			syncEnabled:             true,
// 			fileList:                []string{"0.capture", "1.capture", "2shouldDelete.capture", "3.capture", "4.capture", "5.capture"},
// 			syncerInProgressFiles:   []string{"0.capture", "1.capture"},
// 			expectedDeleteFilenames: []string{"2shouldDelete.capture"},
// 		},
// 		{
// 			name:                    "if sync disabled and files are still being written to, file deleter should not delete any files",
// 			fileList:                []string{"0.prog", "1.prog", "2.prog", "3.prog", "4.prog", "5.prog"},
// 			expectedDeleteFilenames: []string{},
// 		},
// 		{
// 			name:                    "file deleter should not delete non datacapture files",
// 			fileList:                []string{"0.fe", "1.fi", "2.fo", "3.fum", "4.foo", "5.capture"},
// 			expectedDeleteFilenames: []string{"5.capture"},
// 		},
// 		{
// 			name:                "if cancelled context is cancelled, file deleter should return an error",
// 			shouldCancelContext: true,
// 		},
// 	}

// 	for _, tc := range tests {
// 		t.Run(tc.name, func(t *testing.T) {
// 			tempCaptureDir := t.TempDir()
// 			logger := logging.NewTestLogger(t)
// 			mockClient := MockDataSyncServiceClient{}

// 			var syncer *Syncer
// 			if tc.syncEnabled {
// 				config := Config{CaptureDir: tempCaptureDir}.applyDefaults()
// 				syncer = NewSyncer(config, "rick astley", mockClient, clk.New(), logger)
// 				defer syncer.Close()
// 			}

// 			filepaths := writeFiles(t, tempCaptureDir, tc.fileList)
// 			for _, file := range tc.syncerInProgressFiles {
// 				syncer.MarkInProgress(filepaths[file])
// 			}

// 			ctx, cancelFunc := context.WithCancel(context.Background())
// 			defer cancelFunc()
// 			if tc.shouldCancelContext {
// 				cancelFunc()
// 			}
// 			deletedFileCount, err := deleteFiles(ctx, syncer, 5, tempCaptureDir, logger)
// 			if tc.shouldCancelContext {
// 				test.That(t, err, test.ShouldBeError, context.Canceled)
// 			} else {
// 				test.That(t, err, test.ShouldBeNil)
// 				test.That(t, deletedFileCount, test.ShouldEqual, len(tc.expectedDeleteFilenames))
// 				// get list of all files still in capture dir after deletion
// 				files := getFiles(t, tempCaptureDir)
// 				for _, deletedFile := range tc.expectedDeleteFilenames {
// 					test.That(t, files, test.ShouldNotContain, deletedFile)
// 				}
// 			}
// 		})
// 	}
// }

func TestFileDeletionUsageCheck(t *testing.T) {
	tests := []struct {
		name              string
		deletionExpected  bool
		triggerThreshold  float64
		captureUsageRatio float64
		captureDirExists  bool
	}{
		{
			name:              "we should return false from deletion check if not at file system capacity threshold",
			deletionExpected:  false,
			triggerThreshold:  .99,
			captureUsageRatio: .99,
			captureDirExists:  true,
		},
		{
			name:              "we return true from deletion check if at file system capacity threshold",
			deletionExpected:  true,
			triggerThreshold:  math.SmallestNonzeroFloat64,
			captureUsageRatio: math.SmallestNonzeroFloat64,
			captureDirExists:  true,
		},
		{
			name: "we return false from deletion check" +
				"if at file system capacity threshold but not capture dir threshold",
			deletionExpected:  false,
			triggerThreshold:  math.SmallestNonzeroFloat64,
			captureUsageRatio: 1.0,
			captureDirExists:  true,
		},
		{
			name:              "we should return false from deletion check if capture dir does not exist",
			deletionExpected:  false,
			triggerThreshold:  .95,
			captureUsageRatio: .5,
			captureDirExists:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var tempCaptureDir string
			if tc.captureDirExists {
				tempCaptureDir = t.TempDir()
				// write testing files
				writeFiles(t, tempCaptureDir, []string{"1.capture", "2.capture"})
			}
			// overwrite thresholds
			fsThresholdToTriggerDeletion := FSThresholdToTriggerDeletion
			captureDirToFSUsageRatio := CaptureDirToFSUsageRatio
			FSThresholdToTriggerDeletion = tc.triggerThreshold
			CaptureDirToFSUsageRatio = tc.captureUsageRatio
			t.Cleanup(func() {
				FSThresholdToTriggerDeletion = fsThresholdToTriggerDeletion
				CaptureDirToFSUsageRatio = captureDirToFSUsageRatio
			})
			logger := logging.NewTestLogger(t)
			willDelete, err := shouldDeleteBasedOnDiskUsage(context.Background(), tempCaptureDir, logger)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, willDelete, test.ShouldEqual, tc.deletionExpected)
		})
	}
}

func writeFiles(t *testing.T, dir string, filenames []string) map[string]string {
	t.Helper()
	fileContents := []byte("never gonna let you down")
	filePaths := map[string]string{}
	for _, filename := range filenames {
		filePath := fmt.Sprintf("%s/%s", dir, filename)
		err := os.WriteFile(filePath, fileContents, 0o755)
		test.That(t, err, test.ShouldBeNil)
		filePaths[filename] = filePath
	}
	return filePaths
}

func getFiles(t *testing.T, path string) []string {
	t.Helper()
	dir, err := os.Open(path)
	test.That(t, err, test.ShouldBeNil)
	defer dir.Close()
	files, err := dir.Readdir(-1)
	test.That(t, err, test.ShouldBeNil)
	output := []string{}
	for _, file := range files {
		output = append(output, file.Name())
	}
	return output
}

type MockDataSyncServiceClient struct {
	T                              *testing.T
	DataCaptureUploadFunc          func(ctx context.Context, in *v1.DataCaptureUploadRequest, opts ...grpc.CallOption) (*v1.DataCaptureUploadResponse, error)
	FileUploadFunc                 func(ctx context.Context, opts ...grpc.CallOption) (v1.DataSyncService_FileUploadClient, error)
	StreamingDataCaptureUploadFunc func(ctx context.Context, opts ...grpc.CallOption) (v1.DataSyncService_StreamingDataCaptureUploadClient, error)
}

func (c MockDataSyncServiceClient) DataCaptureUpload(ctx context.Context, in *v1.DataCaptureUploadRequest, opts ...grpc.CallOption) (*v1.DataCaptureUploadResponse, error) {
	if c.DataCaptureUploadFunc == nil {
		err := errors.New("DataCaptureUpload unimplemented")
		c.T.Log(err)
		c.T.FailNow()
		return nil, err
	}
	return c.DataCaptureUploadFunc(ctx, in, opts...)
}

func (c MockDataSyncServiceClient) FileUpload(ctx context.Context, opts ...grpc.CallOption) (v1.DataSyncService_FileUploadClient, error) {
	if c.FileUploadFunc == nil {
		err := errors.New("FileUpload unimplmented")
		c.T.Log(err)
		c.T.FailNow()
		return nil, err
	}
	return c.FileUploadFunc(ctx, opts...)
}

func (c MockDataSyncServiceClient) StreamingDataCaptureUpload(ctx context.Context, opts ...grpc.CallOption) (v1.DataSyncService_StreamingDataCaptureUploadClient, error) {
	if c.StreamingDataCaptureUploadFunc == nil {
		err := errors.New("StreamingDataCaptureUpload unimplmented")
		c.T.Log(err)
		c.T.FailNow()
		return nil, errors.New("StreamingDataCaptureUpload unimplmented")
	}
	return c.StreamingDataCaptureUploadFunc(ctx, opts...)
}

type DataSyncService_FileUploadClientMock struct {
	T                *testing.T
	SendFunc         func(*v1.FileUploadRequest) error
	CloseAndRecvFunc func() (*v1.FileUploadResponse, error)
}

func (m *DataSyncService_FileUploadClientMock) Send(in *v1.FileUploadRequest) error {
	if m.SendFunc == nil {
		err := errors.New("Send unimplmented")
		m.T.Log(err)
		m.T.FailNow()
		return err
	}
	return m.SendFunc(in)
}

func (m *DataSyncService_FileUploadClientMock) CloseAndRecv() (*v1.FileUploadResponse, error) {
	if m.CloseAndRecvFunc == nil {
		err := errors.New("CloseAndRecv unimplmented")
		m.T.Log(err)
		m.T.FailNow()
		return nil, err
	}
	return m.CloseAndRecvFunc()
}

func (m *DataSyncService_FileUploadClientMock) Header() (metadata.MD, error) {
	err := errors.New("Header unimplmented")
	m.T.Log(err)
	m.T.FailNow()
	return nil, err
}

func (m *DataSyncService_FileUploadClientMock) Trailer() metadata.MD {
	m.T.Log("Trailer unimplemented")
	m.T.FailNow()
	return metadata.MD{}
}

func (m *DataSyncService_FileUploadClientMock) CloseSend() error {
	err := errors.New("CloseSend unimplmented")
	m.T.Log(err)
	m.T.FailNow()
	return err
}

func (m *DataSyncService_FileUploadClientMock) Context() context.Context {
	m.T.Log("Context unimplmented")
	m.T.FailNow()
	return nil
}

func (m *DataSyncService_FileUploadClientMock) SendMsg(any) error {
	err := errors.New("SendMsg unimplmented")
	m.T.Log(err)
	m.T.FailNow()
	return err
}

func (m *DataSyncService_FileUploadClientMock) RecvMsg(any) error {
	err := errors.New("RecvMsg unimplmented")
	m.T.Log(err)
	m.T.FailNow()
	return err
}
