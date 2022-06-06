//go:build linux
// +build linux

package datamanager

import "syscall"

func getMockStatfsFn(blocks int, bsize int, bavail int, bfree int) func(string, *syscall.Statfs_t) error {
	return func(path string, stat *syscall.Statfs_t) error {
		stat.Blocks = uint64(blocks)
		stat.Bsize = int64(bsize)
		stat.Bavail = uint64(bavail)
		stat.Bfree = uint64(bfree)
		return nil
	}
}
