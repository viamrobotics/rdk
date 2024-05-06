package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"sync"
	"time"

	pb "go.viam.com/api/service/shell/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// copy_rpc supports RPC based copy operations and is agnostic to where files
// originated from beyond the RPC boundary. The main testing for this is done
// in the CLI package since it's just easier to coordinate over there.

// newCopyFileFromMachineFactory returns a factory that can make FileCopiers for
// the Writer/Server side of a CopyFrom.
func newCopyFileFromMachineFactory(
	srv pb.ShellService_CopyFilesFromMachineServer, preserve bool,
) FileCopyFactory {
	return &copyFileFromMachineFactory{srv: srv, preserve: preserve}
}

type copyFileFromMachineFactory struct {
	once     bool
	onceMu   sync.Mutex
	srv      pb.ShellService_CopyFilesFromMachineServer
	preserve bool
}

func (f *copyFileFromMachineFactory) MakeFileCopier(ctx context.Context, sourceType CopyFilesSourceType) (FileCopier, error) {
	f.onceMu.Lock()
	if f.once {
		f.onceMu.Unlock()
		return nil, errors.New("can only support making one file copier right now")
	}
	f.onceMu.Unlock()
	if err := f.srv.Send(&pb.CopyFilesFromMachineResponse{
		Response: &pb.CopyFilesFromMachineResponse_Metadata{
			Metadata: &pb.CopyFilesFromMachineResponseMetadata{
				SourceType: sourceType.ToProto(),
			},
		},
	}); err != nil {
		return nil, err
	}

	return newShellRPCFileCopier(shellRPCCopyWriterFrom{f.srv}, f.preserve), nil
}

// NewCopyFileToMachineFactory returns a simple FileCopyFactory that calls a service's
// CopyFilesToMachine method once the CopyFilesSourceType is discovered by the initiating
// process.
func NewCopyFileToMachineFactory(
	destination string,
	preserve bool,
	shellSvc Service,
) FileCopyFactory {
	return &copyFileToMachineFactory{
		destination: destination,
		preserve:    preserve,
		svc:         shellSvc,
	}
}

type copyFileToMachineFactory struct {
	destination string
	preserve    bool
	svc         Service
}

func (f *copyFileToMachineFactory) MakeFileCopier(ctx context.Context, sourceType CopyFilesSourceType) (FileCopier, error) {
	return f.svc.CopyFilesToMachine(ctx, sourceType, f.destination, f.preserve, nil)
}

// A shellRPCCopyReader is a light abstraction around the different directions of
// copying (reading) in order to get a little bit of code re-use in addition to making it
// more clear how to use the protocol. Implementations are inverses of each other.
type shellRPCCopyReader interface {
	NextFileData() (*pb.FileData, error)
	AckLastFile() error
	Close() error
}

// A shellRPCCopyReaderTo is the To, Reader/Server side of a CopyTo.
type shellRPCCopyReaderTo struct {
	rpcServer pb.ShellService_CopyFilesToMachineServer
}

func (srv shellRPCCopyReaderTo) AckLastFile() error {
	return srv.rpcServer.Send(&pb.CopyFilesToMachineResponse{
		AckLastFile: true,
	})
}

func (srv shellRPCCopyReaderTo) NextFileData() (*pb.FileData, error) {
	req, err := srv.rpcServer.Recv()
	if err != nil {
		return nil, err
	}
	reqFileData, ok := req.Request.(*pb.CopyFilesToMachineRequest_FileData)
	if !ok {
		return nil, errors.New("expected copy request file data")
	}
	return reqFileData.FileData, nil
}

func (srv shellRPCCopyReaderTo) Close() error {
	return nil
}

// A shellRPCCopyReaderFrom is the From, Reader/Client side of a CopyFrom.
type shellRPCCopyReaderFrom struct {
	rpcClient pb.ShellService_CopyFilesFromMachineClient
}

func (client shellRPCCopyReaderFrom) AckLastFile() error {
	return client.rpcClient.Send(&pb.CopyFilesFromMachineRequest{
		Request: &pb.CopyFilesFromMachineRequest_AckLastFile{
			AckLastFile: true,
		},
	})
}

func (client shellRPCCopyReaderFrom) NextFileData() (*pb.FileData, error) {
	resp, err := client.rpcClient.Recv()
	if err != nil {
		return nil, err
	}
	reqFileData, ok := resp.Response.(*pb.CopyFilesFromMachineResponse_FileData)
	if !ok {
		return nil, errors.New("expected copy response file data")
	}
	return reqFileData.FileData, nil
}

func (client shellRPCCopyReaderFrom) Close() error {
	return client.rpcClient.CloseSend()
}

