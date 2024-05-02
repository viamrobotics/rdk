package shell

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"

	pb "go.viam.com/api/service/shell/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/timestamppb"

	shelltestutils "go.viam.com/rdk/services/shell/testutils"
)

func TestShellRPCFileReadCopier(t *testing.T) {
	t.Run("no files", func(t *testing.T) {
		memCopier := inMemoryFileCopier{
			files: map[string]copiedFile{},
		}
		memReader := inMemoryRPCCopyReader{}
		readCopier := newShellRPCFileReadCopier(&memReader, &memCopier)
		test.That(t, readCopier.ReadAll(context.Background()), test.ShouldBeNil)
		test.That(t, readCopier.Close(context.Background()), test.ShouldBeNil)
		test.That(t, memReader.closeCalled, test.ShouldEqual, 1)
	})

	modTime := time.Unix(time.Now().Unix(), 0).UTC()
	mode := uint32(0o222)
	smallFile := []*pb.FileData{
		{
			Name:    "small_file",
			Size:    25,
			Data:    bytes.Repeat([]byte{'a'}, 20),
			ModTime: timestamppb.New(modTime),
			Mode:    &mode,
		},
		{
			Data: bytes.Repeat([]byte{'b'}, 5),
		},
		{
			Eof: true,
		},
	}

	t.Run("single small file", func(t *testing.T) {
		memCopier := inMemoryFileCopier{
			files: map[string]copiedFile{},
		}
		memReader := inMemoryRPCCopyReader{
			fileDatas: smallFile,
		}
		readCopier := newShellRPCFileReadCopier(&memReader, &memCopier)
		test.That(t, readCopier.ReadAll(context.Background()), test.ShouldBeNil)
		test.That(t, readCopier.Close(context.Background()), test.ShouldBeNil)
		test.That(t, memReader.ackCalled, test.ShouldEqual, 1)
		test.That(t, memReader.closeCalled, test.ShouldEqual, 1)
		test.That(t, memCopier.files, test.ShouldResemble, map[string]copiedFile{
			"small_file": {
				name:  "small_file",
				data:  bytes.Join([][]byte{smallFile[0].Data, smallFile[1].Data}, nil),
				mtime: modTime,
				mode:  mode,
			},
		})
	})

	t.Run("single small file incomplete", func(t *testing.T) {
		memCopier := inMemoryFileCopier{
			files: map[string]copiedFile{},
		}
		memReader := inMemoryRPCCopyReader{
			fileDatas: smallFile[:1],
		}
		readCopier := newShellRPCFileReadCopier(&memReader, &memCopier)
		test.That(t, readCopier.ReadAll(context.Background()), test.ShouldBeNil)
		test.That(t, readCopier.Close(context.Background()), test.ShouldBeNil)
		test.That(t, memReader.ackCalled, test.ShouldEqual, 0)
		test.That(t, memReader.closeCalled, test.ShouldEqual, 1)
		test.That(t, memCopier.files, test.ShouldBeEmpty)
	})

	t.Run("single small file duplicate info", func(t *testing.T) {
		memCopier := inMemoryFileCopier{
			files: map[string]copiedFile{},
		}
		memReader := inMemoryRPCCopyReader{
			fileDatas: []*pb.FileData{smallFile[0], smallFile[0]},
		}
		readCopier := newShellRPCFileReadCopier(&memReader, &memCopier)
		err := readCopier.ReadAll(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "unexpected file name")
	})

	otherModTime := time.Unix(time.Now().Unix(), 0).UTC()
	otherMode := uint32(0o333)
	otherSmallFile := []*pb.FileData{
		{
			Name:    "other_file",
			Size:    250,
			Data:    bytes.Repeat([]byte{'c'}, 200),
			ModTime: timestamppb.New(otherModTime),
			Mode:    &otherMode,
		},
		{
			Data: bytes.Repeat([]byte{'d'}, 50),
		},
		{
			Eof: true,
		},
	}

	t.Run("multiple files", func(t *testing.T) {
		memCopier := inMemoryFileCopier{
			files: map[string]copiedFile{},
		}
		var datas []*pb.FileData
		datas = append(datas, smallFile...)
		datas = append(datas, otherSmallFile...)
		memReader := inMemoryRPCCopyReader{
			fileDatas: datas,
		}
		readCopier := newShellRPCFileReadCopier(&memReader, &memCopier)
		test.That(t, readCopier.ReadAll(context.Background()), test.ShouldBeNil)
		test.That(t, readCopier.Close(context.Background()), test.ShouldBeNil)
		test.That(t, memReader.ackCalled, test.ShouldEqual, 2)
		test.That(t, memReader.closeCalled, test.ShouldEqual, 1)
		test.That(t, memCopier.files, test.ShouldResemble, map[string]copiedFile{
			"small_file": {
				name:  "small_file",
				data:  bytes.Join([][]byte{smallFile[0].Data, smallFile[1].Data}, nil),
				mtime: modTime,
				mode:  mode,
			},
			"other_file": {
				name:  "other_file",
				data:  bytes.Join([][]byte{otherSmallFile[0].Data, otherSmallFile[1].Data}, nil),
				mtime: otherModTime,
				mode:  otherMode,
			},
		})
	})
}

