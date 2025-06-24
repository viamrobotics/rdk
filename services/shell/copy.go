package shell

import (
	"context"
	"io/fs"

	servicepb "go.viam.com/api/service/shell/v1"
)

// A File is a complete File to be sent/received.
type File struct {
	// RelativeName is the filesystem agnostic name.
	// For example, ~/my/file would appear as my/file.
	RelativeName string
	// The underlying data for the File and it must have
	// a non-nil fs.FileInfo.
	Data fs.File
}

// A FileCopier receives files to copy into some implementation specific
// location. It is only safe to call for one file at a time.
type FileCopier interface {
	Copy(ctx context.Context, file File) error
	Close(ctx context.Context) error
}

// A FileCopyFactory is used to create FileCopiers when the CopyFilesSourceType is not
// known until a later time. For example, asking to copy files from a machine results in
// an RPC that is likely to be handled by a method that knows how to make FileCopiers, but
// not what kind yet. This is a consequence of the current flow of operations and in the future
// the factory may become obviated.
type FileCopyFactory interface {
	MakeFileCopier(ctx context.Context, sourceType CopyFilesSourceType) (FileCopier, error)
}

// A FileReadCopier is mostly a tee-like utility to read all files from somewhere and
// pass them through to some bound-to, underlying FileCopier. Similar to the FileCopyFactory,
// this abstraction exists so as to promote re-use across client/server interactions while
// reducing some extra abstractions like a FileReader/FileIterator.
type FileReadCopier interface {
	ReadAll(ctx context.Context) error
	Close(ctx context.Context) error
}

// CopyFilesSourceType indicates the types of files being transmitted.
type CopyFilesSourceType int

const (
	// CopyFilesSourceTypeSingleFile is just one normal file being copied.
	CopyFilesSourceTypeSingleFile CopyFilesSourceType = iota
	// CopyFilesSourceTypeSingleDirectory is one top-level directory being
	// copied but the transmission may contain many files. This disambiguates
	// where to place the directory versus multiple files. Multiple files always
	// go within the target destination, but a single directory may be renamed
	// at the top depending on if the target destination exists or not. Either
	// way, the receiving side needs to know what is being worked with. This
	// is a consequence of mimicking SCP and is admittedly a bit involved as well
	// as probably annoying to implement correctly.
	CopyFilesSourceTypeSingleDirectory
	// CopyFilesSourceTypeMultipleFiles indicates multiple files will be transmitted.
	CopyFilesSourceTypeMultipleFiles
	// CopyFilesSourceTypeMultipleUnknown should never be used.
	CopyFilesSourceTypeMultipleUnknown
)

// String returns a human-readable string of the type.
func (t CopyFilesSourceType) String() string {
	switch t {
	case CopyFilesSourceTypeSingleFile:
		return "SingleFile"
	case CopyFilesSourceTypeSingleDirectory:
		return "SingleDirectory"
	case CopyFilesSourceTypeMultipleFiles:
		return "MultipleFiles"
	case CopyFilesSourceTypeMultipleUnknown:
		fallthrough
	default:
		return "Unknown"
	}
}

// CopyFilesSourceTypeFromProto converts from proto to native.
func CopyFilesSourceTypeFromProto(p servicepb.CopyFilesSourceType) CopyFilesSourceType {
	switch p {
	case servicepb.CopyFilesSourceType_COPY_FILES_SOURCE_TYPE_SINGLE_FILE:
		return CopyFilesSourceTypeSingleFile
	case servicepb.CopyFilesSourceType_COPY_FILES_SOURCE_TYPE_SINGLE_DIRECTORY:
		return CopyFilesSourceTypeSingleDirectory
	case servicepb.CopyFilesSourceType_COPY_FILES_SOURCE_TYPE_MULTIPLE_FILES:
		return CopyFilesSourceTypeMultipleFiles
	case servicepb.CopyFilesSourceType_COPY_FILES_SOURCE_TYPE_UNSPECIFIED:
		fallthrough
	default:
		return CopyFilesSourceTypeMultipleUnknown
	}
}

// ToProto converts from native to proto.
func (t CopyFilesSourceType) ToProto() servicepb.CopyFilesSourceType {
	switch t {
	case CopyFilesSourceTypeSingleFile:
		return servicepb.CopyFilesSourceType_COPY_FILES_SOURCE_TYPE_SINGLE_FILE
	case CopyFilesSourceTypeSingleDirectory:
		return servicepb.CopyFilesSourceType_COPY_FILES_SOURCE_TYPE_SINGLE_DIRECTORY
	case CopyFilesSourceTypeMultipleFiles:
		return servicepb.CopyFilesSourceType_COPY_FILES_SOURCE_TYPE_MULTIPLE_FILES
	case CopyFilesSourceTypeMultipleUnknown:
		fallthrough
	default:
		return servicepb.CopyFilesSourceType_COPY_FILES_SOURCE_TYPE_UNSPECIFIED
	}
}