// A shellRPCFileReadCopier is a lightly utility struct that implements FileReadCopier
// and uses a shellRPCCopyReader to copy all files into some FileCopier (RPC based or not).
type shellRPCFileReadCopier struct {
	rpc    shellRPCCopyReader
	copier FileCopier
}

func newShellRPCFileReadCopier(rpc shellRPCCopyReader, copier FileCopier) FileReadCopier {
	return &shellRPCFileReadCopier{
		rpc:    rpc,
		copier: copier,
	}
}

// ReadAll is only safe to call once.
func (reader *shellRPCFileReadCopier) ReadAll(ctx context.Context) error {
	for {
		file, err := reader.rpc.NextFileData()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		if err := reader.copyFile(ctx, file); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		if err := reader.rpc.AckLastFile(); err != nil {
			return err
		}
	}
	return nil
}

func (reader *shellRPCFileReadCopier) copyFile(
	ctx context.Context,
	initialFile *pb.FileData,
) error {
	fileName := initialFile.Name
	if fileName == "" {
		return errors.New("file name blank")
	}
	copyErr := make(chan error, 1)
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// we create a streaming based file using FIFO logic so that we
	// don't keep too much data in memory.
	streamingCopy := &streamingRPCFileCopy{
		info: fileInfoData{
			name:  filepath.Base(initialFile.Name),
			size:  initialFile.Size,
			isDir: initialFile.IsDir,
		},
		dataCh: make(chan []byte),
	}

	// this data is present pending the preserve option being used.
	if initialFile.Mode != nil {
		streamingCopy.info.mode = *initialFile.Mode
	}
	if initialFile.ModTime != nil {
		streamingCopy.info.modTime = initialFile.ModTime.AsTime()
	}
	defer utils.UncheckedErrorFunc(streamingCopy.Close)
	utils.PanicCapturingGo(func() {
		copyErr <- reader.copier.Copy(cancelCtx, File{
			RelativeName: fileName,
			Data:         streamingCopy,
		})
	})

	file := initialFile
	for {
		if file.Eof {
			streamingCopy.close(io.EOF)
			break
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		select {
		case <-ctx.Done():
		case streamingCopy.dataCh <- file.Data:
		}
		var err error
		file, err = reader.rpc.NextFileData()
		if err != nil {
			return err
		}
		if file.Name != "" {
			// Note(erd): this is a forwards compatibility defense against multiple files being
			// at the same time. Without this, we may clobber data together.
			return fmt.Errorf("unexpected file name %q while reading current file %q", file.Name, initialFile.Name)
		}
	}

	return <-copyErr
}

func (reader *shellRPCFileReadCopier) Close(ctx context.Context) error {
	return reader.rpc.Close()
}

// A streamingRPCFileCopy is an efficient, low-memory usage representation
// of an fs.File being populated via RPC.
type streamingRPCFileCopy struct {
	info     fileInfoData
	lastData []byte
	dataCh   chan []byte
	closeMu  sync.Mutex
	closed   bool
	closeErr error
}

type fileInfoData struct {
	name    string
	size    int64
	isDir   bool
	modTime time.Time
	mode    uint32
}

func (info fileInfoData) Name() string {
	return info.name
}

func (info fileInfoData) Size() int64 {
	return info.size
}

func (info fileInfoData) Mode() fs.FileMode {
	// may be 0 -- must check preserve flag elsewhere
	return fs.FileMode(info.mode)
}

func (info fileInfoData) ModTime() time.Time {
	// may be zero value -- must check preserve flag elsewhere
	return info.modTime
}

func (info fileInfoData) IsDir() bool {
	return info.isDir
}

func (info fileInfoData) Sys() any {
	return nil
}

func (file *streamingRPCFileCopy) Stat() (fs.FileInfo, error) {
	return file.info, nil
}

// Read is not safe to call concurrently.
func (file *streamingRPCFileCopy) Read(buf []byte) (int, error) {
	if file.lastData == nil {
		data, ok := <-file.dataCh
		if !ok {
			return 0, file.closeErr
		}
		file.lastData = data
	}
	nToCopy := len(file.lastData)
	if nToCopy > len(buf) {
		nToCopy = len(buf)
	}
	copy(buf, file.lastData[:nToCopy])
	file.lastData = file.lastData[nToCopy:]
	if len(file.lastData) == 0 {
		file.lastData = nil
	}
	return nToCopy, nil
}

func (file *streamingRPCFileCopy) close(reason error) {
	file.closeMu.Lock()
	if file.closed {
		file.closeMu.Unlock()
		return
	}
	file.closed = true
	file.closeErr = reason
	close(file.dataCh)
	file.closeMu.Unlock()
}

func (file *streamingRPCFileCopy) Close() error {
	file.close(fs.ErrClosed)
	return nil
}

// A shellRPCCopyWriter is a light abstraction around the different directions of
// copying (writing) in order to get a little bit of code re-use in addition to making it
// more clear how to use the protocol. Implementations are inverses of each other.
type shellRPCCopyWriter interface {
	SendFile(fileDataProto *pb.FileData) error
	WaitLastACK() error
	Close() error
}

// A shellRPCCopyWriterTo is the To, Writer/Client side of a CopyTo.
type shellRPCCopyWriterTo struct {
	rpcClient pb.ShellService_CopyFilesToMachineClient
}

func (client shellRPCCopyWriterTo) SendFile(fileDataProto *pb.FileData) error {
	return client.rpcClient.Send(&pb.CopyFilesToMachineRequest{
		Request: &pb.CopyFilesToMachineRequest_FileData{
			FileData: fileDataProto,
		},
	})
}

func (client shellRPCCopyWriterTo) WaitLastACK() error {
	_, err := client.rpcClient.Recv()
	return err
}

func (client shellRPCCopyWriterTo) Close() error {
	return client.rpcClient.CloseSend()
}

// A shellRPCCopyWriterFrom is the From, Writer/Server side of a CopyFrom.
type shellRPCCopyWriterFrom struct {
	rpcServer pb.ShellService_CopyFilesFromMachineServer
}

func (srv shellRPCCopyWriterFrom) SendFile(fileDataProto *pb.FileData) error {
	return srv.rpcServer.Send(&pb.CopyFilesFromMachineResponse{
		Response: &pb.CopyFilesFromMachineResponse_FileData{
			FileData: fileDataProto,
		},
	})
}

func (srv shellRPCCopyWriterFrom) WaitLastACK() error {
	_, err := srv.rpcServer.Recv()
	return err
}

func (srv shellRPCCopyWriterFrom) Close() error {
	return nil
}

// A shellFileCopyWriter wraps a shellRPCCopyWriter and handles streaming a file over
// RPC to the other side. It tries to send as large packets as possible but under the
// limits imposed by RPC.
type shellFileCopyWriter struct {
	singleWriterMu sync.Mutex
	rpc            shellRPCCopyWriter
	buf            []byte
	preserve       bool
}

func newShellRPCFileCopier(rpcWriter shellRPCCopyWriter, preserve bool) FileCopier {
	return &shellFileCopyWriter{
		rpc:      rpcWriter,
		buf:      make([]byte, rpc.MaxMessageSize>>1),
		preserve: preserve,
	}
}

var _ = FileCopier(&shellFileCopyWriter{})

func (writer *shellFileCopyWriter) Copy(ctx context.Context, file File) error {
	writer.singleWriterMu.Lock()
	defer writer.singleWriterMu.Unlock()

	defer func() {
		utils.UncheckedError(file.Data.Close())
	}()
	fileInfo, err := file.Data.Stat()
	if err != nil {
		return err
	}

	firstFileDataProto := true
	var isEOF bool
	for !isEOF {
		var fileDataProto pb.FileData

		if fileInfo.IsDir() {
			isEOF = true
		} else {
			n, readErr := file.Data.Read(writer.buf)
			if readErr != nil {
				if !errors.Is(readErr, io.EOF) {
					return readErr
				}
				isEOF = true
			}
			isEOF = isEOF || n == 0
			fileDataProto.Data = writer.buf[:n]
		}
		fileDataProto.Eof = isEOF

		if firstFileDataProto {
			firstFileDataProto = false
			// no reason to keep sending after this it with current protocol
			fileDataProto.Name = file.RelativeName
			fileDataProto.IsDir = fileInfo.IsDir()
			fileDataProto.Size = fileInfo.Size()
			if writer.preserve {
				// Note(erd): maybe support access time in the future if needed
				mode := uint32(fileInfo.Mode())
				fileDataProto.Mode = &mode
				fileDataProto.ModTime = timestamppb.New(fileInfo.ModTime())
			}
		}
		if err := writer.rpc.SendFile(&fileDataProto); err != nil {
			return err
		}
		if !isEOF {
			continue
		}
		if err := writer.rpc.WaitLastACK(); err != nil {
			return err
		}
	}
	return nil
}

func (writer *shellFileCopyWriter) Close(ctx context.Context) error {
	writer.singleWriterMu.Lock()
	defer writer.singleWriterMu.Unlock()

	return writer.rpc.Close()
}