func TestShellRPCFileCopier(t *testing.T) {
	t.Run("no files", func(t *testing.T) {
		memWriter := inMemoryRPCCopyWriter{}

		copier := newShellRPCFileCopier(&memWriter, false)
		test.That(t, copier.Close(context.Background()), test.ShouldBeNil)
		test.That(t, memWriter.ackCalled, test.ShouldEqual, 0)
		test.That(t, memWriter.closeCalled, test.ShouldEqual, 1)
	})

	tfs := shelltestutils.SetupTestFileSystem(t, strings.Repeat("a", 1<<8))
	beforeInfo, err := os.Stat(tfs.SingleFileNested)
	test.That(t, err, test.ShouldBeNil)
	newMode := os.FileMode(0o444)
	test.That(t, beforeInfo.Mode(), test.ShouldNotEqual, newMode)
	test.That(t, os.Chmod(tfs.SingleFileNested, newMode), test.ShouldBeNil)
	modTime := time.Date(1988, 1, 2, 3, 0, 0, 0, time.UTC)
	test.That(t, os.Chtimes(tfs.SingleFileNested, time.Time{}, modTime), test.ShouldBeNil)

	t.Run("single file", func(t *testing.T) {
		for _, preserve := range []bool{false, true} {
			t.Run(fmt.Sprintf("preserve=%t", preserve), func(t *testing.T) {
				file, err := os.Open(tfs.SingleFileNested)
				test.That(t, err, test.ShouldBeNil)

				shellFile := File{
					RelativeName: "this/is_a_file",
					Data:         file,
				}

				memWriter := inMemoryRPCCopyWriter{}
				copier := newShellRPCFileCopier(&memWriter, preserve)
				test.That(t, copier.Copy(context.Background(), shellFile), test.ShouldBeNil)
				test.That(t, copier.Close(context.Background()), test.ShouldBeNil)
				test.That(t, memWriter.ackCalled, test.ShouldEqual, 1)
				test.That(t, memWriter.closeCalled, test.ShouldEqual, 1)
				test.That(t, memWriter.fileDatas[0].Name, test.ShouldEqual, shellFile.RelativeName)
				test.That(t, memWriter.fileDatas[0].Eof, test.ShouldBeFalse)

				if preserve {
					test.That(t, memWriter.fileDatas[0].ModTime, test.ShouldNotBeNil)
					test.That(t, memWriter.fileDatas[0].Mode, test.ShouldNotBeNil)
					test.That(t, memWriter.fileDatas[0].ModTime.AsTime().String(), test.ShouldEqual, modTime.String())
					test.That(t, fs.FileMode(*memWriter.fileDatas[0].Mode), test.ShouldEqual, newMode)
				} else {
					test.That(t, memWriter.fileDatas[0].ModTime, test.ShouldBeNil)
					test.That(t, memWriter.fileDatas[0].Mode, test.ShouldBeNil)
				}

				test.That(t, len(memWriter.fileDatas), test.ShouldEqual, 6)
				test.That(t, memWriter.fileDatas[1].Name, test.ShouldBeEmpty)
				test.That(t, memWriter.fileDatas[1].Eof, test.ShouldBeFalse)
				test.That(t, memWriter.fileDatas[len(memWriter.fileDatas)-1].Data, test.ShouldHaveLength, 0)
				test.That(t, memWriter.fileDatas[len(memWriter.fileDatas)-1].Eof, test.ShouldBeTrue)
			})
		}
	})

	t.Run("multiple files", func(t *testing.T) {
		file1, err := os.Open(tfs.SingleFileNested)
		test.That(t, err, test.ShouldBeNil)
		file2, err := os.Open(tfs.InnerDir)
		test.That(t, err, test.ShouldBeNil)
		shellFile1 := File{
			RelativeName: "this/is_a_file",
			Data:         file1,
		}
		shellFile2 := File{
			RelativeName: "another/file",
			Data:         file2,
		}

		memWriter := inMemoryRPCCopyWriter{}
		copier := newShellRPCFileCopier(&memWriter, false)
		test.That(t, copier.Copy(context.Background(), shellFile1), test.ShouldBeNil)
		test.That(t, memWriter.ackCalled, test.ShouldEqual, 1)
		test.That(t, copier.Copy(context.Background(), shellFile2), test.ShouldBeNil)
		test.That(t, memWriter.ackCalled, test.ShouldEqual, 2)
		test.That(t, copier.Close(context.Background()), test.ShouldBeNil)
		test.That(t, memWriter.ackCalled, test.ShouldEqual, 2)
		test.That(t, memWriter.closeCalled, test.ShouldEqual, 1)

		test.That(t, memWriter.fileDatas[0].Name, test.ShouldEqual, shellFile1.RelativeName)
		test.That(t, memWriter.fileDatas[0].Eof, test.ShouldBeFalse)
		test.That(t, memWriter.fileDatas[0].IsDir, test.ShouldBeFalse)
		test.That(t, len(memWriter.fileDatas), test.ShouldEqual, 7)
		test.That(t, memWriter.fileDatas[1].Name, test.ShouldBeEmpty)
		test.That(t, memWriter.fileDatas[1].Eof, test.ShouldBeFalse)
		test.That(t, memWriter.fileDatas[5].Data, test.ShouldHaveLength, 0)
		test.That(t, memWriter.fileDatas[5].Eof, test.ShouldBeTrue)

		// directory is 0 bytes so EOF
		test.That(t, memWriter.fileDatas[6].Name, test.ShouldEqual, shellFile2.RelativeName)
		test.That(t, memWriter.fileDatas[6].Eof, test.ShouldBeTrue)
		test.That(t, memWriter.fileDatas[6].IsDir, test.ShouldBeTrue)
	})
}

