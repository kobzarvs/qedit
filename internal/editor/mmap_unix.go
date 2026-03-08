//go:build !windows

package editor

import (
	"os"
	"syscall"
)

// mmapFile maps the entire file into memory as a read-only byte slice.
// The returned slice is valid until munmapFile is called.
func mmapFile(path string, size int64) ([]byte, error) {
	if size <= 0 {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data, err := syscall.Mmap(int(f.Fd()), 0, int(size), syscall.PROT_READ, syscall.MAP_PRIVATE)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// munmapFile releases the mmap'd memory region.
func munmapFile(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	return syscall.Munmap(data)
}