type inMemoryRPCCopyWriter struct {
	fileDatas   []*pb.FileData
	ackCalled   int
	closeCalled int
}

func (mem *inMemoryRPCCopyWriter) SendFile(fileDataProto *pb.FileData) error {
	mem.fileDatas = append(mem.fileDatas, fileDataProto)
	return nil
}

func (mem *inMemoryRPCCopyWriter) WaitLastACK() error {
	mem.ackCalled++
	return nil
}

func (mem *inMemoryRPCCopyWriter) Close() error {
	mem.closeCalled++
	return nil
}

type inMemoryRPCCopyReader struct {
	fileDatas   []*pb.FileData
	ackCalled   int
	closeCalled int
}

func (mem *inMemoryRPCCopyReader) NextFileData() (*pb.FileData, error) {
	if len(mem.fileDatas) == 0 {
		return nil, io.EOF
	}
	next := mem.fileDatas[0]
	mem.fileDatas = mem.fileDatas[1:]
	return next, nil
}

func (mem *inMemoryRPCCopyReader) AckLastFile() error {
	mem.ackCalled++
	return nil
}

func (mem *inMemoryRPCCopyReader) Close() error {
	mem.closeCalled++
	return nil
}

type copiedFile struct {
	name  string
	data  []byte
	mode  uint32
	mtime time.Time
}

type inMemoryFileCopier struct {
	files map[string]copiedFile
}

func (mem *inMemoryFileCopier) Copy(ctx context.Context, file File) error {
	var buf bytes.Buffer
	n, err := io.Copy(&buf, file.Data)
	if err != nil {
		return err
	}
	info, err := file.Data.Stat()
	if err != nil {
		return err
	}
	if info.Size() != n {
		return fmt.Errorf("size mismatch %d!=%d (read)", info.Size(), n)
	}
	mem.files[file.RelativeName] = copiedFile{
		name:  file.RelativeName,
		data:  buf.Bytes(),
		mode:  uint32(info.Mode()),
		mtime: info.ModTime(),
	}
	return nil
}

func (mem *inMemoryFileCopier) Close(ctx context.Context) error {
	return nil
}
